package websocketclient

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"
    "time"

    "github.com/gorilla/websocket"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

var testUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// echoServer accepts one connection and echoes every message it receives,
// also recording each one for assertions.
func echoServer(t *testing.T) (*httptest.Server, *sync.Mutex, *[][]byte) {
    t.Helper()
    var mu sync.Mutex
    var received [][]byte
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        conn, err := testUpgrader.Upgrade(w, r, nil)
        require.NoError(t, err)
        defer conn.Close()
        for {
            msgType, data, err := conn.ReadMessage()
            if err != nil {
                return
            }
            mu.Lock()
            received = append(received, data)
            mu.Unlock()
            _ = conn.WriteMessage(msgType, data)
        }
    }))
    return srv, &mu, &received
}

func wsURL(httpURL string) string {
    return "ws" + strings.TrimPrefix(httpURL, "http")
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{URL: "ws://localhost/x"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    srv, _, _ := echoServer(t)
    defer srv.Close()

    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"url": wsURL(srv.URL)}))
    assert.Equal(t, wsURL(srv.URL), n.URL)
    require.NoError(t, n.Close())
}

func TestNode_ConnectsAndSendsReceives(t *testing.T) {
    srv, mu, received := echoServer(t)
    defer srv.Close()

    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"url": wsURL(srv.URL)}))
    defer n.Close()

    got := make(chan []byte, 1)
    n.OnMessage(func(data []byte, isText bool) { got <- data })

    require.Eventually(t, func() bool { return n.Send([]byte("hi"), true) == nil }, 2*time.Second, 10*time.Millisecond)

    select {
    case data := <-got:
        assert.Equal(t, "hi", string(data))
    case <-time.After(2 * time.Second):
        t.Fatal("client never received the echoed message")
    }

    mu.Lock()
    assert.Equal(t, [][]byte{[]byte("hi")}, *received)
    mu.Unlock()
}

func TestNode_Send_NotConnected_Errors(t *testing.T) {
    n := &Node{}
    err := n.Send([]byte("x"), true)
    assert.Error(t, err)
}

func TestNode_Close_StopsReconnectLoop(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"url": "ws://127.0.0.1:1/does-not-exist"}))
    require.NoError(t, n.Close())
    // A second Close should be a safe no-op.
    require.NoError(t, n.Close())
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}
