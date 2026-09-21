package batch

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When the flow context ends, Start must return within 1s and drop the
// messages still buffered for the next flush, so an undeployed flow holds
// nothing.
func TestNode_Start_IntervalMode_DropsBufferOnCancel(t *testing.T) {
	n := &Node{Mode: "interval", IntervalMs: 60000} // effectively never flushes
	_, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "a"})
	_, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "b"})

	ctx, cancel := context.WithCancel(context.Background())
	flushed := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- n.Start(ctx, func(map[string]interface{}) { flushed <- struct{}{} })
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
	assert.Empty(t, n.buffer, "the pending buffer should be released when the flow context ends")
	n.mu.Unlock()
	assert.Empty(t, flushed, "nothing should have been flushed")

	// Close is also safe on an already-drained node.
	require.NoError(t, n.Close())
}
