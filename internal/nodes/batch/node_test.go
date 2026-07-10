package batch

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
    require.NoError(t, n.SetConfig(map[string]interface{}{"mode": "interval", "count": float64(5), "intervalMs": float64(2000)}))
    assert.Equal(t, "interval", n.Mode)
    assert.Equal(t, 5, n.Count)
    assert.Equal(t, int64(2000), n.IntervalMs)
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "count", n.Mode)
    assert.Equal(t, 10, n.Count)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{Mode: "bogus"}).Validate())
    assert.Error(t, (&Node{Count: -1}).Validate())
    assert.Error(t, (&Node{IntervalMs: -1}).Validate())
    assert.NoError(t, (&Node{Mode: "count", Count: 1}).Validate())
}

func TestNode_ExecuteMulti_CountMode(t *testing.T) {
    n := &Node{Mode: "count", Count: 3}

    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
    require.NoError(t, err)
    assert.Empty(t, outputs)

    outputs, err = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "b"})
    require.NoError(t, err)
    assert.Empty(t, outputs)

    outputs, err = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "c"})
    require.NoError(t, err)
    require.Contains(t, outputs, "output")
    assert.Equal(t, []interface{}{"a", "b", "c"}, outputs["output"]["payload"])
}

func TestNode_ExecuteMulti_CountMode_BufferResetsAfterFlush(t *testing.T) {
    n := &Node{Mode: "count", Count: 1}

    first, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"a"}, first["output"]["payload"])

    second, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "b"})
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"b"}, second["output"]["payload"], "the new batch should not include messages from the previous one")
}

func TestNode_ExecuteMulti_IntervalMode_NeverSendsImmediately(t *testing.T) {
    n := &Node{Mode: "interval", IntervalMs: 1000}
    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
    require.NoError(t, err)
    assert.Empty(t, outputs)

    n.mu.Lock()
    assert.Len(t, n.buffer, 1)
    n.mu.Unlock()
}

func TestNode_Start_IntervalMode_FlushesOnSchedule(t *testing.T) {
    n := &Node{Mode: "interval", IntervalMs: 10}
    _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
    _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "b"})

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var mu sync.Mutex
    var flushes []map[string]interface{}
    done := make(chan error, 1)
    go func() {
        done <- n.Start(ctx, func(payload map[string]interface{}) {
            mu.Lock()
            flushes = append(flushes, payload)
            mu.Unlock()
        })
    }()

    require.Eventually(t, func() bool {
        mu.Lock()
        defer mu.Unlock()
        return len(flushes) >= 1
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
    assert.Equal(t, []interface{}{"a", "b"}, flushes[0]["payload"])
}

func TestNode_Start_IntervalMode_SkipsEmptyBuffer(t *testing.T) {
    n := &Node{Mode: "interval", IntervalMs: 10}
    ctx, cancel := context.WithCancel(context.Background())

    called := false
    done := make(chan error, 1)
    go func() { done <- n.Start(ctx, func(map[string]interface{}) { called = true }) }()

    time.Sleep(30 * time.Millisecond)
    cancel()
    <-done
    assert.False(t, called, "an empty buffer should not produce a flush")
}

func TestNode_Start_CountMode_ReturnsOnCancelWithoutFlushing(t *testing.T) {
    n := &Node{Mode: "count", Count: 10}
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
    n := &Node{Mode: "count", Count: 1}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a"})
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"a"}, output["payload"])
}

func TestNode_ExecuteMulti_ConcurrentAccessIsSafe(t *testing.T) {
    n := &Node{Mode: "count", Count: 1000000} // large enough to never flush during the test
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": i})
        }(i)
    }
    wg.Wait()

    n.mu.Lock()
    assert.Len(t, n.buffer, 50)
    n.mu.Unlock()
}
