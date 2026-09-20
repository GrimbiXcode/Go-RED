package websocketclient

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Close must interrupt a dial whose handshake the server never answers
// (otherwise the reconnect loop, and the socket it holds, would linger for
// the full handshake timeout after undeploy) and wait for the loop
// goroutine to exit.
func TestNode_Close_InterruptsPendingHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn // never answer the upgrade request
	}()

	addr := ln.Addr().(*net.TCPAddr)
	n := &Node{}
	require.NoError(t, n.SetConfig(map[string]interface{}{
		"url": "ws://127.0.0.1:" + strconv.Itoa(addr.Port) + "/stall",
	}))

	var serverSide net.Conn
	select {
	case serverSide = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("the client never connected")
	}
	defer serverSide.Close()

	start := time.Now()
	require.NoError(t, n.Close())
	require.Less(t, time.Since(start), time.Second, "Close should not wait for the handshake timeout")

	select {
	case <-n.loopDone:
	case <-time.After(time.Second):
		t.Fatal("the reconnect loop did not exit within 1s of Close")
	}

	// The client's socket was closed as part of aborting the handshake.
	require.NoError(t, serverSide.SetReadDeadline(time.Now().Add(time.Second)))
	buf := make([]byte, 1024)
	for {
		if _, err := serverSide.Read(buf); err != nil {
			break // EOF (or reset): the peer is gone
		}
	}
}
