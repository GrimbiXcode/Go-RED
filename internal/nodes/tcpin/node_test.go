package tcpin

import (
    "context"
    "net"
    "strconv"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func freePort(t *testing.T) int {
    t.Helper()
    ln, err := net.Listen("tcp", ":0")
    require.NoError(t, err)
    port := ln.Addr().(*net.TCPAddr).Port
    require.NoError(t, ln.Close())
    return port
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Port: 1234, Server: true}).Validate())
    assert.Error(t, (&Node{Port: 1234, Server: false}).Validate(), "client mode requires host")
    assert.NoError(t, (&Node{Port: 1234, Server: false, Host: "localhost"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "host": "example.com", "port": float64(1234), "server": false, "splitLines": true, "datatype": "utf8",
    }))
    assert.Equal(t, "example.com", n.Host)
    assert.Equal(t, 1234, n.Port)
    assert.False(t, n.Server)
    assert.True(t, n.SplitLines)
    assert.Equal(t, "utf8", n.Datatype)
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Start_ServerMode_RawChunks(t *testing.T) {
    port := freePort(t)
    n := &Node{Port: port, Server: true, Datatype: "utf8"}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    received := make(chan map[string]interface{}, 1)
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) { received <- payload })
    }()

    var conn net.Conn
    require.Eventually(t, func() bool {
        var err error
        conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
        return err == nil
    }, 2*time.Second, 10*time.Millisecond)
    defer conn.Close()

    _, err := conn.Write([]byte("hello"))
    require.NoError(t, err)

    select {
    case msg := <-received:
        assert.Equal(t, "hello", msg["payload"])
        assert.NotEmpty(t, msg["ip"])
    case <-time.After(2 * time.Second):
        t.Fatal("server never received the data")
    }
}

func TestNode_Start_ServerMode_SplitLines(t *testing.T) {
    port := freePort(t)
    n := &Node{Port: port, Server: true, SplitLines: true, Datatype: "utf8"}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    received := make(chan map[string]interface{}, 3)
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) { received <- payload })
    }()

    var conn net.Conn
    require.Eventually(t, func() bool {
        var err error
        conn, err = net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
        return err == nil
    }, 2*time.Second, 10*time.Millisecond)
    defer conn.Close()

    _, err := conn.Write([]byte("line1\nline2\n"))
    require.NoError(t, err)

    var got []string
    for i := 0; i < 2; i++ {
        select {
        case msg := <-received:
            got = append(got, msg["payload"].(string))
        case <-time.After(2 * time.Second):
            t.Fatalf("only received %d of 2 lines", i)
        }
    }
    assert.Equal(t, []string{"line1", "line2"}, got)
}

func TestNode_Start_ClientMode(t *testing.T) {
    port := freePort(t)
    ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
    require.NoError(t, err)
    defer ln.Close()

    go func() {
        conn, err := ln.Accept()
        if err != nil {
            return
        }
        defer conn.Close()
        _, _ = conn.Write([]byte("greetings"))
    }()

    n := &Node{Host: "127.0.0.1", Port: port, Server: false, Datatype: "utf8"}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    received := make(chan map[string]interface{}, 1)
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) { received <- payload })
    }()

    select {
    case msg := <-received:
        assert.Equal(t, "greetings", msg["payload"])
    case <-time.After(2 * time.Second):
        t.Fatal("client never received the data")
    }
}

