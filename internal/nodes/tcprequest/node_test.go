package tcprequest

import (
    "io"
    "net"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Host: "localhost", Port: 1234}).Validate())
    assert.Error(t, (&Node{Host: "localhost", Port: 1234, Datatype: "bogus"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "host": "example.com", "port": float64(1234), "timeoutMs": float64(5000), "datatype": "utf8",
    }))
    assert.Equal(t, "example.com", n.Host)
    assert.Equal(t, 1234, n.Port)
    assert.Equal(t, int64(5000), n.TimeoutMs)
    assert.Equal(t, "utf8", n.Datatype)
}

func echoOnceServer(t *testing.T) (host string, port int) {
    t.Helper()
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    go func() {
        conn, err := ln.Accept()
        if err != nil {
            return
        }
        defer conn.Close()
        defer ln.Close()
        data, _ := io.ReadAll(conn)
        _, _ = conn.Write(append([]byte("echo:"), data...))
    }()
    addr := ln.Addr().(*net.TCPAddr)
    return addr.IP.String(), addr.Port
}

func TestNode_Execute_SendsAndReceivesReply(t *testing.T) {
    host, port := echoOnceServer(t)

    n := &Node{Host: host, Port: port, Datatype: "utf8", TimeoutMs: 2000}
    out, err := n.Execute(nil, map[string]interface{}{"payload": "hello"})
    require.NoError(t, err)
    assert.Equal(t, "echo:hello", out["payload"])
}

func TestNode_Execute_BufferDatatype(t *testing.T) {
    host, port := echoOnceServer(t)

    n := &Node{Host: host, Port: port, TimeoutMs: 2000}
    out, err := n.Execute(nil, map[string]interface{}{"payload": "hi"})
    require.NoError(t, err)
    payload, ok := out["payload"].([]byte)
    require.True(t, ok)
    assert.Equal(t, "echo:hi", string(payload))
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
