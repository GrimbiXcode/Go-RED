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

// ContextProvider is the context-aware variant both config nodes also
// implement: the send is bounded by ctx (its deadline becomes the write
// deadline; an already-ended ctx fails at once). Execute prefers it over
// Provider.Send whenever the provider offers it.
type ContextProvider interface {
	SendContext(ctx context.Context, data []byte, isText bool) error
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
	if err := c.Err(); err != nil {
		return nil, fmt.Errorf("websocket out: %w", err)
	}
	if cp, ok := provider.(ContextProvider); ok {
		err = cp.SendContext(c, data, isText)
	} else {
		err = provider.Send(data, isText)
	}
	if err != nil {
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
				"server": {
					Type:        "string",
					Description: "ID of an existing websocket-listener or websocket-client config node",
					Default:     "",
					Label:       "Connection",
					Order:       1,
					Widget:      "nodeSelect",
					NodeTypes:   []string{"websocket-listener", "websocket-client"},
				},
			},
			Required: []string{"server"},
		},
		Help: "**Sends `msg.payload` over WebSocket**: to every connected client of a listener, or to the server of a client connection.",
		Icon: "plug-zap",
		Tags: []string{"network", "websocket", "publish"},
	})
	if err != nil {
		panic(err)
	}
}
