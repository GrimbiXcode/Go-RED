package delay

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When the flow context ends, Start must return within 1s and drop the
// messages still queued for release, so an undeployed flow holds nothing.
func TestNode_Start_RateMode_DropsQueueOnCancel(t *testing.T) {
	n := &Node{Mode: "rate", RateLimit: 1, RateIntervalMs: 60000} // effectively never releases
	_, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
	_, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "b"})

	ctx, cancel := context.WithCancel(context.Background())
	released := make(chan struct{}, 2)
	done := make(chan error, 1)
	go func() {
		done <- n.Start(ctx, func(map[string]interface{}) { released <- struct{}{} })
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of cancellation")
	}

	n.mu.Lock()
	assert.Empty(t, n.queue, "the pending queue should be released when the flow context ends")
	n.mu.Unlock()
	assert.Empty(t, released, "nothing should have been released")

	// Close is also safe on an already-drained node.
	require.NoError(t, n.Close())
}
