package httpproxy

import (
    "net/http"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "url":      "http://proxy.example.com:8080",
        "username": "u",
        "password": "p",
        "noProxy":  []interface{}{"internal.example.com", "localhost"},
    }))
    assert.Equal(t, "http://proxy.example.com:8080", n.URL)
    assert.Equal(t, "u", n.Username)
    assert.Equal(t, "p", n.Password)
    assert.Equal(t, []string{"internal.example.com", "localhost"}, n.NoProxy)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{}).Validate())
    assert.NoError(t, (&Node{URL: "http://proxy.example.com:8080"}).Validate())
    assert.Error(t, (&Node{URL: "http://[::1"}).Validate())
}

func TestNode_ProxyFunc_ReturnsConfiguredProxy(t *testing.T) {
    n := &Node{URL: "http://proxy.example.com:8080", Username: "u", Password: "p"}
    req, err := http.NewRequest("GET", "http://target.example.com/path", nil)
    require.NoError(t, err)

    proxyURL, err := n.ProxyFunc()(req)
    require.NoError(t, err)
    require.NotNil(t, proxyURL)
    assert.Equal(t, "proxy.example.com:8080", proxyURL.Host)
    assert.Equal(t, "u", proxyURL.User.Username())
    pass, _ := proxyURL.User.Password()
    assert.Equal(t, "p", pass)
}

func TestNode_ProxyFunc_BypassesNoProxyHosts(t *testing.T) {
    n := &Node{URL: "http://proxy.example.com:8080", NoProxy: []string{"internal.example.com"}}
    req, err := http.NewRequest("GET", "http://internal.example.com/path", nil)
    require.NoError(t, err)

    proxyURL, err := n.ProxyFunc()(req)
    require.NoError(t, err)
    assert.Nil(t, proxyURL)
}

func TestNode_ProxyFunc_NoURLConfigured(t *testing.T) {
    n := &Node{}
    req, err := http.NewRequest("GET", "http://target.example.com/path", nil)
    require.NoError(t, err)

    proxyURL, err := n.ProxyFunc()(req)
    require.NoError(t, err)
    assert.Nil(t, proxyURL)
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}
