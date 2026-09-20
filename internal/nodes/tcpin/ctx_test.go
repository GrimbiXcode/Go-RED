package tcpin

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In client mode the outgoing dial is bound to the flow context: with a
// context that is already cancelled, Start must return promptly with an
// error wrapping context.Canceled instead of connecting.
func TestNode_Start_ClientMode_CancelledContext_ReturnsPromptly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := &Node{Host: "127.0.0.1", Port: port, Server: false, Datatype: "utf8"}
	done := make(chan error, 1)
	go func() {
		done <- n.Start(ctx, func(map[string]interface{}) { t.Error("nothing should be emitted") })
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s for an already-cancelled context")
	}

	// Sanity: the listener really was reachable, so the early return was
	// due to the context, not a dial failure.
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	require.NoError(t, err)
	conn.Close()
}
