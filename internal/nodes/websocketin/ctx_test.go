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

// The OnMessage handler outlives Start (it stays registered on the shared
// config node), so once the flow context is cancelled Start must return
// promptly and a message arriving afterwards must not be emitted.
func TestNode_Start_StopsEmittingAfterCancel(t *testing.T) {
	fs := &fakeServer{}
	rt := registry.NewNodeRuntime("f1", "wsin-1", "websocket in", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
		if nodeID == "s1" {
			return fs, true
		}
		return nil, false
	})
	ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
	defer cancel()

	var mu sync.Mutex
	emitted := 0
	n := &Node{Server: "s1"}
	done := make(chan error, 1)
	go func() {
		done <- n.Start(ctx, func(map[string]interface{}) {
			mu.Lock()
			emitted++
			mu.Unlock()
		})
	}()

	require.Eventually(t, func() bool {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		return len(fs.handlers) > 0
	}, time.Second, 5*time.Millisecond)

	fs.fire([]byte("before"), true)
	mu.Lock()
	assert.Equal(t, 1, emitted)
	mu.Unlock()

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of cancellation")
	}

	fs.fire([]byte("after"), true)
	mu.Lock()
	assert.Equal(t, 1, emitted, "nothing should be emitted after the flow context ended")
	mu.Unlock()
}
