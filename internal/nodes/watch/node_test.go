package watch

import (
    "context"
    "os"
    "path/filepath"
    "sync"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "files":     " /tmp/a.txt , /tmp/b.txt ",
        "recursive": true,
    }))
    assert.Equal(t, []string{"/tmp/a.txt", "/tmp/b.txt"}, n.Files)
    assert.True(t, n.Recursive)

    cfg := n.GetConfig()
    assert.Equal(t, "/tmp/a.txt,/tmp/b.txt", cfg["files"])
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Files: []string{"/tmp"}}).Validate())
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

type collector struct {
    mu   sync.Mutex
    msgs []map[string]interface{}
}

func (c *collector) emit(msg map[string]interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.msgs = append(c.msgs, msg)
}

func (c *collector) count() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return len(c.msgs)
}

func (c *collector) last() map[string]interface{} {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.msgs[len(c.msgs)-1]
}

func TestNode_Start_EmitsOnFileWrite(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "watched.txt")
    require.NoError(t, os.WriteFile(path, []byte("initial"), 0o644))

    n := &Node{Files: []string{dir}}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    c := &collector{}
    done := make(chan error, 1)
    go func() { done <- n.Start(ctx, c.emit) }()

    // Give the watcher a moment to start before triggering an event.
    time.Sleep(50 * time.Millisecond)
    require.NoError(t, os.WriteFile(path, []byte("changed"), 0o644))

    require.Eventually(t, func() bool { return c.count() > 0 }, 2*time.Second, 10*time.Millisecond)
    last := c.last()
    assert.Equal(t, path, last["filename"])
    assert.Equal(t, filepath.Base(path), last["file"])
    assert.Equal(t, "file", last["type"])

    cancel()
    select {
    case err := <-done:
        assert.NoError(t, err)
    case <-time.After(2 * time.Second):
        t.Fatal("Start did not return after context cancellation")
    }
}
