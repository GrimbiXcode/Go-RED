package websocketlistener

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SendContext is what websocket-out hands the per-message context to: an
// already-cancelled context fails at once (wrapping context.Canceled,
// touching no client), and the context's deadline bounds a write that a
// client which never reads would otherwise block forever.
func TestNode_SendContext_HonorsContext(t *testing.T) {
	n := &Node{}
	require.NoError(t, n.SetConfig(map[string]interface{}{"path": "/ws/ctx1"}))
	defer n.Close()

	conn := dialTestClient(t, "/ws/ctx1")
	defer conn.Close()
	require.Eventually(t, func() bool {
		n.mu.Lock()
		defer n.mu.Unlock()
		return len(n.conns) == 1
	}, 2*time.Second, 5*time.Millisecond)

	t.Run("cancelled context fails immediately", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := n.SendContext(ctx, []byte("x"), true)
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	})

	t.Run("deadline bounds a write the client never drains", func(t *testing.T) {
		// The client never reads, so once the kernel buffers are full a
		// write can only return through the deadline.
		payload := make([]byte, 1<<20)
		deadline := time.Now().Add(15 * time.Second)
		var err error
		for time.Now().Before(deadline) {
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			start := time.Now()
			err = n.SendContext(ctx, payload, false)
			elapsed := time.Since(start)
			cancel()
			require.Less(t, elapsed, 2*time.Second, "a single send must be bounded by its context deadline")
			if err != nil {
				break
			}
		}
		require.Error(t, err, "sending to a client that never reads should eventually fail on the write deadline")
	})
}
