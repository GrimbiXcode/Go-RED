package udpin

import (
    "context"
    "net"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func freePort(t *testing.T) int {
    t.Helper()
    ln, err := net.ListenUDP("udp", &net.UDPAddr{})
    require.NoError(t, err)
    port := ln.LocalAddr().(*net.UDPAddr).Port
    require.NoError(t, ln.Close())
    return port
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Port: 1234}).Validate())
    assert.Error(t, (&Node{Port: 1234, Datatype: "bogus"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"port": float64(1234), "datatype": "utf8"}))
    assert.Equal(t, 1234, n.Port)
    assert.Equal(t, "utf8", n.Datatype)
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Start_EmitsOnDatagram(t *testing.T) {
    port := freePort(t)
    n := &Node{Port: port, Datatype: "utf8"}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    received := make(chan map[string]interface{}, 1)
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) { received <- payload })
    }()

    raddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
    conn, err := net.DialUDP("udp", nil, raddr)
    require.NoError(t, err)
    defer conn.Close()

    // Unlike TCP, a UDP Write succeeds immediately whether or not anyone is
    // listening yet (no handshake) - so a single send racing against
    // n.Start's own goroutine binding its listener can be silently dropped.
    // Keep sending on a ticker until the message arrives or the outer
    // timeout fires, instead of treating one successful Write as proof of
    // delivery.
    stopSending := make(chan struct{})
    defer close(stopSending)
    go func() {
        ticker := time.NewTicker(20 * time.Millisecond)
        defer ticker.Stop()
        for {
            select {
            case <-stopSending:
                return
            case <-ticker.C:
                _, _ = conn.Write([]byte("hello"))
            }
        }
    }()

    select {
    case msg := <-received:
        assert.Equal(t, "hello", msg["payload"])
        assert.NotEmpty(t, msg["ip"])
    case <-time.After(2 * time.Second):
        t.Fatal("udp in never emitted a message")
    }
}
