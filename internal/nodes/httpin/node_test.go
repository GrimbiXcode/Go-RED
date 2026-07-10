package httpin

import (
    "context"
    "net/http"
    "net/http/httptest"
    "os"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRouter_ExactMatch(t *testing.T) {
    r := &router{}
    called := false
    route := r.add("GET", "/api/hello", func(w http.ResponseWriter, req *http.Request, params map[string]string) {
        called = true
        assert.Empty(t, params)
        w.WriteHeader(http.StatusOK)
    })
    defer r.remove(route)

    req := httptest.NewRequest("GET", "/api/hello", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    assert.True(t, called)
    assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRouter_PathParam(t *testing.T) {
    r := &router{}
    var gotParams map[string]string
    route := r.add("GET", "/api/users/:id", func(w http.ResponseWriter, req *http.Request, params map[string]string) {
        gotParams = params
        w.WriteHeader(http.StatusOK)
    })
    defer r.remove(route)

    req := httptest.NewRequest("GET", "/api/users/42", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    assert.Equal(t, map[string]string{"id": "42"}, gotParams)
}

func TestRouter_MethodMismatch_404(t *testing.T) {
    r := &router{}
    route := r.add("POST", "/api/hello", func(w http.ResponseWriter, req *http.Request, params map[string]string) {
        t.Fatal("handler should not be called for a GET request")
    })
    defer r.remove(route)

    req := httptest.NewRequest("GET", "/api/hello", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRouter_Remove(t *testing.T) {
    r := &router{}
    route := r.add("GET", "/api/hello", func(w http.ResponseWriter, req *http.Request, params map[string]string) {})
    r.remove(route)

    req := httptest.NewRequest("GET", "/api/hello", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.Error(t, (&Node{Path: "no-leading-slash"}).Validate())
    assert.NoError(t, (&Node{Path: "/ok"}).Validate())
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"method": "post", "path": "/api/x", "responseTimeoutMs": float64(5000)}))
    assert.Equal(t, "POST", n.Method)
    assert.Equal(t, "/api/x", n.Path)
    assert.Equal(t, int64(5000), n.ResponseTimeoutMs)
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

func TestMain(m *testing.M) {
    os.Setenv(portEnvVar, "0")
    os.Exit(m.Run())
}

// testAddr returns SharedAddr() with a 0.0.0.0 bind address rewritten to
// 127.0.0.1, since 0.0.0.0 is a valid bind address but not always a valid
// client-connect destination.
func testAddr() string {
    return strings.Replace(SharedAddr(), "0.0.0.0", "127.0.0.1", 1)
}

func TestNode_Start_EndToEnd(t *testing.T) {
    n := &Node{Method: "POST", Path: "/api/echo/:name", ResponseTimeoutMs: 2000}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var received map[string]interface{}
    done := make(chan struct{})
    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) {
            received = payload
            close(done)
        })
    }()

    require.Eventually(t, func() bool { return SharedAddr() != "" }, 2*time.Second, 5*time.Millisecond)
    // Route registration happens synchronously right after Start begins, on
    // its own goroutine - give it a moment to run before the client dials.
    time.Sleep(50 * time.Millisecond)

    // The POST blocks until this node's own handler completes the response
    // (via handle.Write below), so it must run concurrently with the code
    // that reads "done" and calls Write - not sequentially before it.
    type postResult struct {
        resp *http.Response
        err  error
    }
    resultCh := make(chan postResult, 1)
    go func() {
        resp, err := http.Post("http://"+testAddr()+"/api/echo/world", "application/json", strings.NewReader(`{"a":1}`))
        resultCh <- postResult{resp, err}
    }()

    select {
    case <-done:
    case <-time.After(2 * time.Second):
        t.Fatal("emit was never called")
    }
    require.NotNil(t, received)
    payload, ok := received["payload"].(map[string]interface{})
    require.True(t, ok, "JSON content-type body should decode to a map")
    assert.Equal(t, float64(1), payload["a"])
    req := received["req"].(map[string]interface{})
    params := req["params"].(map[string]interface{})
    assert.Equal(t, "world", params["name"])

    handle := received[KeyResponseHandle].(*ResponseHandle)
    handle.Write(http.StatusCreated, map[string]string{"X-Test": "yes"}, []byte("created"))

    select {
    case result := <-resultCh:
        require.NoError(t, result.err)
        require.NoError(t, result.resp.Body.Close())
        assert.Equal(t, http.StatusCreated, result.resp.StatusCode)
    case <-time.After(2 * time.Second):
        t.Fatal("POST never completed after handle.Write")
    }
}

func TestNode_Start_TimesOutWithoutResponse(t *testing.T) {
    n := &Node{Method: "GET", Path: "/api/hang", ResponseTimeoutMs: 50}
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        _ = n.Start(ctx, func(payload map[string]interface{}) {
            // deliberately never completes the response
        })
    }()

    require.Eventually(t, func() bool { return SharedAddr() != "" }, 2*time.Second, 5*time.Millisecond)

    var resp *http.Response
    require.Eventually(t, func() bool {
        var err error
        resp, err = http.Get("http://" + testAddr() + "/api/hang")
        return err == nil
    }, 2*time.Second, 10*time.Millisecond)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
}
