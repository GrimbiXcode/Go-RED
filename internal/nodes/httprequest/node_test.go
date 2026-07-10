package httprequest

import (
    "context"
    "io"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpproxy"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/tlsconfig"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "url":                map[string]interface{}{"type": "str", "value": "https://example.com"},
        "method":             "post",
        "headers":            map[string]interface{}{"X-Test": "1"},
        "timeoutMs":          float64(5000),
        "maxRedirects":       float64(2),
        "tls":                "tls-1",
        "proxy":              "proxy-1",
        "insecureSkipVerify": true,
    }))
    assert.Equal(t, typedvalue.Value{Type: typedvalue.TypeString, Value: "https://example.com"}, n.Url)
    assert.Equal(t, "POST", n.Method)
    assert.Equal(t, "1", n.Headers["X-Test"])
    assert.Equal(t, int64(5000), n.TimeoutMs)
    assert.Equal(t, 2, n.MaxRedirects)
    assert.Equal(t, "tls-1", n.TLS)
    assert.Equal(t, "proxy-1", n.Proxy)
    assert.True(t, n.InsecureSkipVerify)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: "https://example.com"}}).Validate())
}

func TestNode_Execute_GET_JSONResponse(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"hello":"world"}`))
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}}
    out, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, float64(200), out["statusCode"])
    assert.Equal(t, map[string]interface{}{"hello": "world"}, out["payload"])
}

func TestNode_Execute_POST_SendsBody(t *testing.T) {
    var gotBody []byte
    var gotMethod string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotMethod = r.Method
        gotBody, _ = io.ReadAll(r.Body)
        w.WriteHeader(http.StatusCreated)
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}, Method: "POST"}
    out, err := n.Execute(context.Background(), map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)
    assert.Equal(t, "POST", gotMethod)
    assert.JSONEq(t, `{"a":1}`, string(gotBody))
    assert.Equal(t, float64(201), out["statusCode"])
}

func TestNode_Execute_MsgResolvedURL(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeMsg, Value: "url"}}
    out, err := n.Execute(context.Background(), map[string]interface{}{"url": srv.URL})
    require.NoError(t, err)
    assert.Equal(t, float64(200), out["statusCode"])
}

func TestNode_Execute_EmptyURL_Errors(t *testing.T) {
    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: ""}}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Execute_Timeout(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(200 * time.Millisecond)
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}, TimeoutMs: 20}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Execute_ResponseSizeCap(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        buf := make([]byte, maxResponseBytes+1024)
        _, _ = w.Write(buf)
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}}
    out, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    payload, ok := out["payload"].(string)
    require.True(t, ok)
    assert.LessOrEqual(t, len(payload), maxResponseBytes)
}

func TestNode_Execute_RedirectCap(t *testing.T) {
    var target *httptest.Server
    target = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, target.URL, http.StatusFound) // redirects to itself forever
    }))
    defer target.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: target.URL}, MaxRedirects: 2}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "redirect")
}

func TestNode_Execute_WithTLSConfigNode(t *testing.T) {
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    tlsNode := &tlsconfig.Node{VerifyServerCert: false}
    rt := registry.NewNodeRuntime("f1", "req-1", "http request", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        if nodeID == "tls-1" {
            return tlsNode, true
        }
        return nil, false
    })
    ctx := registry.WithRuntime(context.Background(), rt)

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}, TLS: "tls-1"}
    out, err := n.Execute(ctx, map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, float64(200), out["statusCode"])
}

func TestNode_Execute_WithTLSConfigNode_Missing_Errors(t *testing.T) {
    rt := registry.NewNodeRuntime("f1", "req-1", "http request", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        return nil, false
    })
    ctx := registry.WithRuntime(context.Background(), rt)

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: "https://example.com"}, TLS: "missing"}
    _, err := n.Execute(ctx, map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_Execute_WithHTTPProxyNode(t *testing.T) {
    var proxyHit bool
    proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        proxyHit = true
        w.WriteHeader(http.StatusOK)
    }))
    defer proxy.Close()

    proxyNode := &httpproxy.Node{URL: proxy.URL}
    rt := registry.NewNodeRuntime("f1", "req-1", "http request", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
        if nodeID == "proxy-1" {
            return proxyNode, true
        }
        return nil, false
    })
    ctx := registry.WithRuntime(context.Background(), rt)

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: "http://internal.example.invalid/"}, Proxy: "proxy-1"}
    _, err := n.Execute(ctx, map[string]interface{}{})
    require.NoError(t, err)
    assert.True(t, proxyHit, "request should have gone through the configured proxy")
}

func TestNode_Execute_Headers(t *testing.T) {
    var got string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        got = r.Header.Get("X-Custom")
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}, Headers: map[string]string{"X-Custom": "abc"}}
    _, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, "abc", got)
}

func TestNode_Execute_NonJSONResponse_StringPayload(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/plain")
        _, _ = w.Write([]byte("plain text"))
    }))
    defer srv.Close()

    n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}}
    out, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    assert.Equal(t, "plain text", out["payload"])
}
