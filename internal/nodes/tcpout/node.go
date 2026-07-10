// Package tcpout provides the "tcp out" node implementation - connects out
// to a remote host:port and writes msg.payload, once per message (a new
// connection per Execute call, not a persistent one reused across
// messages).
//
// Node-RED's tcp out can also act as its own server, broadcasting to every
// client that has connected to it (sharing a "server" concept with a
// matching tcp in on the same host:port); that mode is not implemented
// here - use internal/nodes/tcpin in server mode to receive, and this node
// to send to a specific known remote address. See
// docs/NODE_PALETTE_PLAN.md for this documented scope cut.
package tcpout

import (
    "encoding/json"
    "fmt"
    "net"
    "strconv"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

const dialTimeout = 10 * time.Second

// Node holds a TCP-out node's configuration.
type Node struct {
    Host string
    Port int
}

// Execute dials Host:Port, writes input's payload, and closes the
// connection.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    data, err := toBytes(input["payload"])
    if err != nil {
        return nil, fmt.Errorf("tcp out: %w", err)
    }

    addr := net.JoinHostPort(n.Host, strconv.Itoa(n.Port))
    conn, err := net.DialTimeout("tcp", addr, dialTimeout)
    if err != nil {
        return nil, fmt.Errorf("tcp out: %w", err)
    }
    defer conn.Close()

    if _, err := conn.Write(data); err != nil {
        return nil, fmt.Errorf("tcp out: %w", err)
    }
    return input, nil
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

func (n *Node) Validate() error {
    if n.Host == "" {
        return fmt.Errorf("tcp out: host is required")
    }
    if n.Port <= 0 || n.Port > 65535 {
        return fmt.Errorf("tcp out: port must be between 1 and 65535")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"host": n.Host, "port": float64(n.Port)}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["host"].(string); ok {
        n.Host = v
    }
    if v, ok := config["port"].(float64); ok {
        n.Port = int(v)
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("tcp out", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "tcp out",
        Type:        "tcp out",
        Name:        "TCP out",
        Description: "Connects out to host:port and writes msg.payload (a new connection per message)",
        Category:    "network",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to send", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message after a successful send", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "host": {Type: "string", Description: "Remote host", Default: ""},
                "port": {Type: "number", Description: "Remote port", Default: float64(0), Min: floatPtr(1), Max: floatPtr(65535)},
            },
            Required: []string{"host", "port"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"network", "tcp", "client"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
