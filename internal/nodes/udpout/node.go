// Package udpout provides the "udp out" node implementation - sends
// msg.payload as a single UDP datagram to Host:Port.
//
// Node-RED's udp out also supports broadcasting (sending to
// 255.255.255.255, which needs the SO_BROADCAST socket option) and
// multicast; neither is implemented here (unicast only) - see
// docs/NODE_PALETTE_PLAN.md for this documented scope cut.
package udpout

import (
    "encoding/json"
    "fmt"
    "net"
    "strconv"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a UDP-out node's configuration.
type Node struct {
    Host string
    Port int
}

// Execute sends input's payload as a single UDP datagram to Host:Port.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    data, err := toBytes(input["payload"])
    if err != nil {
        return nil, fmt.Errorf("udp out: %w", err)
    }

    addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(n.Host, strconv.Itoa(n.Port)))
    if err != nil {
        return nil, fmt.Errorf("udp out: %w", err)
    }
    conn, err := net.DialUDP("udp", nil, addr)
    if err != nil {
        return nil, fmt.Errorf("udp out: %w", err)
    }
    defer conn.Close()

    if _, err := conn.Write(data); err != nil {
        return nil, fmt.Errorf("udp out: %w", err)
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
        return fmt.Errorf("udp out: host is required")
    }
    if n.Port <= 0 || n.Port > 65535 {
        return fmt.Errorf("udp out: port must be between 1 and 65535")
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
    err := reg.RegisterFactory("udp out", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "udp out",
        Type:        "udp out",
        Name:        "UDP out",
        Description: "Sends msg.payload as a single UDP datagram to host:port",
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
        Tags: []string{"network", "udp", "client"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
