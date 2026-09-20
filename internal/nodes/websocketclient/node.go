// Package websocketclient provides the "websocket-client" config node - an
// outgoing WebSocket connection to a remote URL, referenced by ID from
// websocket in/websocket out (see docs/NODE_PALETTE_PLAN.md, Phase 6's
// config-node concept).
//
// Like websocketlistener, the connection is established from SetConfig
// (via a background reconnect loop, not blocking Deploy), not from an
// EmittingNode.Start - see that package's doc comment for why.
package websocketclient

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/gorilla/websocket"
)

const (
	reconnectInterval = 5 * time.Second
	handshakeTimeout  = 10 * time.Second
)

// Node holds a WebSocket-client node's configuration and connection.
type Node struct {
	URL string

	mu        sync.Mutex
	conn      *websocket.Conn
	onMessage []func(data []byte, isText bool)
	// cancel ends the context created in SetConfig that the dial, the
	// reconnect back-off and the read loop are all bound to; Close calls it.
	cancel context.CancelFunc
	// loopDone is closed when connectLoop has returned.
	loopDone chan struct{}
	closed   bool

	// writeMu serializes writers: gorilla/websocket allows only one
	// concurrent writer per connection, and several websocket-out
	// Executes may run at once.
	writeMu sync.Mutex
}

func (n *Node) connectLoop(ctx context.Context) {
	defer close(n.loopDone)

	// gorilla's DialContext honors ctx for the TCP dial and turns
	// HandshakeTimeout into a socket deadline, but a cancel that arrives
	// while it is waiting for the handshake response is not noticed until
	// that deadline. Closing the socket from ctx makes the stalled
	// handshake return at once; the registration is dropped again as soon
	// as DialContext returns (NetDialContext runs synchronously inside it).
	var stopCloseOnCancel func() bool
	dialer := websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
		NetDialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			c, err := (&net.Dialer{}).DialContext(dialCtx, network, addr)
			if err != nil {
				return nil, err
			}
			stopCloseOnCancel = context.AfterFunc(ctx, func() { c.Close() })
			return c, nil
		},
	}
	for {
		if ctx.Err() != nil {
			return
		}

		stopCloseOnCancel = nil
		conn, _, err := dialer.DialContext(ctx, n.URL, nil)
		if stopCloseOnCancel != nil {
			stopCloseOnCancel()
		}
		if err != nil {
			if !sleepCtx(ctx, reconnectInterval) {
				return
			}
			continue
		}

		n.mu.Lock()
		if n.closed {
			// Close ran between the dial returning and this store: it
			// could not see conn, so close it here.
			n.mu.Unlock()
			conn.Close()
			return
		}
		n.conn = conn
		n.mu.Unlock()

		n.readLoop(conn)

		n.mu.Lock()
		if n.conn == conn {
			n.conn = nil
		}
		n.mu.Unlock()
		conn.Close()

		if !sleepCtx(ctx, reconnectInterval) {
			return
		}
	}
}

// sleepCtx waits for d, returning false early (without leaking the timer)
// if ctx ends first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (n *Node) readLoop(conn *websocket.Conn) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		n.dispatch(data, msgType == websocket.TextMessage)
	}
}

func (n *Node) dispatch(data []byte, isText bool) {
	n.mu.Lock()
	handlers := append([]func([]byte, bool){}, n.onMessage...)
	n.mu.Unlock()
	for _, h := range handlers {
		h(data, isText)
	}
}

// Send writes data to the current connection, if any, with no bound on
// how long the write may block; SendContext is what websocket-out uses.
func (n *Node) Send(data []byte, isText bool) error {
	return n.SendContext(context.Background(), data, isText)
}

// SendContext writes data to the current connection, if any. The write is
// bounded by ctx: an already-ended ctx fails immediately, and ctx's
// deadline (if any) becomes the write deadline.
func (n *Node) SendContext(ctx context.Context, data []byte, isText bool) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("websocket-client: %w", err)
	}
	n.mu.Lock()
	conn := n.conn
	n.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("websocket-client: not connected")
	}
	msgType := websocket.BinaryMessage
	if isText {
		msgType = websocket.TextMessage
	}

	n.writeMu.Lock()
	defer n.writeMu.Unlock()
	deadline, _ := ctx.Deadline() // zero time clears any earlier deadline
	_ = conn.SetWriteDeadline(deadline)
	return conn.WriteMessage(msgType, data)
}

// OnMessage registers handler to run for every message received, for as
// long as this node is deployed (across reconnects).
func (n *Node) OnMessage(handler func(data []byte, isText bool)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.onMessage = append(n.onMessage, handler)
}

// Close stops the reconnect loop (interrupting a dial or back-off wait in
// progress), closes the current connection, if any, and waits for the loop
// goroutine to exit.
func (n *Node) Close() error {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return nil
	}
	n.closed = true
	cancel := n.cancel
	loopDone := n.loopDone
	conn := n.conn
	n.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if conn != nil {
		conn.Close()
	}
	if loopDone != nil {
		<-loopDone
	}
	return nil
}

// Execute exists only to satisfy registry.NodeExecutor - a config node has
// no message inputs/outputs of its own and is never wired into a flow's
// message path.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("websocket-client: config nodes have no message input")
}

func (n *Node) Validate() error {
	if n.URL == "" {
		return fmt.Errorf("websocket-client: url is required")
	}
	return nil
}

func (n *Node) GetConfig() map[string]interface{} {
	return map[string]interface{}{"url": n.URL}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
	if v, ok := config["url"].(string); ok {
		n.URL = v
	}
	if err := n.Validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	n.mu.Lock()
	n.cancel = cancel
	n.loopDone = make(chan struct{})
	n.mu.Unlock()
	go n.connectLoop(ctx)
	return nil
}

func init() {
	reg := registry.GetGlobalRegistry()
	err := reg.RegisterFactory("websocket-client", func() registry.NodeExecutor {
		return &Node{}
	}, registry.NodeMetadata{
		ID:          "websocket-client",
		Type:        "websocket-client",
		Name:        "WebSocket Client",
		Description: "Outgoing WebSocket connection to a remote URL (auto-reconnecting), referenced by ID from websocket in/out",
		Category:    "config",
		Inputs:      []registry.Port{},
		Outputs:     []registry.Port{},
		ConfigSchema: registry.Schema{
			Properties: map[string]registry.Property{
				"url": {
					Type:        "string",
					Description: "WebSocket URL to connect to, e.g. ws://example.com/socket",
					Default:     "",
					Label:       "URL",
					Placeholder: "ws://host:port/path",
					Order:       1,
					Widget:      "text",
				},
			},
			Required: []string{"url"},
		},
		Help: "**Outgoing WebSocket connection** shared by `websocket in` and `websocket out` nodes. The connection is kept open and re-established when it drops.",
		Icon: "plug",
		Tags: []string{"config", "websocket", "client"},
	})
	if err != nil {
		panic(err)
	}
}
