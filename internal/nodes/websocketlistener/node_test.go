package websocketlistener

import (
    "os"
    "strings"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
    "github.com/gorilla/websocket"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
    os.Setenv("GORED_HTTP_NODE_PORT", "0")
    os.Exit(m.Run())
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Path: "/ws"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"path": "/ws/test1"}))
    assert.Equal(t, "/ws/test1", n.Path)
    require.NoError(t, n.Close())
}

func dialTestClient(t *testing.T, path string) *websocket.Conn {
    t.Helper()
    require.Eventually(t, func() bool { return httpin.SharedAddr() != "" }, 2*time.Second, 5*time.Millisecond)
    addr := strings.Replace(httpin.SharedAddr(), "0.0.0.0", "127.0.0.1", 1)
    conn, _, err := websocket.DefaultDialer.Dial("ws://"+addr+path, nil)
    require.NoError(t, err)
    return conn
}

func TestNode_UpgradeAndEcho(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"path": "/ws/echo1"}))
    defer n.Close()

    var mu sync.Mutex
    var received [][]byte
    got := make(chan struct{}, 1)
    n.OnMessage(func(data []byte, isText bool) {
        mu.Lock()
        received = append(received, data)
        mu.Unlock()
        got <- struct{}{}
    })

    conn := dialTestClient(t, "/ws/echo1")
    defer conn.Close()

    require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("hello")))

    select {
    case <-got:
    case <-time.After(2 * time.Second):
        t.Fatal("server never received the message")
    }
    mu.Lock()
    assert.Equal(t, [][]byte{[]byte("hello")}, received)
    mu.Unlock()
}

func TestNode_Send_BroadcastsToClient(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"path": "/ws/broadcast1"}))
    defer n.Close()

    conn := dialTestClient(t, "/ws/broadcast1")
    defer conn.Close()

    // Give the server a moment to register the connection before sending.
    time.Sleep(50 * time.Millisecond)
    require.NoError(t, n.Send([]byte("from-server"), true))

    conn.SetReadDeadline(time.Now().Add(2 * time.Second))
    _, data, err := conn.ReadMessage()
    require.NoError(t, err)
    assert.Equal(t, "from-server", string(data))
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}
