package delay

import (
    "context"
    "sync"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "mode": "rate", "rateLimit": float64(5), "rateIntervalMs": float64(2000), "dropIntermediate": true,
    }))
    assert.Equal(t, "rate", n.Mode)
    assert.Equal(t, 5, n.RateLimit)
    assert.Equal(t, int64(2000), n.RateIntervalMs)
    assert.True(t, n.DropIntermediate)

    cfg := n.GetConfig()
    assert.Equal(t, "rate", cfg["mode"])
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "delay", n.Mode)
    assert.Equal(t, int64(5000), n.DelayMs)
    assert.Equal(t, 1, n.RateLimit)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{Mode: "bogus"}).Validate())
    assert.Error(t, (&Node{DelayMs: -1}).Validate())
    assert.Error(t, (&Node{RateLimit: -1}).Validate())
    assert.NoError(t, (&Node{Mode: "delay", DelayMs: 100}).Validate())
}

func TestNode_ExecuteMulti_DelayMode(t *testing.T) {
    t.Run("sends on output after the configured delay elapses", func(t *testing.T) {
        n := &Node{Mode: "delay", DelayMs: 20}
        start := time.Now()
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "x"})
        elapsed := time.Since(start)

        require.NoError(t, err)
        assert.Contains(t, outputs, "output")
        assert.Equal(t, "x", outputs["output"]["payload"])
        assert.GreaterOrEqual(t, elapsed, 20*time.Millisecond)
    })

    t.Run("errors if the context is cancelled before the delay elapses", func(t *testing.T) {
        n := &Node{Mode: "delay", DelayMs: 5000}
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
        defer cancel()

        _, err := n.ExecuteMulti(ctx, map[string]interface{}{})
        assert.Error(t, err)
    })
}

func TestNode_ExecuteMulti_RateMode_Enqueues(t *testing.T) {
    n := &Node{Mode: "rate", RateLimit: 1, RateIntervalMs: 1000}
    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "x"})
    require.NoError(t, err)
    assert.Empty(t, outputs, "rate mode never sends immediately")

    n.mu.Lock()
    assert.Len(t, n.queue, 1)
    n.mu.Unlock()
}

func TestNode_ExecuteMulti_RateMode_DropIntermediate(t *testing.T) {
    n := &Node{Mode: "rate", DropIntermediate: true, RateLimit: 1, RateIntervalMs: 1000}
    _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "first"})
    require.NoError(t, err)
    _, err = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "second"})
    require.NoError(t, err)

    n.mu.Lock()
    require.Len(t, n.queue, 1)
    assert.Equal(t, "second", n.queue[0]["payload"])
    n.mu.Unlock()
}

func TestNode_ExecuteMulti_RateMode_QueuesAllWithoutDrop(t *testing.T) {
    n := &Node{Mode: "rate", RateLimit: 1, RateIntervalMs: 1000}
    _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "first"})
    _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "second"})

    n.mu.Lock()
    require.Len(t, n.queue, 2)
    n.mu.Unlock()
}

func TestNode_Start_RateMode_ReleasesQueuedMessages(t *testing.T) {
    n := &Node{Mode: "rate", RateLimit: 10, RateIntervalMs: 100} // releases roughly every 10ms
    _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
    _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "b"})

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var mu sync.Mutex
    var received []map[string]interface{}
    done := make(chan error, 1)
    go func() {
        done <- n.Start(ctx, func(payload map[string]interface{}) {
            mu.Lock()
            received = append(received, payload)
            mu.Unlock()
        })
    }()

    require.Eventually(t, func() bool {
        mu.Lock()
        defer mu.Unlock()
        return len(received) == 2
    }, time.Second, 5*time.Millisecond)

    cancel()
    select {
    case err := <-done:
        assert.NoError(t, err)
    case <-time.After(time.Second):
        t.Fatal("Start did not return after context cancellation")
    }

    mu.Lock()
    defer mu.Unlock()
    assert.Equal(t, "a", received[0]["payload"])
    assert.Equal(t, "b", received[1]["payload"])
}

func TestNode_Start_DelayMode_ReturnsOnCancelWithoutReleasingAnything(t *testing.T) {
    n := &Node{Mode: "delay"}
    ctx, cancel := context.WithCancel(context.Background())

    called := false
    done := make(chan error, 1)
    go func() { done <- n.Start(ctx, func(map[string]interface{}) { called = true }) }()

    cancel()
    select {
    case err := <-done:
        assert.NoError(t, err)
    case <-time.After(time.Second):
        t.Fatal("Start did not return after context cancellation")
    }
    assert.False(t, called)
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    n := &Node{Mode: "delay", DelayMs: 5}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "x"})
    require.NoError(t, err)
    assert.Equal(t, "x", output["payload"])
}

func TestNode_ExecuteMulti_ConcurrentEnqueueIsSafe(t *testing.T) {
    n := &Node{Mode: "rate", RateLimit: 1, RateIntervalMs: 1000}
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"i": i})
        }(i)
    }
    wg.Wait()

    n.mu.Lock()
    assert.Len(t, n.queue, 50)
    n.mu.Unlock()
}
