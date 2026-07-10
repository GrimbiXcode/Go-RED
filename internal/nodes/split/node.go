// Package split provides the Split node implementation.
//
// Split breaks a single message's property (an array, an object, or a
// string) into one message per element, attaching Node-RED-style
// msg.parts grouping metadata - {id, index, count, type, ...} - as a
// regular "parts" key on each output message, the same way msg.topic or
// msg.payload are just keys of the same map[string]interface{}. There is
// no dedicated engine-level representation for this (see
// docs/NODE_PALETTE_PLAN.md, Phase 4): a Join node downstream reads the
// same "parts" key back out.
//
// Since one input message becomes N output messages all on the *same*
// output port - a different fan-out shape than registry.MultiOutputExecutor
// was designed for (at most one message per port) - only the first part is
// returned as ExecuteMulti's normal result; the rest are delivered via
// registry.NodeRuntime.SubmitToNode(rt.NodeID, ...), which re-enters this
// node's own outgoing wires exactly as if each had been ExecuteMulti's
// result (the same technique internal/nodes/trigger uses for its delayed
// second send).
package split

import (
    "context"
    "fmt"
    "sort"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/google/uuid"
)

// Node holds a Split node's configuration.
type Node struct {
    // Property is the value to split, default msg.payload.
    Property typedvalue.PropertyRef
    // Mode is "array", "string", or "object". Empty auto-detects from
    // Property's resolved Go type.
    Mode string
    // Separator is used in "string" mode; default "\n".
    Separator string
}

// ExecuteMulti splits Property and returns the first resulting message on
// the "output" port, dispatching any remaining ones via SubmitToNode.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    _, propResolver := resolvers(ctx, input)
    val, exists := n.Property.Get(propResolver)
    if !exists {
        return nil, fmt.Errorf("split: property not found")
    }

    mode := n.Mode
    if mode == "" {
        mode = detectMode(val)
        if mode == "" {
            return nil, fmt.Errorf("split: cannot auto-detect a split mode for %T; configure Mode explicitly", val)
        }
    }

    var parts []map[string]interface{}
    switch mode {
    case "array":
        arr, ok := val.([]interface{})
        if !ok {
            return nil, fmt.Errorf("split: property is not an array, got %T", val)
        }
        parts = n.splitArray(arr, input)
    case "string":
        str, ok := val.(string)
        if !ok {
            return nil, fmt.Errorf("split: property is not a string, got %T", val)
        }
        parts = n.splitString(str, input)
    case "object":
        obj, ok := val.(map[string]interface{})
        if !ok {
            return nil, fmt.Errorf("split: property is not an object, got %T", val)
        }
        parts = n.splitObject(obj, input)
    default:
        return nil, fmt.Errorf("split: unsupported mode %q", mode)
    }

    if len(parts) == 0 {
        return map[string]map[string]interface{}{}, nil
    }

    if rt, ok := runtimeFrom(ctx); ok {
        for _, p := range parts[1:] {
            rt.SubmitToNode(rt.NodeID, p)
        }
    }
    return map[string]map[string]interface{}{"output": parts[0]}, nil
}

func detectMode(val interface{}) string {
    switch val.(type) {
    case []interface{}:
        return "array"
    case map[string]interface{}:
        return "object"
    case string:
        return "string"
    default:
        return ""
    }
}

func (n *Node) splitArray(arr []interface{}, input map[string]interface{}) []map[string]interface{} {
    groupID := uuid.New().String()
    result := make([]map[string]interface{}, len(arr))
    for i, item := range arr {
        msg := cloneMap(input)
        msg["payload"] = item
        msg["parts"] = map[string]interface{}{
            "id": groupID, "index": float64(i), "count": float64(len(arr)), "type": "array",
        }
        result[i] = msg
    }
    return result
}

func (n *Node) splitString(s string, input map[string]interface{}) []map[string]interface{} {
    sep := n.Separator
    if sep == "" {
        sep = "\n"
    }
    pieces := strings.Split(s, sep)
    groupID := uuid.New().String()
    result := make([]map[string]interface{}, len(pieces))
    for i, p := range pieces {
        msg := cloneMap(input)
        msg["payload"] = p
        msg["parts"] = map[string]interface{}{
            "id": groupID, "index": float64(i), "count": float64(len(pieces)), "type": "string", "ch": sep,
        }
        result[i] = msg
    }
    return result
}

func (n *Node) splitObject(obj map[string]interface{}, input map[string]interface{}) []map[string]interface{} {
    keys := make([]string, 0, len(obj))
    for k := range obj {
        keys = append(keys, k)
    }
    sort.Strings(keys) // deterministic order (map iteration order is not)

    groupID := uuid.New().String()
    result := make([]map[string]interface{}, len(keys))
    for i, k := range keys {
        msg := cloneMap(input)
        msg["payload"] = obj[k]
        msg["topic"] = k
        msg["parts"] = map[string]interface{}{
            "id": groupID, "index": float64(i), "count": float64(len(keys)), "type": "object", "key": k,
        }
        result[i] = msg
    }
    return result
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.MultiOutputExecutor) - the engine always calls ExecuteMulti for
// a node implementing it, never this.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    outputs, err := n.ExecuteMulti(ctx, input)
    if err != nil {
        return nil, err
    }
    return outputs["output"], nil
}

func (n *Node) Validate() error {
    switch n.Mode {
    case "", "array", "string", "object":
        return nil
    default:
        return fmt.Errorf("split: unsupported mode %q", n.Mode)
    }
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "property":  propertyRefToConfig(n.Property),
        "mode":      n.Mode,
        "separator": n.Separator,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Property = parsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})
    if m, ok := config["mode"].(string); ok {
        n.Mode = m
    }
    if s, ok := config["separator"].(string); ok {
        n.Separator = s
    }
    return n.Validate()
}

func runtimeFrom(ctx interface{}) (*registry.NodeRuntime, bool) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, false
    }
    return registry.RuntimeFromContext(c)
}

func resolvers(ctx interface{}, input map[string]interface{}) (typedvalue.Resolver, typedvalue.PropertyResolver) {
    valueResolver := typedvalue.Resolver{Message: input}
    propResolver := typedvalue.PropertyResolver{Message: input}

    rt, ok := runtimeFrom(ctx)
    if !ok {
        return valueResolver, propResolver
    }
    if rt.FlowContext != nil {
        valueResolver.FlowContext = rt.FlowContext
        propResolver.FlowContext = rt.FlowContext
    }
    if rt.GlobalContext != nil {
        valueResolver.GlobalContext = rt.GlobalContext
        propResolver.GlobalContext = rt.GlobalContext
    }
    return valueResolver, propResolver
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func parsePropertyRef(raw interface{}, def typedvalue.PropertyRef) typedvalue.PropertyRef {
    m, ok := raw.(map[string]interface{})
    if !ok {
        return def
    }
    t, _ := m["type"].(string)
    p, _ := m["path"].(string)
    if t == "" {
        return def
    }
    return typedvalue.PropertyRef{Type: typedvalue.Type(t), Path: p}
}

func propertyRefToConfig(ref typedvalue.PropertyRef) map[string]interface{} {
    return map[string]interface{}{"type": string(ref.Type), "path": ref.Path}
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("split", func() registry.NodeExecutor {
        return &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    }, registry.NodeMetadata{
        ID:          "split",
        Type:        "split",
        Name:        "Split",
        Description: "Splits an array, object, or string property into one message per element, with msg.parts grouping metadata for Join/Sort",
        Category:    "flow-control",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to split", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "One message per element", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "property":  {Type: "object", Description: `Property to split, e.g. {"type":"msg","path":"payload"}`, Default: map[string]interface{}{"type": "msg", "path": "payload"}},
                "mode":      {Type: "string", Description: "array, string, or object; empty auto-detects from the property's type", Default: "", Enum: []string{"", "array", "string", "object"}},
                "separator": {Type: "string", Description: "Separator for string mode", Default: "\n"},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h7v7H4zm9 0h7v7h-7zM4 13h7v7H4zm9 0h7v7h-7z"/></svg>`,
        Tags: []string{"flow-control", "split", "sequence"},
    })
    if err != nil {
        panic(err)
    }
}
