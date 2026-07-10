// Package tcprequest provides the "tcp request" node implementation -
// connects out, writes msg.payload, half-closes the write side (signaling
// EOF to the peer, the common TCP request/response idiom), and returns
// whatever the peer sends back before closing its own side (or before
// TimeoutMs/maxReplyBytes is hit).
package tcprequest

import (
    "encoding/json"
    "fmt"
    "io"
    "net"
    "strconv"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

const (
    defaultTimeout = 30 * time.Second
    maxReplyBytes  = 10 << 20 // 10 MiB
)

// Node holds a TCP-request node's configuration.
type Node struct {
    Host      string
    Port      int
    TimeoutMs int64
    // Datatype is "buffer" (default: msg.payload is []byte) or "utf8"
    // (msg.payload is a string) for the reply.
    Datatype string
}

// Execute connects to Host:Port, writes input's payload, half-closes the
// connection, and reads the reply until the peer closes its side, the
// reply exceeds maxReplyBytes, or TimeoutMs elapses.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    data, err := toBytes(input["payload"])
    if err != nil {
        return nil, fmt.Errorf("tcp request: %w", err)
    }

    timeout := time.Duration(n.TimeoutMs) * time.Millisecond
    if timeout <= 0 {
        timeout = defaultTimeout
    }

    addr := net.JoinHostPort(n.Host, strconv.Itoa(n.Port))
    conn, err := net.DialTimeout("tcp", addr, timeout)
    if err != nil {
        return nil, fmt.Errorf("tcp request: %w", err)
    }
    defer conn.Close()
    conn.SetDeadline(time.Now().Add(timeout))

    if _, err := conn.Write(data); err != nil {
        return nil, fmt.Errorf("tcp request: %w", err)
    }
    if tcpConn, ok := conn.(*net.TCPConn); ok {
        tcpConn.CloseWrite()
    }

    reply, err := io.ReadAll(io.LimitReader(conn, maxReplyBytes))
    if err != nil {
        return nil, fmt.Errorf("tcp request: reading reply: %w", err)
    }

    out := cloneMap(input)
    if n.Datatype == "utf8" {
        out["payload"] = string(reply)
    } else {
        out["payload"] = reply
    }
    return out, nil
}

func toBytes(payload interface{}) ([]byte, error) {
    switch v := payload.(type) {
    case []byte:
        return v, nil
    case string:
        return []byte(v), nil
    case nil:
        return []byte{}, nil
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

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func (n *Node) Validate() error {
    if n.Host == "" {
        return fmt.Errorf("tcp request: host is required")
    }
    if n.Port <= 0 || n.Port > 65535 {
        return fmt.Errorf("tcp request: port must be between 1 and 65535")
    }
    switch n.Datatype {
    case "", "buffer", "utf8":
    default:
        return fmt.Errorf("tcp request: unsupported datatype %q", n.Datatype)
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "host":      n.Host,
        "port":      float64(n.Port),
        "timeoutMs": n.TimeoutMs,
        "datatype":  n.Datatype,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["host"].(string); ok {
        n.Host = v
    }
    if v, ok := config["port"].(float64); ok {
        n.Port = int(v)
    }
    if v, ok := config["timeoutMs"].(float64); ok {
        n.TimeoutMs = int64(v)
    }
    n.Datatype = "buffer"
    if v, ok := config["datatype"].(string); ok && v != "" {
        n.Datatype = v
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("tcp request", func() registry.NodeExecutor {
        return &Node{Datatype: "buffer"}
    }, registry.NodeMetadata{
        ID:          "tcp request",
        Type:        "tcp request",
        Name:        "TCP request",
        Description: "Connects out, writes msg.payload, and returns the peer's reply",
        Category:    "network",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to send", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "payload=the peer's reply", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "host":      {Type: "string", Description: "Remote host", Default: ""},
                "port":      {Type: "number", Description: "Remote port", Default: float64(0), Min: floatPtr(1), Max: floatPtr(65535)},
                "timeoutMs": {Type: "number", Description: "Overall timeout in milliseconds", Default: float64(30000), Min: floatPtr(0)},
                "datatype":  {Type: "string", Description: "buffer (raw bytes) or utf8 (string) for the reply", Default: "buffer", Enum: []string{"buffer", "utf8"}},
            },
            Required: []string{"host", "port"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"network", "tcp", "request"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
