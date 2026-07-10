// Package httprequest provides the "http request" node implementation -
// issues an outgoing HTTP request and returns the response.
//
// # Security
//
// This node lets a flow make arbitrary outgoing HTTP(S) requests - to
// whatever URL Url resolves to, which may be a fixed string or resolved
// from the message (msg/flow/global via typedvalue.Value, the same
// typed-input mechanism used throughout this codebase). Node-RED's own
// http request node has the identical property and, like it, this node
// does not attempt to block requests to private/internal network ranges
// (Server-Side Request Forgery, SSRF): determining what counts as
// "internal" is inherently environment-specific (cloud metadata
// endpoints, container-internal services, VPC-peered hosts, ...) and a
// wrong guess either blocks legitimate integrations or gives a false
// sense of safety. Operators who let untrusted parties author flows
// should restrict egress at the network level (firewall/security-group
// rules, an explicit egress proxy) rather than relying on
// application-level URL filtering here.
//
// What this node does mitigate directly:
//   - TimeoutMs bounds the whole request (headers + body), so a slow or
//     wedged server can't hang a node indefinitely.
//   - The response body is capped at maxResponseBytes (10 MiB); the rest
//     is discarded rather than buffered, bounding memory use against a
//     very large or malicious response.
//   - Redirects are capped at MaxRedirects (default 5) via
//     http.Client.CheckRedirect, rather than following net/http's default
//     unlimited-ish chain, limiting the blast radius of a redirect-based
//     SSRF attempt (an attacker-controlled server sending a 302 to an
//     internal address on a request whose original destination looked
//     external) to that bound rather than an effectively unlimited chain.
//   - TLS certificate verification is on by default; InsecureSkipVerify
//     must be explicitly set (or a referenced tls-config node's own
//     VerifyServerCert explicitly false) to disable it.
package httprequest

import (
    "bytes"
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpproxy"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/tlsconfig"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

const (
    defaultTimeout    = 30 * time.Second
    maxResponseBytes  = 10 << 20 // 10 MiB
    defaultMaxRedirects = 5
)

// Node holds an HTTP-request node's configuration.
type Node struct {
    // Url resolves to the request URL; usually a fixed string, optionally
    // msg/flow/global/env-sourced.
    Url typedvalue.Value
    // Method defaults to "GET".
    Method  string
    Headers map[string]string
    // TimeoutMs bounds the whole request; default 30s.
    TimeoutMs int64
    // MaxRedirects bounds the redirect chain; default 5.
    MaxRedirects int
    // TLS is an optional tls-config node ID.
    TLS string
    // Proxy is an optional http-proxy node ID.
    Proxy string
    // InsecureSkipVerify disables TLS certificate verification when TLS is
    // not set. Ignored if TLS is set (the referenced tls-config node's own
    // VerifyServerCert controls that instead).
    InsecureSkipVerify bool
}

// Execute resolves Url, issues the request (with msg.payload as the body
// for a non-GET/HEAD method unless Headers/msg already sets Content-Length
// 0), and returns a copy of input with payload/statusCode/headers set from
// the response.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    c, ok := ctx.(context.Context)
    if !ok {
        c = context.Background()
    }
    rt, _ := registry.RuntimeFromContext(c)

    valueResolver := typedvalue.Resolver{Message: input}
    if rt != nil {
        if rt.FlowContext != nil {
            valueResolver.FlowContext = rt.FlowContext
        }
        if rt.GlobalContext != nil {
            valueResolver.GlobalContext = rt.GlobalContext
        }
    }
    rawURL, err := n.Url.Resolve(valueResolver)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    urlStr, _ := rawURL.(string)
    if urlStr == "" {
        return nil, fmt.Errorf("http request: url is empty")
    }

    method := n.Method
    if method == "" {
        method = "GET"
    }

    var bodyReader io.Reader
    if method != "GET" && method != "HEAD" {
        body, err := requestBodyBytes(input["payload"])
        if err != nil {
            return nil, fmt.Errorf("http request: %w", err)
        }
        bodyReader = bytes.NewReader(body)
    }

    timeout := time.Duration(n.TimeoutMs) * time.Millisecond
    if timeout <= 0 {
        timeout = defaultTimeout
    }
    reqCtx, cancel := context.WithTimeout(c, timeout)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, method, urlStr, bodyReader)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    for k, v := range n.Headers {
        req.Header.Set(k, v)
    }

    client, err := n.buildClient(rt)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
    if err != nil {
        return nil, fmt.Errorf("http request: reading response: %w", err)
    }

    out := cloneMap(input)
    out["statusCode"] = float64(resp.StatusCode)
    out["headers"] = headerToMap(resp.Header)
    if isJSON(resp.Header.Get("Content-Type")) {
        var v interface{}
        if err := json.Unmarshal(respBody, &v); err == nil {
            out["payload"] = v
        } else {
            out["payload"] = string(respBody)
        }
    } else {
        out["payload"] = string(respBody)
    }
    return out, nil
}

// buildClient resolves an optional tls-config/http-proxy config node
// reference (safe to do here: Execute always runs after Deploy completes,
// unlike an EmittingNode's Start - see docs/NODE_PALETTE_PLAN.md, Phase 6).
func (n *Node) buildClient(rt *registry.NodeRuntime) (*http.Client, error) {
    transport := &http.Transport{}

    if n.TLS != "" {
        if rt == nil {
            return nil, fmt.Errorf("tls config node %q requested but no runtime is available", n.TLS)
        }
        exec, ok := rt.GetNode(n.TLS)
        if !ok {
            return nil, fmt.Errorf("tls config node %q not found", n.TLS)
        }
        provider, ok := exec.(tlsconfig.Provider)
        if !ok {
            return nil, fmt.Errorf("node %q is not a tls-config", n.TLS)
        }
        tlsCfg, err := provider.TLSConfig()
        if err != nil {
            return nil, err
        }
        transport.TLSClientConfig = tlsCfg
    } else if n.InsecureSkipVerify {
        transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
    }

    if n.Proxy != "" {
        if rt == nil {
            return nil, fmt.Errorf("http proxy node %q requested but no runtime is available", n.Proxy)
        }
        exec, ok := rt.GetNode(n.Proxy)
        if !ok {
            return nil, fmt.Errorf("http proxy config node %q not found", n.Proxy)
        }
        provider, ok := exec.(httpproxy.Provider)
        if !ok {
            return nil, fmt.Errorf("node %q is not an http proxy", n.Proxy)
        }
        transport.Proxy = provider.ProxyFunc()
    }

    maxRedirects := n.MaxRedirects
    if maxRedirects <= 0 {
        maxRedirects = defaultMaxRedirects
    }
    return &http.Client{
        Transport: transport,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= maxRedirects {
                return fmt.Errorf("stopped after %d redirects", maxRedirects)
            }
            return nil
        },
    }, nil
}

func requestBodyBytes(payload interface{}) ([]byte, error) {
    switch v := payload.(type) {
    case nil:
        return nil, nil
    case string:
        return []byte(v), nil
    case []byte:
        return v, nil
    case bool, float64:
        return []byte(fmt.Sprint(v)), nil
    default:
        encoded, err := json.Marshal(v)
        if err != nil {
            return nil, fmt.Errorf("payload cannot be encoded: %w", err)
        }
        return encoded, nil
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

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func (n *Node) Validate() error {
    switch n.Url.Type {
    case "":
        return fmt.Errorf("http request: url is required")
    }
    if n.MaxRedirects < 0 {
        return fmt.Errorf("http request: maxRedirects cannot be negative")
    }
    if n.TimeoutMs < 0 {
        return fmt.Errorf("http request: timeoutMs cannot be negative")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    headers := make(map[string]interface{}, len(n.Headers))
    for k, v := range n.Headers {
        headers[k] = v
    }
    return map[string]interface{}{
        "url":                valueToConfig(n.Url),
        "method":             n.Method,
        "headers":            headers,
        "timeoutMs":          n.TimeoutMs,
        "maxRedirects":       n.MaxRedirects,
        "tls":                n.TLS,
        "proxy":              n.Proxy,
        "insecureSkipVerify": n.InsecureSkipVerify,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Url = parseValue(config["url"])
    n.Method = "GET"
    if v, ok := config["method"].(string); ok && v != "" {
        n.Method = strings.ToUpper(v)
    }
    n.Headers = nil
    if raw, ok := config["headers"].(map[string]interface{}); ok {
        n.Headers = make(map[string]string, len(raw))
        for k, v := range raw {
            if s, ok := v.(string); ok {
                n.Headers[k] = s
            }
        }
    }
    if v, ok := config["timeoutMs"].(float64); ok {
        n.TimeoutMs = int64(v)
    }
    if v, ok := config["maxRedirects"].(float64); ok {
        n.MaxRedirects = int(v)
    }
    if v, ok := config["tls"].(string); ok {
        n.TLS = v
    }
    if v, ok := config["proxy"].(string); ok {
        n.Proxy = v
    }
    if v, ok := config["insecureSkipVerify"].(bool); ok {
        n.InsecureSkipVerify = v
    }
    return n.Validate()
}

func parseValue(raw interface{}) typedvalue.Value {
    m, ok := raw.(map[string]interface{})
    if !ok {
        return typedvalue.Value{}
    }
    t, _ := m["type"].(string)
    v, _ := m["value"].(string)
    return typedvalue.Value{Type: typedvalue.Type(t), Value: v}
}

func valueToConfig(v typedvalue.Value) map[string]interface{} {
    return map[string]interface{}{"type": string(v.Type), "value": v.Value}
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("http request", func() registry.NodeExecutor {
        return &Node{Method: "GET"}
    }, registry.NodeMetadata{
        ID:          "http request",
        Type:        "http request",
        Name:        "HTTP request",
        Description: "Issues an outgoing HTTP request and returns the response - see package docs for the security rationale (SSRF is not blocked; timeouts/size caps/redirect caps are)",
        Category:    "network",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message that triggers the request", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "payload=response body, statusCode, headers", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "url":                {Type: "object", Description: `Request URL, e.g. {"type":"str","value":"https://example.com"}`, Default: map[string]interface{}{"type": "str", "value": ""}},
                "method":             {Type: "string", Description: "HTTP method", Default: "GET", Enum: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"}},
                "headers":            {Type: "object", Description: "Fixed request headers", Default: map[string]interface{}{}},
                "timeoutMs":          {Type: "number", Description: "Request timeout in milliseconds", Default: float64(30000), Min: floatPtr(0)},
                "maxRedirects":       {Type: "number", Description: "Maximum redirects to follow", Default: float64(5), Min: floatPtr(0)},
                "tls":                {Type: "string", Description: "ID of an existing tls-config node (optional)", Default: ""},
                "proxy":              {Type: "string", Description: "ID of an existing http-proxy node (optional)", Default: ""},
                "insecureSkipVerify": {Type: "boolean", Description: "Skip TLS certificate verification (ignored if tls is set); testing only", Default: false},
            },
            Required: []string{"url"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zM2 12h20M12 2a15 15 0 010 20 15 15 0 010-20z"/></svg>`,
        Tags: []string{"network", "http", "request", "client"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
