package tcprequest

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cancelling the per-message context while Execute is blocked reading the
// reply must close the connection and return promptly with an error that
// wraps context.Canceled - not wait for TimeoutMs (5s here).
func TestNode_Execute_CancelDuringRead_ReturnsPromptly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	hold := make(chan struct{})
	defer close(hold)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Drain the request (ReadAll returns at the node's half-close),
		// then keep our side open without ever replying: the node's read
		// can only end through its own context.
		_, _ = io.ReadAll(conn)
		<-hold
	}()

	addr := ln.Addr().(*net.TCPAddr)
	n := &Node{Host: addr.IP.String(), Port: addr.Port, TimeoutMs: 5000}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = n.Execute(ctx, map[string]interface{}{"payload": "hello"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Less(t, elapsed, time.Second)
}
