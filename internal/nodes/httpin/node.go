// Package httpin provides the "http in" node implementation - registers a
// (method, path) route on a shared HTTP listener and emits one message per
// matching incoming request, pairing it with a *ResponseHandle so a
// downstream "http response" node (internal/nodes/httpresponse) can
// complete it.
//
// # Shared listener
//
// All "http in" nodes across every deployed flow share a single
// *http.Server, started lazily on the first Deploy that includes one (see
// server.go). It listens on GORED_HTTP_NODE_PORT (default 1880) -
// deliberately separate from cmd/go-red's own -port editor/API server,
// since node packages self-register via init() with no access to that
// server's mux; see server.go's doc comment for the full rationale.
//
// # Security: request size, timeouts, slowloris
//
// Incoming bodies are capped at maxBodyBytes (10 MiB) to bound memory use
// from a large or slow upload. If no "http response" node completes the
// message within ResponseTimeoutMs (default 30s), this node itself writes
// a 504 and unblocks the waiting HTTP handler goroutine - a flow that
// forgets to wire an http response (or whose downstream logic hangs) can't
// leak the underlying connection indefinitely. The shared *http.Server
// also sets ReadHeaderTimeout and IdleTimeout (server.go) - net/http's
// zero-value (unlimited) ReadHeaderTimeout is a well-known footgun that
// lets a client hold a connection (and the goroutine/file descriptor
// behind it) open indefinitely by sending headers one byte at a time
// ("slowloris").
package httpin

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

const defaultResponseTimeout = 30 * time.Second

// Node holds an HTTP-in node's configuration.
type Node struct {
    Method            string
    Path              string
    ResponseTimeoutMs int64

    mu   sync.Mutex
    emit func(map[string]interface{})
}

// Start registers Method/Path on the shared router and serves matching
// requests until ctx is cancelled, at which point the route is removed
// (the shared listener itself keeps running for other nodes/flows).
func (n *Node) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
    if err := ensureServerStarted(); err != nil {
        return fmt.Errorf("http in: %w", err)
    }

    n.mu.Lock()
    n.emit = emit
    n.mu.Unlock()

    r := sharedRouter.add(n.Method, n.Path, n.handle)
    defer sharedRouter.remove(r)

    <-ctx.Done()
    return nil
}

func (n *Node) handle(w http.ResponseWriter, r *http.Request, params map[string]string) {
    n.mu.Lock()
    emit := n.emit
    n.mu.Unlock()
    if emit == nil {
        http.Error(w, "node not ready", http.StatusServiceUnavailable)
        return
    }

    body, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
    var payload interface{} = string(body)
    if isJSON(r.Header.Get("Content-Type")) {
        var v interface{}
        if err := json.Unmarshal(body, &v); err == nil {
            payload = v
        }
    }

    handle := newResponseHandle(w)
    msg := map[string]interface{}{
        "payload": payload,
        "req": map[string]interface{}{
            "method":  r.Method,
            "url":     r.URL.String(),
            "headers": headerToMap(r.Header),
            "query":   queryToMap(r.URL.Query()),
            "params":  paramsToMap(params),
        },
        KeyResponseHandle: handle,
    }
    emit(msg)

    timeout := time.Duration(n.ResponseTimeoutMs) * time.Millisecond
    if timeout <= 0 {
        timeout = defaultResponseTimeout
    }
    select {
    case <-handle.Done():
    case <-time.After(timeout):
        handle.Write(http.StatusGatewayTimeout, nil, []byte("http in: no http response within timeout"))
    }
}

func isJSON(contentType string) bool {
    return strings.Contains(strings.ToLower(contentType), "json")
}

func headerToMap(h http.Header) map[string]interface{} {
    out := make(map[string]interface{}, len(h))
    for k := range h {
        out[strings.ToLower(k)] = h.Get(k)
    }
    return out
}

func queryToMap(values map[string][]string) map[string]interface{} {
    out := make(map[string]interface{}, len(values))
    for k, v := range values {
        if len(v) > 0 {
            out[k] = v[0]
        }
    }
    return out
}

func paramsToMap(params map[string]string) map[string]interface{} {
    out := make(map[string]interface{}, len(params))
    for k, v := range params {
        out[k] = v
    }
    return out
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.EmittingNode) - HTTP in has no input port; the engine never
// calls it for a node with no wired input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("http in: has no input port")
}

func (n *Node) Validate() error {
    if n.Path == "" {
        return fmt.Errorf("http in: path is required")
    }
    if !strings.HasPrefix(n.Path, "/") {
        return fmt.Errorf("http in: path must start with /")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "method":            n.Method,
        "path":              n.Path,
        "responseTimeoutMs": n.ResponseTimeoutMs,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Method = "GET"
    if v, ok := config["method"].(string); ok && v != "" {
        n.Method = strings.ToUpper(v)
    }
    if v, ok := config["path"].(string); ok {
        n.Path = v
    }
    if v, ok := config["responseTimeoutMs"].(float64); ok {
        n.ResponseTimeoutMs = int64(v)
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("http in", func() registry.NodeExecutor {
        return &Node{Method: "GET"}
    }, registry.NodeMetadata{
        ID:          "http in",
        Type:        "http in",
        Name:        "HTTP in",
        Description: "Registers an HTTP route on the shared http-in listener and emits one message per request",
        Category:    "network",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "One message per matching HTTP request", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "method":            {Type: "string", Description: "HTTP method to match", Default: "GET", Enum: []string{"GET", "POST", "PUT", "DELETE", "PATCH"}},
                "path":              {Type: "string", Description: "Path to match, e.g. /api/users/:id", Default: ""},
                "responseTimeoutMs": {Type: "number", Description: "Milliseconds to wait for an http response node before returning 504", Default: float64(30000), Min: floatPtr(1)},
            },
            Required: []string{"path"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v4H4zM4 10h16v10H4z"/></svg>`,
        Tags: []string{"network", "http", "server", "endpoint"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
