// Package rangenode provides the Range node implementation (Node-RED type
// ID "range" - named rangenode here since "range" is a Go keyword and
// can't be a package name).
//
// Range linearly rescales a numeric property from an input range to an
// output range, optionally clamping or wrapping ("rolling") the result into
// the output range, and optionally rounding to an integer.
package rangenode

import (
    "context"
    "fmt"
    "math"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Node holds a Range node's configuration.
type Node struct {
    Property                       typedvalue.PropertyRef
    MinIn, MaxIn, MinOut, MaxOut   float64
    // Action is "scale" (plain linear rescale, the default - values outside
    // [MinIn,MaxIn] produce values outside [MinOut,MaxOut] too), "clamp"
    // (scale then clamp into [MinOut,MaxOut]), or "roll" (scale then wrap
    // into [MinOut,MaxOut]).
    Action string
    Round  bool
}

// Execute scales Property in a copy of input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    output := cloneMap(input)
    _, propResolver := resolvers(ctx, output)

    val, exists := n.Property.Get(propResolver)
    if !exists {
        return output, nil
    }

    num, ok := toFloat(val)
    if !ok {
        return nil, fmt.Errorf("range: property value %v is not numeric", val)
    }

    result := scale(num, n.MinIn, n.MaxIn, n.MinOut, n.MaxOut)
    switch n.Action {
    case "clamp":
        result = clamp(result, n.MinOut, n.MaxOut)
    case "roll":
        result = roll(result, n.MinOut, n.MaxOut)
    }
    if n.Round {
        result = math.Round(result)
    }

    if err := n.Property.Set(propResolver, result); err != nil {
        return nil, fmt.Errorf("range: %w", err)
    }
    return output, nil
}

func scale(v, minIn, maxIn, minOut, maxOut float64) float64 {
    if maxIn == minIn {
        return minOut
    }
    return (v-minIn)/(maxIn-minIn)*(maxOut-minOut) + minOut
}

func clamp(v, a, b float64) float64 {
    lo, hi := a, b
    if lo > hi {
        lo, hi = hi, lo
    }
    switch {
    case v < lo:
        return lo
    case v > hi:
        return hi
    default:
        return v
    }
}

// roll wraps v into [minOut, maxOut) using true modulo (not a loop), so it
// stays well-defined and fast for any input magnitude.
func roll(v, minOut, maxOut float64) float64 {
    span := maxOut - minOut
    if span == 0 {
        return minOut
    }
    m := math.Mod(v-minOut, span)
    if m < 0 {
        m += span
    }
    return minOut + m
}

func (n *Node) Validate() error {
    switch n.Action {
    case "", "scale", "clamp", "roll":
        return nil
    default:
        return fmt.Errorf("range: unsupported action %q", n.Action)
    }
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "property": propertyRefToConfig(n.Property),
        "minin":    n.MinIn,
        "maxin":    n.MaxIn,
        "minout":   n.MinOut,
        "maxout":   n.MaxOut,
        "action":   n.Action,
        "round":    n.Round,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Property = parsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})
    n.MinIn = getFloat(config["minin"], 0)
    n.MaxIn = getFloat(config["maxin"], 100)
    n.MinOut = getFloat(config["minout"], 0)
    n.MaxOut = getFloat(config["maxout"], 100)

    n.Action = "scale"
    if a, ok := config["action"].(string); ok && a != "" {
        n.Action = a
    }
    if r, ok := config["round"].(bool); ok {
        n.Round = r
    }
    return n.Validate()
}

func resolvers(ctx interface{}, output map[string]interface{}) (typedvalue.Resolver, typedvalue.PropertyResolver) {
    valueResolver := typedvalue.Resolver{Message: output}
    propResolver := typedvalue.PropertyResolver{Message: output}

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

func getFloat(raw interface{}, def float64) float64 {
    if f, ok := raw.(float64); ok {
        return f
    }
    return def
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
    err := reg.RegisterFactory("range", func() registry.NodeExecutor {
        return &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            MinIn: 0, MaxIn: 100, MinOut: 0, MaxOut: 100,
            Action: "scale",
        }
    }, registry.NodeMetadata{
        ID:          "range",
        Type:        "range",
        Name:        "Range",
        Description: "Scales a numeric property from one range to another, with optional clamp/roll and rounding",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message with a numeric property", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with the scaled property", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "property": {Type: "object", Description: `Property to scale, e.g. {"type":"msg","path":"payload"}`, Default: map[string]interface{}{"type": "msg", "path": "payload"}},
                "minin":    {Type: "number", Description: "Input range minimum", Default: float64(0)},
                "maxin":    {Type: "number", Description: "Input range maximum", Default: float64(100)},
                "minout":   {Type: "number", Description: "Output range minimum", Default: float64(0)},
                "maxout":   {Type: "number", Description: "Output range maximum", Default: float64(100)},
                "action":   {Type: "string", Description: "scale, clamp, or roll", Default: "scale", Enum: []string{"scale", "clamp", "roll"}},
                "round":    {Type: "boolean", Description: "Round the result to the nearest integer", Default: false},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M3 11h18v2H3z"/></svg>`,
        Tags: []string{"function", "range", "scale"},
    })
    if err != nil {
        panic(err)
    }
}
