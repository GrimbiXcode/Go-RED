// Package httpresponse provides the "http response" node implementation -
// completes the in-flight HTTP request an upstream "http in" node
// (internal/nodes/httpin) paired with the message via a *httpin.ResponseHandle
// stored under httpin.KeyResponseHandle.
package httpresponse

import (
    "encoding/json"
    "fmt"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds an HTTP-response node's configuration.
type Node struct {
    // StatusCode overrides msg.statusCode; 0 means use msg.statusCode (or
    // 200 if that's unset too).
    StatusCode int
}

// Execute writes statusCode/headers/body to the pending HTTP response
// carried by input, using: StatusCode if set, else msg.statusCode, else
// 200; msg.headers merged in (as string values only) on top of a
// Content-Type this node infers from the payload's Go type unless
// msg.headers already sets one; and msg.payload as the body (JSON-encoded
// unless it's already a string or []byte).
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    raw, ok := input[httpin.KeyResponseHandle]
    if !ok {
        return nil, fmt.Errorf("http response: message has no pending HTTP request (missing %s - is there an http in node upstream?)", httpin.KeyResponseHandle)
    }
    handle, ok := raw.(*httpin.ResponseHandle)
    if !ok {
        return nil, fmt.Errorf("http response: unexpected type for %s", httpin.KeyResponseHandle)
    }

    statusCode := n.StatusCode
    if statusCode == 0 {
        if sc, ok := input["statusCode"].(float64); ok && sc != 0 {
            statusCode = int(sc)
        } else {
            statusCode = 200
        }
    }

    headers := map[string]string{}
    if h, ok := input["headers"].(map[string]interface{}); ok {
        for k, v := range h {
            if s, ok := v.(string); ok {
                headers[k] = s
            }
        }
    }

    body, contentType, err := encodeBody(input["payload"])
    if err != nil {
        return nil, fmt.Errorf("http response: %w", err)
    }
    if _, exists := headers["Content-Type"]; !exists && contentType != "" {
        headers["Content-Type"] = contentType
    }

    handle.Write(statusCode, headers, body)
    return input, nil
}

func encodeBody(payload interface{}) ([]byte, string, error) {
    switch v := payload.(type) {
    case nil:
        return []byte{}, "", nil
    case string:
        return []byte(v), "text/plain; charset=utf-8", nil
    case []byte:
        return v, "", nil
    default:
        encoded, err := json.Marshal(v)
        if err != nil {
            return nil, "", fmt.Errorf("payload cannot be encoded as JSON: %w", err)
        }
        return encoded, "application/json", nil
    }
}

func (n *Node) Validate() error {
    return nil
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"statusCode": float64(n.StatusCode)}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if v, ok := config["statusCode"].(float64); ok {
        n.StatusCode = int(v)
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("http response", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "http response",
        Type:        "http response",
        Name:        "HTTP response",
        Description: "Completes the HTTP request an upstream http in node is holding open",
        Category:    "network",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message carrying a pending HTTP request", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message after the response was sent", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "statusCode": {Type: "number", Description: "Fixed status code; 0 uses msg.statusCode, defaulting to 200", Default: float64(0), Min: floatPtr(0), Max: floatPtr(599)},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v16H4zM8 10h8M8 14h5"/></svg>`,
        Tags: []string{"network", "http", "server", "response"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
