// Package tcpin provides the "tcp in" node implementation - either listens
// for TCP connections (Server mode) or connects out to a remote host
// (client mode), emitting one message per line or per raw read from every
// connection.
//
// Node-RED's tcp in supports a configurable split character/count/time and
// a shared-connection "server" concept keyed by host:port that a matching
// tcp out can also send through. This implementation keeps each node
// self-contained instead (no shared-connection registry): SplitLines picks
// between newline-delimited (bufio.Scanner's default line splitting - "\n",
// tolerant of a preceding "\r") and raw-chunk framing; a configurable
// arbitrary delimiter/byte-count/time-based split is not implemented. See
// docs/NODE_PALETTE_PLAN.md for this documented scope cut.
package tcpin

import (
    "bufio"
    "context"
    "fmt"
    "net"
    "strconv"
    "sync"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a TCP-in node's configuration.
type Node struct {
    // Host is used only in client mode (Server == false).
    Host       string
    Port       int
    Server     bool
    SplitLines bool
    // Datatype is "buffer" (default: msg.payload is []byte) or "utf8"
    // (msg.payload is a string).
    Datatype string

    mu       sync.Mutex
    listener net.Listener
    conns    map[net.Conn]struct{}
}

// Start listens (Server mode) or connects out (client mode) and emits one
// message per line/chunk received on every connection, until ctx is
// cancelled.
func (n *Node) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
    n.mu.Lock()
    n.conns = make(map[net.Conn]struct{})
    n.mu.Unlock()

    if n.Server {
        return n.runServer(ctx, emit)
    }
    return n.runClient(ctx, emit)
}

func (n *Node) runServer(ctx context.Context, emit func(map[string]interface{})) error {
    ln, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port))
    if err != nil {
        return fmt.Errorf("tcp in: %w", err)
    }
    n.mu.Lock()
    n.listener = ln
    n.mu.Unlock()

    go func() {
        for {
            conn, err := ln.Accept()
            if err != nil {
                return
            }
            n.trackConn(conn)
            go n.readConn(conn, emit)
        }
    }()

    <-ctx.Done()
    n.closeAll()
    return nil
}

func (n *Node) runClient(ctx context.Context, emit func(map[string]interface{})) error {
    conn, err := net.Dial("tcp", net.JoinHostPort(n.Host, strconv.Itoa(n.Port)))
    if err != nil {
        return fmt.Errorf("tcp in: %w", err)
    }
    n.trackConn(conn)

    done := make(chan struct{})
    go func() {
        n.readConn(conn, emit)
        close(done)
    }()

    select {
    case <-ctx.Done():
        n.closeAll()
    case <-done:
    }
    return nil
}

func (n *Node) trackConn(conn net.Conn) {
    n.mu.Lock()
    n.conns[conn] = struct{}{}
    n.mu.Unlock()
}

func (n *Node) closeAll() {
    n.mu.Lock()
    defer n.mu.Unlock()
    if n.listener != nil {
        n.listener.Close()
    }
    for c := range n.conns {
        c.Close()
    }
    n.conns = nil
}

func (n *Node) readConn(conn net.Conn, emit func(map[string]interface{})) {
    defer func() {
        n.mu.Lock()
        delete(n.conns, conn)
        n.mu.Unlock()
        conn.Close()
    }()

    remote := conn.RemoteAddr().String()
    if n.SplitLines {
        scanner := bufio.NewScanner(conn)
        for scanner.Scan() {
            emit(n.buildMessage(scanner.Bytes(), remote))
        }
        return
    }

    buf := make([]byte, 4096)
    for {
        nRead, err := conn.Read(buf)
        if nRead > 0 {
            emit(n.buildMessage(buf[:nRead], remote))
        }
        if err != nil {
            return
        }
    }
}

func (n *Node) buildMessage(data []byte, remote string) map[string]interface{} {
    var payload interface{} = append([]byte(nil), data...)
    if n.Datatype == "utf8" {
        payload = string(data)
    }
    return map[string]interface{}{"payload": payload, "ip": remote}
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.EmittingNode) - TCP in has no input port; the engine never
// calls it for a node with no wired input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("tcp in: has no input port")
}

func (n *Node) Validate() error {
    if n.Port <= 0 || n.Port > 65535 {
        return fmt.Errorf("tcp in: port must be between 1 and 65535")
    }
    if !n.Server && n.Host == "" {
        return fmt.Errorf("tcp in: host is required in client mode")
    }
    switch n.Datatype {
    case "", "buffer", "utf8":
    default:
        return fmt.Errorf("tcp in: unsupported datatype %q", n.Datatype)
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "host":       n.Host,
        "port":       float64(n.Port),
        "server":     n.Server,
        "splitLines": n.SplitLines,
        "datatype":   n.Datatype,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["host"].(string); ok {
        n.Host = v
    }
    if v, ok := config["port"].(float64); ok {
        n.Port = int(v)
    }
    if v, ok := config["server"].(bool); ok {
        n.Server = v
    }
    if v, ok := config["splitLines"].(bool); ok {
        n.SplitLines = v
    }
    n.Datatype = "buffer"
    if v, ok := config["datatype"].(string); ok && v != "" {
        n.Datatype = v
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("tcp in", func() registry.NodeExecutor {
        return &Node{Server: true, Datatype: "buffer"}
    }, registry.NodeMetadata{
        ID:          "tcp in",
        Type:        "tcp in",
        Name:        "TCP in",
        Description: "Listens for TCP connections (server mode) or connects out (client mode), emitting one message per line or raw read",
        Category:    "network",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "One message per line/chunk received", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "host":       {Type: "string", Description: "Remote host (client mode only)", Default: ""},
                "port":       {Type: "number", Description: "Port to listen on (server mode) or connect to (client mode)", Default: float64(0), Min: floatPtr(1), Max: floatPtr(65535)},
                "server":     {Type: "boolean", Description: "true: listen for connections; false: connect out to host:port", Default: true},
                "splitLines": {Type: "boolean", Description: "Split incoming data into one message per line instead of per raw read", Default: false},
                "datatype":   {Type: "string", Description: "buffer (raw bytes) or utf8 (string)", Default: "buffer", Enum: []string{"buffer", "utf8"}},
            },
            Required: []string{"port"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"network", "tcp", "server"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
