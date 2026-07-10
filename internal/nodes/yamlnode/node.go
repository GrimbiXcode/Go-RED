// Package yamlnode provides the YAML node implementation (Node-RED type ID
// "yaml" - named yamlnode here to avoid shadowing the imported yaml.v3
// package within this file).
//
// YAML converts msg.payload bidirectionally based on its current type: a
// string is parsed as YAML, anything else is marshaled to a YAML string.
package yamlnode

import (
    "fmt"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "gopkg.in/yaml.v3"
)

// Node holds a YAML node's configuration. YAML has no pretty/compact
// distinction the way JSON does, so there is nothing to configure yet.
type Node struct{}

// Execute parses input["payload"] if it's a string, or marshals it to a
// YAML string otherwise.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    output := cloneMap(input)

    switch payload := input["payload"].(type) {
    case string:
        var parsed interface{}
        if err := yaml.Unmarshal([]byte(payload), &parsed); err != nil {
            return nil, fmt.Errorf("yaml: %w", err)
        }
        output["payload"] = normalize(parsed)
    default:
        data, err := yaml.Marshal(payload)
        if err != nil {
            return nil, fmt.Errorf("yaml: %w", err)
        }
        output["payload"] = string(data)
    }
    return output, nil
}

// normalize converts the map[string]interface{} yaml.v3 usually produces
// for mappings into itself unchanged, but recurses to convert any nested
// map[interface{}]interface{} (which yaml.v3 can still produce for
// non-string keys) into map[string]interface{} so downstream nodes get the
// same shape encoding/json would produce, keeping payloads consistent
// across parser nodes regardless of which one built them.
func normalize(v interface{}) interface{} {
    switch t := v.(type) {
    case map[string]interface{}:
        out := make(map[string]interface{}, len(t))
        for k, val := range t {
            out[k] = normalize(val)
        }
        return out
    case map[interface{}]interface{}:
        out := make(map[string]interface{}, len(t))
        for k, val := range t {
            out[fmt.Sprintf("%v", k)] = normalize(val)
        }
        return out
    case []interface{}:
        out := make([]interface{}, len(t))
        for i, val := range t {
            out[i] = normalize(val)
        }
        return out
    default:
        return v
    }
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} { return map[string]interface{}{} }

func (n *Node) SetConfig(config map[string]interface{}) error { return nil }

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("yaml", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "yaml",
        Type:        "yaml",
        Name:        "YAML",
        Description: "Converts msg.payload between a YAML string and an object, based on its current type",
        Category:    "parser",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message with a YAML string or object payload", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with the converted payload", Required: true},
        },
        ConfigSchema: registry.Schema{},
        Icon:         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h16v2H4zm0 5h10v2H4zm0 5h16v2H4zm0 5h10v2H4z"/></svg>`,
        Tags:         []string{"parser", "yaml"},
    })
    if err != nil {
        panic(err)
    }
}
