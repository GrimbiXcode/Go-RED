package udpout

import (
    "net"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.Error(t, (&Node{Host: "localhost"}).Validate())
    assert.NoError(t, (&Node{Host: "localhost", Port: 1234}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"host": "example.com", "port": float64(1234)}))
    assert.Equal(t, "example.com", n.Host)
    assert.Equal(t, 1234, n.Port)
}

func TestNode_Execute_SendsDatagram(t *testing.T) {
    ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
    require.NoError(t, err)
    defer ln.Close()
    port := ln.LocalAddr().(*net.UDPAddr).Port

    received := make(chan []byte, 1)
    go func() {
        buf := make([]byte, 1024)
        nRead, _, err := ln.ReadFromUDP(buf)
        if err != nil {
            return
        }
        received <- buf[:nRead]
    }()

    n := &Node{Host: "127.0.0.1", Port: port}
    out, err := n.Execute(nil, map[string]interface{}{"payload": "hello"})
    require.NoError(t, err)
    assert.Equal(t, "hello", out["payload"])

    select {
    case data := <-received:
        assert.Equal(t, "hello", string(data))
    case <-time.After(2 * time.Second):
        t.Fatal("server never received the datagram")
    }
}

func TestNode_Execute_JSONPayload(t *testing.T) {
    ln, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
    require.NoError(t, err)
    defer ln.Close()
    port := ln.LocalAddr().(*net.UDPAddr).Port

    received := make(chan []byte, 1)
    go func() {
        buf := make([]byte, 1024)
        nRead, _, err := ln.ReadFromUDP(buf)
        if err != nil {
            return
        }
        received <- buf[:nRead]
    }()

    n := &Node{Host: "127.0.0.1", Port: port}
    _, err = n.Execute(nil, map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)

    select {
    case data := <-received:
        assert.JSONEq(t, `{"a":1}`, string(data))
    case <-time.After(2 * time.Second):
        t.Fatal("server never received the datagram")
    }
}
