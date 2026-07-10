package httpin

import (
    "log"
    "net"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"
)

// portEnvVar overrides the default port the shared "http in" listener binds
// to. Separate from the main Go-RED editor/API server's -port flag
// (cmd/go-red/main.go) since node packages self-register via init() and
// have no access to that server's own mux - unifying the two would require
// plumbing the node registry into cmd/go-red's server setup, out of scope
// for this phase (see docs/NODE_PALETTE_PLAN.md, Phase 6).
const portEnvVar = "GORED_HTTP_NODE_PORT"
const defaultPort = "1880"

// maxBodyBytes caps how much of an incoming request body is read, so a
// large or streaming upload can't exhaust memory.
const maxBodyBytes = 10 << 20 // 10 MiB

// Timeouts on the shared *http.Server itself - net/http's zero-value
// (unlimited) ReadHeaderTimeout is a well-known footgun (a client that
// sends headers one byte at a time ties up a connection, and the
// goroutine/file descriptor behind it, indefinitely; "slowloris").
// ReadTimeout/WriteTimeout are deliberately left at net/http's default
// (unlimited) rather than a fixed value here: the body is already bounded
// by maxBodyBytes (via io.LimitReader in node.go's handle), and the
// overall request-to-response time is already bounded per-node by
// httpin.Node's own configurable ResponseTimeoutMs (default 30s, node.go)
// - a fixed server-level WriteTimeout would silently cut off a flow
// deliberately configured with a longer ResponseTimeoutMs.
const (
    readHeaderTimeout = 10 * time.Second
    idleTimeout       = 120 * time.Second
)

type route struct {
    method  string
    pattern []segment
    handler func(w http.ResponseWriter, r *http.Request, params map[string]string)
    // raw, if set, is served directly (any HTTP method, exact path only -
    // no :param segments) instead of going through handler/pattern
    // matching. Used by websocket-listener to mount a raw upgrade handler
    // on the same shared listener as "http in".
    raw http.Handler
}

type segment struct {
    literal  string
    isParam  bool
    paramKey string
}

func parsePattern(path string) []segment {
    parts := strings.Split(strings.Trim(path, "/"), "/")
    segments := make([]segment, 0, len(parts))
    for _, p := range parts {
        if p == "" {
            continue
        }
        if strings.HasPrefix(p, ":") {
            segments = append(segments, segment{isParam: true, paramKey: p[1:]})
        } else {
            segments = append(segments, segment{literal: p})
        }
    }
    return segments
}

func (s segment) matches(part string) bool {
    return s.isParam || s.literal == part
}

// router dispatches incoming requests to registered "http in" nodes by
// (method, path) - a fixed literal/":param" subset of Node-RED's full
// Express routing (no wildcards, no regex), matching the same
// "reasonable-subset, not full parity" scope cut used by internal/nodes/
// htmlnode's CSS selectors (see docs/NODE_PALETTE_PLAN.md).
type router struct {
    mu     sync.RWMutex
    routes []*route
}

func (rt *router) add(method string, path string, handler func(w http.ResponseWriter, r *http.Request, params map[string]string)) *route {
    rt.mu.Lock()
    defer rt.mu.Unlock()
    r := &route{method: strings.ToUpper(method), pattern: parsePattern(path), handler: handler}
    rt.routes = append(rt.routes, r)
    return r
}

func (rt *router) addRaw(path string, handler http.Handler) *route {
    rt.mu.Lock()
    defer rt.mu.Unlock()
    r := &route{pattern: parsePattern(path), raw: handler}
    rt.routes = append(rt.routes, r)
    return r
}

func (rt *router) remove(target *route) {
    rt.mu.Lock()
    defer rt.mu.Unlock()
    for i, r := range rt.routes {
        if r == target {
            rt.routes = append(rt.routes[:i], rt.routes[i+1:]...)
            return
        }
    }
}

func (rt *router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    requestParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
    if len(requestParts) == 1 && requestParts[0] == "" {
        requestParts = nil
    }

    rt.mu.RLock()
    var matched *route
    var params map[string]string
    for _, route := range rt.routes {
        if route.raw != nil {
            if len(route.pattern) != len(requestParts) {
                continue
            }
            if pathLiteralMatch(route.pattern, requestParts) {
                matched = route
                break
            }
            continue
        }
        if route.method != r.Method || len(route.pattern) != len(requestParts) {
            continue
        }
        ok := true
        p := make(map[string]string)
        for i, seg := range route.pattern {
            if !seg.matches(requestParts[i]) {
                ok = false
                break
            }
            if seg.isParam {
                p[seg.paramKey] = requestParts[i]
            }
        }
        if ok {
            matched = route
            params = p
            break
        }
    }
    rt.mu.RUnlock()

    if matched == nil {
        http.NotFound(w, r)
        return
    }
    if matched.raw != nil {
        matched.raw.ServeHTTP(w, r)
        return
    }
    matched.handler(w, r, params)
}

func pathLiteralMatch(pattern []segment, requestParts []string) bool {
    for i, seg := range pattern {
        if !seg.matches(requestParts[i]) {
            return false
        }
    }
    return true
}

var (
    sharedRouter   = &router{}
    sharedOnce     sync.Once
    sharedStartErr error
    sharedAddr     string
    sharedAddrMu   sync.RWMutex
)

// ensureServerStarted lazily starts the shared HTTP listener the first time
// any "http in" node deploys. Errors (e.g. the port is already in use) are
// returned to the first caller and cached for subsequent ones. Binding
// GORED_HTTP_NODE_PORT to "0" (as tests do) picks a random free port,
// discoverable afterward via SharedAddr.
func ensureServerStarted() error {
    sharedOnce.Do(func() {
        port := os.Getenv(portEnvVar)
        if port == "" {
            port = defaultPort
        }
        ln, err := net.Listen("tcp", ":"+port)
        if err != nil {
            sharedStartErr = err
            return
        }
        sharedAddrMu.Lock()
        sharedAddr = ln.Addr().String()
        sharedAddrMu.Unlock()

        server := &http.Server{
            Handler:           sharedRouter,
            ReadHeaderTimeout: readHeaderTimeout,
            IdleTimeout:       idleTimeout,
        }
        go func() {
            if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
                log.Printf("[http in] listener stopped: %v", err)
            }
        }()
        log.Printf("[http in] listening on %s", sharedAddr)
    })
    return sharedStartErr
}

// SharedAddr returns the shared "http in" listener's actual address
// (host:port), once ensureServerStarted has run at least once. Used by
// tests (and the Phase 6 engine milestone test) that bind to port "0" and
// need to discover the real port afterward.
func SharedAddr() string {
    sharedAddrMu.RLock()
    defer sharedAddrMu.RUnlock()
    return sharedAddr
}

// RegisterHandler mounts handler at path (any HTTP method, exact match
// only) on the shared "http in" listener, starting it first if needed.
// Used by websocket-listener to serve WS upgrades on the same port as
// http in/http response. The returned unregister function removes the
// mount; the shared listener itself keeps running for other nodes.
func RegisterHandler(path string, handler http.Handler) (unregister func(), err error) {
    if err := ensureServerStarted(); err != nil {
        return nil, err
    }
    r := sharedRouter.addRaw(path, handler)
    return func() { sharedRouter.remove(r) }, nil
}
