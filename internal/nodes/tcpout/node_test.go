package tcpout

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

func TestNode_Execute_WritesPayload(t *testing.T) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    defer ln.Close()
    port := ln.Addr().(*net.TCPAddr).Port

    received := make(chan []byte, 1)
    go func() {
        conn, err := ln.Accept()
        if err != nil {
            return
        }
        defer conn.Close()
        buf := make([]byte, 1024)
        n, _ := conn.Read(buf)
        received <- buf[:n]
    }()

    n := &Node{Host: "127.0.0.1", Port: port}
    out, err := n.Execute(nil, map[string]interface{}{"payload": "hello"})
    require.NoError(t, err)
    assert.Equal(t, "hello", out["payload"])

    select {
    case data := <-received:
        assert.Equal(t, "hello", string(data))
    case <-time.After(2 * time.Second):
        t.Fatal("server never received the data")
    }
}

func TestNode_Execute_ConnectionRefused_Errors(t *testing.T) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    port := ln.Addr().(*net.TCPAddr).Port
    require.NoError(t, ln.Close())

    n := &Node{Host: "127.0.0.1", Port: port}
    _, err = n.Execute(nil, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}
