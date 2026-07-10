// Package websocketout provides the "websocket out" node implementation -
// sends msg.payload through a shared websocket-listener or
// websocket-client config node (internal/nodes/websocketlistener,
// internal/nodes/websocketclient).
package websocketout

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Provider is implemented by websocket-listener and websocket-client.
type Provider interface {
    Send(data []byte, isText bool) error
}

// Node holds a WebSocket-out node's configuration.
type Node struct {
    // Server is a websocket-listener or websocket-client config node's ID.
    Server string
}

// Execute sends input's payload (passed through as-is for a string/[]byte
// - sent as a text/binary frame respectively - JSON-encoded as a text
// frame otherwise) via the shared server/client connection.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    c, ok := ctx.(context.Context)
    if !ok {
        c = context.Background()
    }
    rt, ok := registry.RuntimeFromContext(c)
    if !ok {
        return nil, fmt.Errorf("websocket out: no runtime available")
    }
    exec, ok := rt.GetNode(n.Server)
    if !ok {
        return nil, fmt.Errorf("websocket out: server config node %q not found", n.Server)
    }
    provider, ok := exec.(Provider)
    if !ok {
        return nil, fmt.Errorf("websocket out: node %q is not a websocket-listener/websocket-client", n.Server)
    }

    data, isText, err := toFrame(input["payload"])
    if err != nil {
        return nil, fmt.Errorf("websocket out: %w", err)
    }
    if err := provider.Send(data, isText); err != nil {
        return nil, fmt.Errorf("websocket out: %w", err)
    }
    return input, nil
}

func toFrame(payload interface{}) (data []byte, isText bool, err error) {
    switch v := payload.(type) {
    case []byte:
        return v, false, nil
    case string:
        return []byte(v), true, nil
    case nil:
        return []byte{}, true, nil
    default:
        encoded, err := json.Marshal(v)
        if err != nil {
            return nil, false, fmt.Errorf("payload cannot be encoded: %w", err)
        }
        return encoded, true, nil
    }
}

func (n *Node) Validate() error {
    if n.Server == "" {
        return fmt.Errorf("websocket out: server is required")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"server": n.Server}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["server"].(string); ok {
        n.Server = v
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("websocket out", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "websocket out",
        Type:        "websocket out",
        Name:        "WebSocket out",
        Description: "Sends msg.payload through a websocket-listener or websocket-client config node",
        Category:    "network",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to send", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message after a successful send", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "server": {Type: "string", Description: "ID of an existing websocket-listener or websocket-client config node", Default: ""},
            },
            Required: []string{"server"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"network", "websocket", "publish"},
    })
    if err != nil {
        panic(err)
    }
}
