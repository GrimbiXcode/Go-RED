package catch

import (
    "context"
    "errors"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"scope": []interface{}{"a", "b"}}))
    assert.Equal(t, []string{"a", "b"}, n.Scope)
    assert.Equal(t, map[string]interface{}{"scope": []interface{}{"a", "b"}}, n.GetConfig())
}

func TestNode_InScope(t *testing.T) {
    t.Run("empty scope matches every node", func(t *testing.T) {
        n := &Node{}
        assert.True(t, n.inScope("anything"))
    })

    t.Run("non-empty scope matches only listed node IDs", func(t *testing.T) {
        n := &Node{Scope: []string{"a", "b"}}
        assert.True(t, n.inScope("a"))
        assert.False(t, n.inScope("c"))
    })
}

func TestNode_Execute(t *testing.T) {
    n := &Node{}
    input := map[string]interface{}{"payload": 1}
    output, err := n.Execute(nil, input)
    require.NoError(t, err)
    assert.Equal(t, input, output)
}

func TestNode_Start(t *testing.T) {
    t.Run("emits an error-augmented message for an in-scope error", func(t *testing.T) {
        n := &Node{}
        bus := registry.NewEventBus()
        rt := registry.NewNodeRuntime("flow-1", "catch-1", "catch", nil, nil, bus, nil, nil)
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

        // Give Start a moment to subscribe before publishing.
        time.Sleep(20 * time.Millisecond)
        bus.PublishError(registry.NodeErrorEvent{
            NodeID:   "failing-node",
            NodeType: "some-type",
            Err:      errors.New("boom"),
            Payload:  map[string]interface{}{"payload": "original"},
        })

        select {
        case <-done:
        case <-time.After(time.Second):
            t.Fatal("emit was never called")
        }

        mu.Lock()
        defer mu.Unlock()
        assert.Equal(t, "original", got["payload"])
        errInfo, ok := got["error"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "boom", errInfo["message"])
        source, ok := errInfo["source"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "failing-node", source["id"])
        assert.Equal(t, "some-type", source["type"])
    })

    t.Run("does not emit for an out-of-scope error", func(t *testing.T) {
        n := &Node{Scope: []string{"only-this-one"}}
        bus := registry.NewEventBus()
        rt := registry.NewNodeRuntime("flow-1", "catch-1", "catch", nil, nil, bus, nil, nil)
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        rtCtx := registry.WithRuntime(ctx, rt)

        called := false
        go func() {
            _ = n.Start(rtCtx, func(payload map[string]interface{}) { called = true })
        }()

        time.Sleep(20 * time.Millisecond)
        bus.PublishError(registry.NodeErrorEvent{NodeID: "someone-else", Err: errors.New("boom")})
        time.Sleep(20 * time.Millisecond)

        assert.False(t, called)
    })

    t.Run("returns when ctx is cancelled", func(t *testing.T) {
        n := &Node{}
        bus := registry.NewEventBus()
        rt := registry.NewNodeRuntime("flow-1", "catch-1", "catch", nil, nil, bus, nil, nil)
        ctx, cancel := context.WithCancel(context.Background())
        rtCtx := registry.WithRuntime(ctx, rt)

        done := make(chan error, 1)
        go func() { done <- n.Start(rtCtx, func(map[string]interface{}) {}) }()

        cancel()

        select {
        case err := <-done:
            assert.NoError(t, err)
        case <-time.After(time.Second):
            t.Fatal("Start did not return after context cancellation")
        }
    })
}
