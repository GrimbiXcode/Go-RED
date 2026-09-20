package tcpout

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An already-cancelled per-message context must fail the dial at once,
// with an error that wraps context.Canceled, instead of connecting and
// writing anyway.
func TestNode_Execute_CancelledContext_ReturnsPromptly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := &Node{Host: "127.0.0.1", Port: port}
	start := time.Now()
	_, err = n.Execute(ctx, map[string]interface{}{"payload": "hello"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Less(t, elapsed, time.Second)
}
