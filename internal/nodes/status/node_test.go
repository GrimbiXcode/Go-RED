package status

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"scope": []interface{}{"a"}}))
    assert.Equal(t, []string{"a"}, n.Scope)
    assert.Equal(t, map[string]interface{}{"scope": []interface{}{"a"}}, n.GetConfig())
}

func TestNode_Execute(t *testing.T) {
    n := &Node{}
    input := map[string]interface{}{"payload": 1}
    output, err := n.Execute(nil, input)
    require.NoError(t, err)
    assert.Equal(t, input, output)
}

func TestNode_Start(t *testing.T) {
    t.Run("emits a message for an in-scope status change", func(t *testing.T) {
        n := &Node{}
        bus := registry.NewEventBus()
        rt := registry.NewNodeRuntime("flow-1", "status-1", "status", nil, nil, bus, nil, nil)
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        rtCtx := registry.WithRuntime(ctx, rt)

        var mu sync.Mutex
        var got map[string]interface{}
        done := make(chan struct{})
        go func() {
            _ = n.Start(rtCtx, func(payload map[string]interface{}) {
                mu.Lock()
                got = payload
                mu.Unlock()
                close(done)
            })
        }()

        time.Sleep(20 * time.Millisecond)
        bus.PublishStatus(registry.NodeStatusEvent{
            NodeID:   "mqtt-1",
            NodeType: "mqtt in",
            Status:   "connected",
            Detail:   "broker.local:1883",
        })

        select {
        case <-done:
        case <-time.After(time.Second):
            t.Fatal("emit was never called")
        }

        mu.Lock()
        defer mu.Unlock()
        st, ok := got["status"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "connected", st["value"])
        assert.Equal(t, "broker.local:1883", st["text"])
        source, ok := got["source"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "mqtt-1", source["id"])
    })

    t.Run("does not emit for an out-of-scope status change", func(t *testing.T) {
        n := &Node{Scope: []string{"only-this-one"}}
        bus := registry.NewEventBus()
        rt := registry.NewNodeRuntime("flow-1", "status-1", "status", nil, nil, bus, nil, nil)
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        rtCtx := registry.WithRuntime(ctx, rt)

        called := false
        go func() {
            _ = n.Start(rtCtx, func(payload map[string]interface{}) { called = true })
        }()

        time.Sleep(20 * time.Millisecond)
        bus.PublishStatus(registry.NodeStatusEvent{NodeID: "someone-else", Status: "ok"})
        time.Sleep(20 * time.Millisecond)

        assert.False(t, called)
    })
}
