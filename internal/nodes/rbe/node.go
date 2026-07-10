// Package rbe provides the RBE ("report by exception") / Filter node
// implementation.
//
// RBE suppresses a message unless its tested property differs from the
// last one that passed through - "rbe" mode compares for any change
// (deep equality), "deadband" mode compares numerically and only passes a
// change of at least Gap. Node-RED's "narrowband" modes and percent-based
// deadband gaps are not implemented (see docs/NODE_PALETTE_PLAN.md); use
// "rbe" or absolute "deadband".
package rbe

import (
    "context"
    "fmt"
    "math"
    "reflect"
    "sync"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Node holds an RBE node's configuration and per-topic last-value state.
type Node struct {
    // Mode is "rbe" (any change) or "deadband" (numeric change >= Gap).
    Mode string
    Gap  float64
    // Property is the message property compared across messages, default
    // msg.payload.
    Property typedvalue.PropertyRef
    // SeparateTopics tracks a distinct last value per msg.topic instead of
    // one shared last value for the node.
    SeparateTopics bool

    mu        sync.Mutex
    lastValue map[string]interface{}
}

// ExecuteMulti sends a copy of input on the "output" port only if it passes
// the configured filter; otherwise it sends on no port.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    _, propResolver := resolvers(ctx, input)
    val, exists := n.Property.Get(propResolver)
    if !exists {
        return map[string]map[string]interface{}{}, nil
    }

    topic := ""
    if n.SeparateTopics {
        if t, ok := input["topic"].(string); ok {
            topic = t
        }
    }

    pass, err := n.evaluate(topic, val)
    if err != nil {
        return nil, err
    }
    if !pass {
        return map[string]map[string]interface{}{}, nil
    }
    return map[string]map[string]interface{}{"output": cloneMap(input)}, nil
}

// evaluate compares val against the last value seen for topic, updating
// the stored last value if val passes.
func (n *Node) evaluate(topic string, val interface{}) (bool, error) {
    n.mu.Lock()
    defer n.mu.Unlock()

    if n.lastValue == nil {
        n.lastValue = make(map[string]interface{})
    }
    prev, hadPrev := n.lastValue[topic]

    var pass bool
    switch n.Mode {
    case "deadband":
        curNum, ok := toFloat(val)
        if !ok {
            return false, fmt.Errorf("rbe: deadband mode requires a numeric property, got %v", val)
        }
        prevNum, prevOk := toFloat(prev)
        pass = !hadPrev || !prevOk || math.Abs(curNum-prevNum) >= n.Gap
    default: // "rbe"
        pass = !hadPrev || !reflect.DeepEqual(prev, val)
    }

    if pass {
        n.lastValue[topic] = val
    }
    return pass, nil
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
    case "", "rbe", "deadband":
        return nil
    default:
        return fmt.Errorf("rbe: unsupported mode %q", n.Mode)
    }
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "mode":           n.Mode,
        "gap":            n.Gap,
        "property":       propertyRefToConfig(n.Property),
        "separateTopics": n.SeparateTopics,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Mode = "rbe"
    if m, ok := config["mode"].(string); ok && m != "" {
        n.Mode = m
    }
    if g, ok := config["gap"].(float64); ok {
        n.Gap = g
    }
    n.Property = parsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})
    if s, ok := config["separateTopics"].(bool); ok {
        n.SeparateTopics = s
    }
    return n.Validate()
}

func resolvers(ctx interface{}, input map[string]interface{}) (typedvalue.Resolver, typedvalue.PropertyResolver) {
    valueResolver := typedvalue.Resolver{Message: input}
    propResolver := typedvalue.PropertyResolver{Message: input}

    c, ok := ctx.(context.Context)
    if !ok {
        return valueResolver, propResolver
    }
    rt, ok := registry.RuntimeFromContext(c)
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

func toFloat(v interface{}) (float64, bool) {
    switch n := v.(type) {
    case float64:
        return n, true
    case int:
        return float64(n), true
    case int64:
        return float64(n), true
    default:
        return 0, false
    }
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
    err := reg.RegisterFactory("rbe", func() registry.NodeExecutor {
        return &Node{
            Mode:     "rbe",
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
        }
    }, registry.NodeMetadata{
        ID:          "rbe",
        Type:        "rbe",
        Name:        "Filter",
        Description: "Blocks messages unless a property has changed (rbe) or changed by at least a numeric gap (deadband)",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to filter", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Fires only when the message passes the filter", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "mode":           {Type: "string", Description: "rbe (any change) or deadband (numeric change >= gap)", Default: "rbe", Enum: []string{"rbe", "deadband"}},
                "gap":            {Type: "number", Description: "Minimum change required in deadband mode", Default: float64(0), Min: floatPtr(0)},
                "property":       {Type: "object", Description: `Property to compare, e.g. {"type":"msg","path":"payload"}`, Default: map[string]interface{}{"type": "msg", "path": "payload"}},
                "separateTopics": {Type: "boolean", Description: "Track a separate last value per msg.topic", Default: false},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 5h16l-6 8v6l-4-2v-4z"/></svg>`,
        Tags: []string{"function", "filter", "rbe", "deadband"},
    })
    if err != nil {
        panic(err)
    }
}

func floatPtr(f float64) *float64 { return &f }
