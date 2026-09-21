package udpout

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An already-cancelled per-message context must fail before any datagram
// is sent, with an error that wraps context.Canceled.
func TestNode_Execute_CancelledContext_Errors(t *testing.T) {
	ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	defer ln.Close()
	port := ln.LocalAddr().(*net.UDPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	n := &Node{Host: "127.0.0.1", Port: port}
	start := time.Now()
	_, err = n.Execute(ctx, map[string]interface{}{"payload": "hello"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Less(t, elapsed, time.Second)

	// Nothing should have arrived.
	require.NoError(t, ln.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
	buf := make([]byte, 64)
	_, _, err = ln.ReadFromUDP(buf)
	assert.Error(t, err, "no datagram should have been sent for a cancelled message")
}
