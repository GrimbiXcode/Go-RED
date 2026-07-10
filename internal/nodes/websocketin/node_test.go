package websocketin

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// fakeServer implements Provider (and registry.NodeExecutor, since
// NodeRuntime.GetNode returns that interface) without a real connection.
type fakeServer struct {
    mu       sync.Mutex
    handlers []func([]byte, bool)
}

func (f *fakeServer) OnMessage(handler func(data []byte, isText bool)) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.handlers = append(f.handlers, handler)
}

func (f *fakeServer) fire(data []byte, isText bool) {
    f.mu.Lock()
    handlers := append([]func([]byte, bool){}, f.handlers...)
    f.mu.Unlock()
    for _, h := range handlers {
        h(data, isText)
    }
}

func (f *fakeServer) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, nil
}
func (f *fakeServer) Validate() error                             { return nil }
func (f *fakeServer) GetConfig() map[string]interface{}           { return nil }
func (f *fakeServer) SetConfig(config map[string]interface{}) error { return nil }

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Server: "s1"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"server": "s1"}))
    assert.Equal(t, "s1", n.Server)
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Start_EmitsOnMessage(t *testing.T) {
    fs := &fakeServer{}
    rt := registry.NewNodeRuntime("f1", "wsin-1", "websocket in", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        if nodeID == "s1" {
            return fs, true
        }
        return nil, false
    })
    ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
    defer cancel()

    n := &Node{Server: "s1"}
    received := make(chan map[string]interface{}, 1)
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) { received <- payload })
    }()

    require.Eventually(t, func() bool {
        fs.mu.Lock()
        defer fs.mu.Unlock()
        return len(fs.handlers) > 0
    }, time.Second, 5*time.Millisecond)

    fs.fire([]byte("hello"), true)

    select {
    case msg := <-received:
        assert.Equal(t, "hello", msg["payload"])
        assert.Equal(t, true, msg["isText"])
    case <-time.After(time.Second):
        t.Fatal("emit was never called")
    }
}

func TestNode_Start_NoRuntime_Errors(t *testing.T) {
    n := &Node{Server: "s1"}
    err := n.Start(context.Background(), func(map[string]interface{}) {})
    assert.Error(t, err)
}

func TestNode_Start_UnknownServer_Errors(t *testing.T) {
    rt := registry.NewNodeRuntime("f1", "wsin-1", "websocket in", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        return nil, false
    })
    ctx := registry.WithRuntime(context.Background(), rt)
    n := &Node{Server: "missing"}
    err := n.Start(ctx, func(map[string]interface{}) {})
    assert.Error(t, err)
}
