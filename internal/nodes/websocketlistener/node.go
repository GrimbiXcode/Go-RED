// Package websocketlistener provides the "websocket-listener" config node -
// a server-side WebSocket endpoint mounted on the shared "http in" listener
// (internal/nodes/httpin), referenced by ID from websocket in/websocket out
// (see docs/NODE_PALETTE_PLAN.md, Phase 6's config-node concept).
//
// Connection setup happens in SetConfig, not in an EmittingNode.Start, for
// the same reason internal/nodes/mqttbroker does: SetConfig runs
// synchronously inside Deploy's single-node-at-a-time initialization loop,
// which always finishes before any Start goroutine is launched, so a
// consumer resolving this node via registry.NodeRuntime.GetNode from its
// own Start is guaranteed the handler is already mounted - no ordering
// race against internal/engine/engine.go's startEmittingNodes, which gives
// no ordering guarantee between different EmittingNode goroutines.
package websocketlistener

import (
    "fmt"
    "net/http"
    "sync"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  4096,
    WriteBufferSize: 4096,
    CheckOrigin:     func(r *http.Request) bool { return true },
}

// Node holds a WebSocket-listener node's configuration and connected
// clients.
type Node struct {
    Path string

    mu         sync.Mutex
    conns      map[*websocket.Conn]struct{}
    onMessage  []func(data []byte, isText bool)
    unregister func()
}

// Send broadcasts data to every currently connected client.
func (n *Node) Send(data []byte, isText bool) error {
    msgType := websocket.BinaryMessage
    if isText {
        msgType = websocket.TextMessage
    }

    n.mu.Lock()
    conns := make([]*websocket.Conn, 0, len(n.conns))
    for c := range n.conns {
        conns = append(conns, c)
    }
    n.mu.Unlock()

    var firstErr error
    for _, c := range conns {
        if err := c.WriteMessage(msgType, data); err != nil && firstErr == nil {
            firstErr = err
        }
    }
    return firstErr
}

// OnMessage registers handler to run for every message received from any
// connected client, for as long as this node is deployed.
func (n *Node) OnMessage(handler func(data []byte, isText bool)) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.onMessage = append(n.onMessage, handler)
}

func (n *Node) dispatch(data []byte, isText bool) {
    n.mu.Lock()
    handlers := append([]func([]byte, bool){}, n.onMessage...)
    n.mu.Unlock()
    for _, h := range handlers {
        h(data, isText)
    }
}

func (n *Node) handleUpgrade(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    n.mu.Lock()
    n.conns[conn] = struct{}{}
    n.mu.Unlock()
    defer func() {
        n.mu.Lock()
        delete(n.conns, conn)
        n.mu.Unlock()
        conn.Close()
    }()

    for {
        msgType, data, err := conn.ReadMessage()
        if err != nil {
            return
        }
        n.dispatch(data, msgType == websocket.TextMessage)
    }
}

// Close unmounts the upgrade handler and closes every connected client.
func (n *Node) Close() error {
    n.mu.Lock()
    unregister := n.unregister
    conns := n.conns
    n.conns = nil
    n.mu.Unlock()

    if unregister != nil {
        unregister()
    }
    for c := range conns {
        c.Close()
    }
    return nil
}

// Execute exists only to satisfy registry.NodeExecutor - a config node has
// no message inputs/outputs of its own and is never wired into a flow's
// message path.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("websocket-listener: config nodes have no message input")
}

func (n *Node) Validate() error {
    if n.Path == "" {
        return fmt.Errorf("websocket-listener: path is required")
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"path": n.Path}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["path"].(string); ok {
        n.Path = v
    }
    if err := n.Validate(); err != nil {
        return err
    }
    n.conns = make(map[*websocket.Conn]struct{})
    unregister, err := httpin.RegisterHandler(n.Path, http.HandlerFunc(n.handleUpgrade))
    if err != nil {
        return fmt.Errorf("websocket-listener: %w", err)
    }
    n.unregister = unregister
    return nil
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("websocket-listener", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "websocket-listener",
        Type:        "websocket-listener",
        Name:        "WebSocket Listener",
        Description: "Server-side WebSocket endpoint on the shared http-in listener, referenced by ID from websocket in/out",
        Category:    "config",
        Inputs:      []registry.Port{},
        Outputs:     []registry.Port{},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "path": {Type: "string", Description: "WebSocket endpoint path, e.g. /ws/echo", Default: ""},
            },
            Required: []string{"path"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 9l3 3-3 3M13 15h4"/></svg>`,
        Tags: []string{"config", "websocket", "server"},
    })
    if err != nil {
        panic(err)
    }
}
