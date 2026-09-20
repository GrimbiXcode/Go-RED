// Package udpout provides the "udp out" node implementation - sends
// msg.payload as a single UDP datagram to Host:Port.
//
// Node-RED's udp out also supports broadcasting (sending to
// 255.255.255.255, which needs the SO_BROADCAST socket option) and
// multicast; neither is implemented here (unicast only) - see
// docs/NODE_PALETTE_PLAN.md for this documented scope cut.
package udpout

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/GrimbiXcode/Go-RED/internal/nodes/base"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a UDP-out node's configuration.
type Node struct {
	Host string
	Port int
}

// Execute sends input's payload as a single UDP datagram to Host:Port.
// Host resolution (which may involve DNS) and the send are bounded by the
// per-message context.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	c := base.Context(ctx)

	data, err := base.ToBytes(input["payload"])
	if err != nil {
		return nil, fmt.Errorf("udp out: %w", err)
	}

	// net.Dialer.DialContext on "udp" resolves Host under ctx and returns a
	// connected (fixed-peer) UDP socket, the same as net.DialUDP would.
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(c, "udp", net.JoinHostPort(n.Host, strconv.Itoa(n.Port)))
	if err != nil {
		return nil, fmt.Errorf("udp out: %w", wrapCtxErr(c, err))
	}
	defer conn.Close()

	if d, ok := c.Deadline(); ok {
		_ = conn.SetWriteDeadline(d)
	}
	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("udp out: %w", wrapCtxErr(c, err))
	}
	return input, nil
}

// wrapCtxErr attaches the context's own error (context.Canceled or
// context.DeadlineExceeded) to a network error that was caused by that
// context ending, so callers can errors.Is against the context error.
func wrapCtxErr(ctx context.Context, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return fmt.Errorf("%w: %w", cerr, err)
	}
	return err
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
				"host": {
					Type:        "string",
					Description: "Remote host",
					Default:     "",
					Label:       "Host",
					Order:       1,
					Widget:      "text",
				},
				"port": {
					Type:        "number",
					Description: "Remote port",
					Default:     float64(0),
					Min:         base.FloatPtr(1),
					Max:         base.FloatPtr(65535),
					Label:       "Port",
					Order:       2,
					Widget:      "number",
				},
			},
			Required: []string{"host", "port"},
		},
		Help: "**Sends `msg.payload` as a UDP datagram** to the given host and port.",
		Icon: "satellite",
		Tags: []string{"network", "udp", "client"},
	})
	if err != nil {
		panic(err)
	}
}
