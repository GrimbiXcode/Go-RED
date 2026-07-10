// Package websocketin provides the "websocket in" node implementation -
// emits one message per message received on a shared websocket-listener or
// websocket-client config node (internal/nodes/websocketlistener,
// internal/nodes/websocketclient - either works, since both expose the
// same OnMessage method this node type-asserts for).
package websocketin

import (
    "context"
    "fmt"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Provider is implemented by websocket-listener and websocket-client.
type Provider interface {
    OnMessage(handler func(data []byte, isText bool))
}

// Node holds a WebSocket-in node's configuration.
type Node struct {
    // Server is a websocket-listener or websocket-client config node's ID.
    Server string
}

// Start resolves Server via registry.NodeRuntime.GetNode and registers a
// handler that emits one message per received WebSocket message, until ctx
// is cancelled.
func (n *Node) Start(ctx context.Context, emit func(payload map[string]interface{})) error {
    rt, ok := registry.RuntimeFromContext(ctx)
    if !ok {
        return fmt.Errorf("websocket in: no runtime available")
    }
    exec, ok := rt.GetNode(n.Server)
    if !ok {
        return fmt.Errorf("websocket in: server config node %q not found", n.Server)
    }
    provider, ok := exec.(Provider)
    if !ok {
        return fmt.Errorf("websocket in: node %q is not a websocket-listener/websocket-client", n.Server)
    }

    provider.OnMessage(func(data []byte, isText bool) {
        emit(map[string]interface{}{
            "payload": string(data),
            "isText":  isText,
        })
    })

    <-ctx.Done()
    return nil
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.EmittingNode) - WebSocket in has no input port; the engine
// never calls it for a node with no wired input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("websocket in: has no input port")
}

func (n *Node) Validate() error {
    if n.Server == "" {
        return fmt.Errorf("websocket in: server is required")
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
    err := reg.RegisterFactory("websocket in", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "websocket in",
        Type:        "websocket in",
        Name:        "WebSocket in",
        Description: "Emits one message per message received on a websocket-listener or websocket-client config node",
        Category:    "network",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "One message per received WebSocket message", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "server": {Type: "string", Description: "ID of an existing websocket-listener or websocket-client config node", Default: ""},
            },
            Required: []string{"server"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"network", "websocket", "subscribe"},
    })
    if err != nil {
        panic(err)
    }
}
