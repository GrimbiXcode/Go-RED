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
    "fmt"
    "sync"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/gorilla/websocket"
)

const reconnectInterval = 5 * time.Second

// Node holds a WebSocket-client node's configuration and connection.
type Node struct {
    URL string

    mu        sync.Mutex
    conn      *websocket.Conn
    onMessage []func(data []byte, isText bool)
    stopCh    chan struct{}
    closed    bool
}

func (n *Node) connectLoop() {
    for {
        select {
        case <-n.stopCh:
            return
        default:
        }

        conn, _, err := websocket.DefaultDialer.Dial(n.URL, nil)
        if err != nil {
            select {
            case <-n.stopCh:
                return
            case <-time.After(reconnectInterval):
                continue
            }
        }

        n.mu.Lock()
        n.conn = conn
        n.mu.Unlock()

        n.readLoop(conn)

        n.mu.Lock()
        if n.conn == conn {
            n.conn = nil
        }
        n.mu.Unlock()

        select {
        case <-n.stopCh:
            return
        case <-time.After(reconnectInterval):
        }
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

// Send writes data to the current connection, if any.
func (n *Node) Send(data []byte, isText bool) error {
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
    return conn.WriteMessage(msgType, data)
}

// OnMessage registers handler to run for every message received, for as
// long as this node is deployed (across reconnects).
func (n *Node) OnMessage(handler func(data []byte, isText bool)) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.onMessage = append(n.onMessage, handler)
}

// Close stops the reconnect loop and closes the current connection, if any.
func (n *Node) Close() error {
    n.mu.Lock()
    if n.closed {
        n.mu.Unlock()
        return nil
    }
    n.closed = true
    close(n.stopCh)
    conn := n.conn
    n.mu.Unlock()

    if conn != nil {
        conn.Close()
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
    n.stopCh = make(chan struct{})
    go n.connectLoop()
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
                "url": {Type: "string", Description: "WebSocket URL to connect to, e.g. ws://example.com/socket", Default: ""},
            },
            Required: []string{"url"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"config", "websocket", "client"},
    })
    if err != nil {
        panic(err)
    }
}
