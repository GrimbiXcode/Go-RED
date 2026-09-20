// Package tcprequest provides the "tcp request" node implementation -
// connects out, writes msg.payload, half-closes the write side (signaling
// EOF to the peer, the common TCP request/response idiom), and returns
// whatever the peer sends back before closing its own side (or before
// TimeoutMs/maxReplyBytes is hit).
package tcprequest

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/nodes/base"
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
// reply exceeds maxReplyBytes, TimeoutMs elapses, or the per-message
// context ends (its deadline, if earlier than TimeoutMs, is the effective
// one; its cancellation closes the connection so the blocking read
// returns).
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	c := base.Context(ctx)

	data, err := base.ToBytes(input["payload"])
	if err != nil {
		return nil, fmt.Errorf("tcp request: %w", err)
	}

	timeout := time.Duration(n.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	deadline := time.Now().Add(timeout)
	if d, ok := c.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	addr := net.JoinHostPort(n.Host, strconv.Itoa(n.Port))
	dialer := &net.Dialer{Deadline: deadline}
	conn, err := dialer.DialContext(c, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp request: %w", wrapCtxErr(c, err))
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(c, func() { conn.Close() })
	defer stop()

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("tcp request: %w", wrapCtxErr(c, err))
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	reply, err := io.ReadAll(io.LimitReader(conn, maxReplyBytes))
	if err != nil {
		return nil, fmt.Errorf("tcp request: reading reply: %w", wrapCtxErr(c, err))
	}

	out := base.CloneMap(input)
	if n.Datatype == "utf8" {
		out["payload"] = string(reply)
	} else {
		out["payload"] = reply
	}
	return out, nil
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
				"datatype": {
					Type:        "string",
					Description: "buffer (raw bytes) or utf8 (string) for the reply",
					Default:     "buffer",
					Label:       "Data type",
					Order:       3,
					Widget:      "select",
					Options:     []registry.Option{{Value: "buffer", Label: "Buffer (base64)"}, {Value: "utf8", Label: "Text (UTF-8)"}},
				},
				"timeoutMs": {
					Type:        "number",
					Description: "Overall timeout in milliseconds",
					Default:     float64(30000),
					Min:         base.FloatPtr(0),
					Label:       "Timeout",
					Order:       4,
					Widget:      "duration",
					Unit:        "ms",
				},
			},
			Required: []string{"host", "port"},
		},
		Help: "**Sends `msg.payload` over TCP and waits for the reply**, which becomes the outgoing `msg.payload`.",
		Icon: "arrow-left-right",
		Tags: []string{"network", "tcp", "request"},
	})
	if err != nil {
		panic(err)
	}
}
