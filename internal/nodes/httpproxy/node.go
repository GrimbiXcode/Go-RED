// Package httpproxy provides the "http proxy" config node - a reusable
// outbound HTTP/HTTPS proxy configuration referenced by ID from other
// nodes (http request), rather than being wired into a flow itself (see
// docs/NODE_PALETTE_PLAN.md, Phase 6's config-node concept).
package httpproxy

import (
    "fmt"
    "net/http"
    "net/url"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Provider is implemented by http-proxy nodes (this package's Node),
// exposing a net/http.Transport-compatible Proxy function to a consuming
// node.
type Provider interface {
    // ProxyFunc returns a function suitable for http.Transport.Proxy: the
    // configured proxy URL for any request whose host isn't covered by
    // NoProxy, nil otherwise (meaning: connect directly).
    ProxyFunc() func(*http.Request) (*url.URL, error)
}

// Node holds an HTTP-proxy node's configuration.
type Node struct {
    // URL is the proxy's own address, e.g. "http://proxy.example.com:8080".
    URL string
    // Username/Password authenticate to the proxy itself (HTTP Basic),
    // embedded into URL's userinfo when building the proxy URL.
    Username string
    Password string
    // NoProxy lists hostnames (exact match) that bypass the proxy and
    // connect directly.
    NoProxy []string
}

// ProxyFunc returns a Transport-compatible proxy function reflecting
// URL/Username/Password/NoProxy.
func (n *Node) ProxyFunc() func(*http.Request) (*url.URL, error) {
    return func(req *http.Request) (*url.URL, error) {
        if n.URL == "" {
            return nil, nil
        }
        host := req.URL.Hostname()
        for _, skip := range n.NoProxy {
            if strings.EqualFold(strings.TrimSpace(skip), host) {
                return nil, nil
            }
        }
        proxyURL, err := url.Parse(n.URL)
        if err != nil {
            return nil, fmt.Errorf("http proxy: invalid url %q: %w", n.URL, err)
        }
        if n.Username != "" {
            proxyURL.User = url.UserPassword(n.Username, n.Password)
        }
        return proxyURL, nil
    }
}

// Execute exists only to satisfy registry.NodeExecutor - a config node has
// no message inputs/outputs of its own and is never wired into a flow's
// message path.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("http proxy: config nodes have no message input")
}

func (n *Node) Validate() error {
    if n.URL == "" {
        return fmt.Errorf("http proxy: url is required")
    }
    if _, err := url.Parse(n.URL); err != nil {
        return fmt.Errorf("http proxy: invalid url %q: %w", n.URL, err)
    }
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    noProxy := make([]interface{}, len(n.NoProxy))
    for i, v := range n.NoProxy {
        noProxy[i] = v
    }
    return map[string]interface{}{
        "url":      n.URL,
        "username": n.Username,
        "password": n.Password,
        "noProxy":  noProxy,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["url"].(string); ok {
        n.URL = v
    }
    if v, ok := config["username"].(string); ok {
        n.Username = v
    }
    if v, ok := config["password"].(string); ok {
        n.Password = v
    }
    n.NoProxy = nil
    if raw, ok := config["noProxy"].([]interface{}); ok {
        for _, v := range raw {
            if s, ok := v.(string); ok {
                n.NoProxy = append(n.NoProxy, s)
            }
        }
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("http proxy", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "http proxy",
        Type:        "http proxy",
        Name:        "HTTP Proxy",
        Description: "Reusable outbound HTTP/HTTPS proxy configuration, referenced by ID from http request",
        Category:    "config",
        Inputs:      []registry.Port{},
        Outputs:     []registry.Port{},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "url":      {Type: "string", Description: "Proxy address, e.g. http://proxy.example.com:8080", Default: ""},
                "username": {Type: "string", Description: "Proxy Basic Auth username", Default: ""},
                "password": {Type: "string", Description: "Proxy Basic Auth password", Default: ""},
                "noProxy":  {Type: "array", Description: "Hostnames that bypass the proxy and connect directly", Default: []interface{}{}},
            },
            Required: []string{"url"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v12H4zM2 18h20v2H2zM9 8h6v4H9z"/></svg>`,
        Tags: []string{"config", "http", "proxy"},
    })
    if err != nil {
        panic(err)
    }
}
