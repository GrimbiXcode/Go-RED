package trigger

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "firstPayload":  map[string]interface{}{"type": "str", "value": "go"},
        "secondPayload": map[string]interface{}{"type": "bool", "value": "false"},
        "delayMs":       float64(500),
    }))
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeString, Value: "go"}, n.FirstPayload)
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"}, n.SecondPayload)
    assert.Equal(t, int64(500), n.DelayMs)

    cfg := n.GetConfig()
    assert.Equal(t, int64(500), cfg["delayMs"])
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"}, n.FirstPayload)
    assert.Equal(t, typedvalue.Value{}, n.SecondPayload)
    assert.Equal(t, int64(250), n.DelayMs)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{DelayMs: -1}).Validate())
    assert.NoError(t, (&Node{DelayMs: 0}).Validate())
}

func TestNode_Execute_FirstPayload(t *testing.T) {
    t.Run("default passthrough sends the original payload unchanged", func(t *testing.T) {
        n := &Node{FirstPayload: typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"}}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "hello", "topic": "t"})
        require.NoError(t, err)
        assert.Equal(t, "hello", output["payload"])
        assert.Equal(t, "t", output["topic"])
    })

    t.Run("a configured literal overrides the payload", func(t *testing.T) {
        n := &Node{FirstPayload: typedvalue.Value{Type: typedvalue.TypeString, Value: "fixed"}}
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "original"})
        require.NoError(t, err)
        assert.Equal(t, "fixed", output["payload"])
    })
}

func TestNode_Execute_SecondPayload(t *testing.T) {
    t.Run("delivers the second payload to this node's own output after the delay", func(t *testing.T) {
        var mu sync.Mutex
        var deliveredTo string
        var deliveredPayload map[string]interface{}
        done := make(chan struct{})

        rt := registry.NewNodeRuntime("f1", "trigger-1", "trigger", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
            mu.Lock()
            deliveredTo = nodeID
            deliveredPayload = payload
            mu.Unlock()
            close(done)
        }, nil)
        ctx := registry.WithRuntime(context.Background(), rt)

        n := &Node{
            FirstPayload:  typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"},
            SecondPayload: typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"},
            DelayMs:       10,
        }

        output, err := n.Execute(ctx, map[string]interface{}{"payload": "hello"})
        require.NoError(t, err)
        assert.Equal(t, "hello", output["payload"], "the immediate send is unaffected by the scheduled second send")

        select {
        case <-done:
        case <-time.After(time.Second):
            t.Fatal("second payload was never delivered")
        }

        mu.Lock()
        defer mu.Unlock()
        assert.Equal(t, "trigger-1", deliveredTo, "delivered to this node's own ID, so it goes out its normal wires")
        assert.Equal(t, false, deliveredPayload["payload"])
    })

    t.Run("no second send is scheduled when SecondPayload is unconfigured", func(t *testing.T) {
        called := false
        rt := registry.NewNodeRuntime("f1", "trigger-1", "trigger", nil, nil, nil, func(nodeID string, payload map[string]interface{}) { called = true }, nil)
        ctx := registry.WithRuntime(context.Background(), rt)

        n := &Node{FirstPayload: typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"}, DelayMs: 5}
        _, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
        require.NoError(t, err)

        time.Sleep(30 * time.Millisecond)
        assert.False(t, called)
    })

    t.Run("no NodeRuntime available: first payload still sends, second is silently skipped", func(t *testing.T) {
        n := &Node{
            FirstPayload:  typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"},
            SecondPayload: typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"},
            DelayMs:       5,
        }
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x"})
        require.NoError(t, err)
        assert.Equal(t, "x", output["payload"])
    })
}

func TestNode_Close_StopsPendingTimers(t *testing.T) {
    var called bool
    var mu sync.Mutex
    rt := registry.NewNodeRuntime("f1", "trigger-1", "trigger", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
        mu.Lock()
        called = true
        mu.Unlock()
    }, nil)
    ctx := registry.WithRuntime(context.Background(), rt)

    n := &Node{
        FirstPayload:  typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"},
        SecondPayload: typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"},
        DelayMs:       50,
    }

    _, err := n.Execute(ctx, map[string]interface{}{})
    require.NoError(t, err)

    require.NoError(t, n.Close())

    time.Sleep(100 * time.Millisecond) // long enough for the (cancelled) timer to have fired
    mu.Lock()
    defer mu.Unlock()
    assert.False(t, called, "Close should have stopped the pending timer before it fired")
}

func TestNode_Close_PreventsSchedulingNewTimers(t *testing.T) {
    called := false
    rt := registry.NewNodeRuntime("f1", "trigger-1", "trigger", nil, nil, nil, func(nodeID string, payload map[string]interface{}) { called = true }, nil)
    ctx := registry.WithRuntime(context.Background(), rt)

    n := &Node{FirstPayload: typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"}, SecondPayload: typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"}, DelayMs: 5}
    require.NoError(t, n.Close())

    _, err := n.Execute(ctx, map[string]interface{}{})
    require.NoError(t, err)

    time.Sleep(30 * time.Millisecond)
    assert.False(t, called)
}

func TestNode_Execute_ConcurrentSchedulingIsSafe(t *testing.T) {
    rt := registry.NewNodeRuntime("f1", "trigger-1", "trigger", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {}, nil)
    ctx := registry.WithRuntime(context.Background(), rt)
    n := &Node{FirstPayload: typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"}, SecondPayload: typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"}, DelayMs: 20}

    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, _ = n.Execute(ctx, map[string]interface{}{})
        }()
    }
    wg.Wait()
    require.NoError(t, n.Close())
}
