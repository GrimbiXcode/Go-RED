package httpresponse

import (
    "context"
    "io"
    "net/http"
    "os"
    "strings"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
    os.Setenv("GORED_HTTP_NODE_PORT", "0")
    os.Exit(m.Run())
}

// requestAndAwaitMessage deploys a real httpin.Node, fires a request at it
// in the background (it blocks until this test's Node.Execute completes
// the response), and returns the message httpin emitted plus a channel
// that will receive the eventual HTTP response.
func requestAndAwaitMessage(t *testing.T, method, path, reqPath string) (map[string]interface{}, <-chan *http.Response) {
    t.Helper()
    n := &httpin.Node{Method: method, Path: path, ResponseTimeoutMs: 5000}
    ctx, cancel := context.WithCancel(context.Background())
    t.Cleanup(cancel)

    received := make(chan map[string]interface{}, 1)
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) {
            received <- payload
        })
    }()

    require.Eventually(t, func() bool { return httpin.SharedAddr() != "" }, 2*time.Second, 5*time.Millisecond)
    time.Sleep(50 * time.Millisecond)

    addr := strings.Replace(httpin.SharedAddr(), "0.0.0.0", "127.0.0.1", 1)
    respCh := make(chan *http.Response, 1)
    go func() {
        resp, err := http.Get("http://" + addr + reqPath)
        if err == nil {
            respCh <- resp
        }
    }()

    select {
    case msg := <-received:
        return msg, respCh
    case <-time.After(2 * time.Second):
        t.Fatal("http in never emitted a message")
        return nil, nil
    }
}

func TestNode_Execute_WritesJSONResponse(t *testing.T) {
    msg, respCh := requestAndAwaitMessage(t, "GET", "/rt/json", "/rt/json")
    msg["payload"] = map[string]interface{}{"ok": true}

    n := &Node{}
    out, err := n.Execute(nil, msg)
    require.NoError(t, err)
    assert.Equal(t, msg, out)

    select {
    case resp := <-respCh:
        defer resp.Body.Close()
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
        body, _ := io.ReadAll(resp.Body)
        assert.JSONEq(t, `{"ok":true}`, string(body))
    case <-time.After(2 * time.Second):
        t.Fatal("response never completed")
    }
}

func TestNode_Execute_StringPayload_PlainText(t *testing.T) {
    msg, respCh := requestAndAwaitMessage(t, "GET", "/rt/text", "/rt/text")
    msg["payload"] = "hello"

    n := &Node{}
    _, err := n.Execute(nil, msg)
    require.NoError(t, err)

    resp := <-respCh
    defer resp.Body.Close()
    assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
    body, _ := io.ReadAll(resp.Body)
    assert.Equal(t, "hello", string(body))
}

func TestNode_Execute_FixedStatusCode(t *testing.T) {
    msg, respCh := requestAndAwaitMessage(t, "GET", "/rt/status", "/rt/status")
    msg["payload"] = "created"

    n := &Node{StatusCode: 201}
    _, err := n.Execute(nil, msg)
    require.NoError(t, err)

    resp := <-respCh
    defer resp.Body.Close()
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestNode_Execute_MsgStatusCode(t *testing.T) {
    msg, respCh := requestAndAwaitMessage(t, "GET", "/rt/msgstatus", "/rt/msgstatus")
    msg["payload"] = "teapot"
    msg["statusCode"] = float64(418)

    n := &Node{}
    _, err := n.Execute(nil, msg)
    require.NoError(t, err)

    resp := <-respCh
    defer resp.Body.Close()
    assert.Equal(t, 418, resp.StatusCode)
}

func TestNode_Execute_CustomHeaders(t *testing.T) {
    msg, respCh := requestAndAwaitMessage(t, "GET", "/rt/headers", "/rt/headers")
    msg["payload"] = "x"
    msg["headers"] = map[string]interface{}{"X-Custom": "value"}

    n := &Node{}
    _, err := n.Execute(nil, msg)
    require.NoError(t, err)

    resp := <-respCh
    defer resp.Body.Close()
    assert.Equal(t, "value", resp.Header.Get("X-Custom"))
}

func TestNode_Execute_MissingHandle_Errors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(nil, map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"statusCode": float64(204)}))
    assert.Equal(t, 204, n.StatusCode)
}
