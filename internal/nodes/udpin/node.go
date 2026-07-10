// Package udpin provides the "udp in" node implementation - listens for
// UDP datagrams on Port and emits one message per received datagram.
//
// Node-RED's udp in also supports joining a multicast group; that is not
// implemented here (unicast/broadcast reception only) - see
// docs/NODE_PALETTE_PLAN.md for this documented scope cut.
package udpin

import (
    "context"
    "fmt"
    "net"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

const maxDatagramBytes = 65535

// Node holds a UDP-in node's configuration.
type Node struct {
    Port int
    // Datatype is "buffer" (default: msg.payload is []byte) or "utf8"
    // (msg.payload is a string).
    Datatype string
}

// Start listens on Port and emits one message per received datagram until
// ctx is cancelled.
func (n *Node) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
    addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", n.Port))
    if err != nil {
        return fmt.Errorf("udp in: %w", err)
    }
    conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        return fmt.Errorf("udp in: %w", err)
    }

    done := make(chan struct{})
    go func() {
        defer close(done)
        buf := make([]byte, maxDatagramBytes)
        for {
            nRead, remote, err := conn.ReadFromUDP(buf)
            if nRead > 0 {
                emit(n.buildMessage(buf[:nRead], remote))
            }
            if err != nil {
                return
            }
        }
    }()

    select {
    case <-ctx.Done():
        conn.Close()
        <-done
    case <-done:
    }
    return nil
}

func (n *Node) buildMessage(data []byte, remote *net.UDPAddr) map[string]interface{} {
    var payload interface{} = append([]byte(nil), data...)
    if n.Datatype == "utf8" {
        payload = string(data)
    }
    return map[string]interface{}{
        "payload": payload,
        "ip":      remote.IP.String(),
        "port":    float64(remote.Port),
    }
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.EmittingNode) - UDP in has no input port; the engine never
// calls it for a node with no wired input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("udp in: has no input port")
}

func (n *Node) Validate() error {
    if n.Port <= 0 || n.Port > 65535 {
        return fmt.Errorf("udp in: port must be between 1 and 65535")
    }
    switch n.Datatype {
    case "", "buffer", "utf8":
    default:
        return fmt.Errorf("udp in: unsupported datatype %q", n.Datatype)
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"port": float64(n.Port), "datatype": n.Datatype}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["port"].(float64); ok {
        n.Port = int(v)
    }
    n.Datatype = "buffer"
    if v, ok := config["datatype"].(string); ok && v != "" {
        n.Datatype = v
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("udp in", func() registry.NodeExecutor {
        return &Node{Datatype: "buffer"}
    }, registry.NodeMetadata{
        ID:          "udp in",
        Type:        "udp in",
        Name:        "UDP in",
        Description: "Listens for UDP datagrams and emits one message per received datagram",
        Category:    "network",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "One message per received datagram", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "port":     {Type: "number", Description: "Port to listen on", Default: float64(0), Min: floatPtr(1), Max: floatPtr(65535)},
                "datatype": {Type: "string", Description: "buffer (raw bytes) or utf8 (string)", Default: "buffer", Enum: []string{"buffer", "utf8"}},
            },
            Required: []string{"port"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"network", "udp", "server"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
