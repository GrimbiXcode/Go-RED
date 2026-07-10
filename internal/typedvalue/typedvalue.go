// Package typedvalue resolves Node-RED-style "typed input" values: a
// (type, value) pair such as {"str", "hello"} or {"msg", "payload.count"}
// that a node's configuration uses wherever Node-RED lets a user pick
// between a literal, a message property, or flow/global context in the
// editor (the widget backing Switch's rules, Change's set/to, Template's
// fields, ...). See docs/NODE_PALETTE_PLAN.md, Phase 0 item 6 and Phase 2
// (Switch/Change/Template are its first consumers).
//
// JSONata is intentionally unsupported (see "Offene Fragen" in the plan
// doc); msg/flow/global lookups only support dot-separated map paths
// ("payload.foo.bar"), not array indexing ("payload.parts[0]").
package typedvalue

import (
    "encoding/json"
    "fmt"
    "os"
    "strconv"
    "strings"
)

// Type identifies how Value should be interpreted.
type Type string

const (
    TypeString Type = "str"
    TypeNumber Type = "num"
    TypeBool   Type = "bool"
    TypeJSON   Type = "json"
    TypeEnv    Type = "env"
    TypeMsg    Type = "msg"
    TypeFlow   Type = "flow"
    TypeGlobal Type = "global"
)

// Value is a typed input as stored in a node's configuration.
type Value struct {
    Type  Type   `json:"type"`
    Value string `json:"value"`
}

// ContextGetter is the read side of a key-value context store. Both
// engine.ContextStore and any future backend implement it structurally, so
// this package has no dependency on internal/engine (which would create an
// import cycle, since engine will eventually construct Resolvers for
// nodes).
type ContextGetter interface {
    Get(key string) (interface{}, bool)
}

// Resolver supplies the data a Value may reference: the current message,
// and this flow's/the engine's context stores. FlowContext/GlobalContext
// may be nil if unavailable (e.g. in a unit test); resolving a "flow" or
// "global" Value against a nil store is an error.
type Resolver struct {
    Message       map[string]interface{}
    FlowContext   ContextGetter
    GlobalContext ContextGetter
}

// Resolve evaluates v against r, returning the resulting Go value.
func (v Value) Resolve(r Resolver) (interface{}, error) {
    switch v.Type {
    case TypeString:
        return v.Value, nil

    case TypeNumber:
        n, err := strconv.ParseFloat(v.Value, 64)
        if err != nil {
            return nil, fmt.Errorf("typedvalue: %q is not a valid number: %w", v.Value, err)
        }
        return n, nil

    case TypeBool:
        b, err := strconv.ParseBool(v.Value)
        if err != nil {
            return nil, fmt.Errorf("typedvalue: %q is not a valid bool: %w", v.Value, err)
        }
        return b, nil

    case TypeJSON:
        var out interface{}
        if err := json.Unmarshal([]byte(v.Value), &out); err != nil {
            return nil, fmt.Errorf("typedvalue: %q is not valid JSON: %w", v.Value, err)
        }
        return out, nil

    case TypeEnv:
        return os.Getenv(v.Value), nil

    case TypeMsg:
        val, _ := lookupPath(r.Message, v.Value)
        return val, nil

    case TypeFlow:
        if r.FlowContext == nil {
            return nil, fmt.Errorf("typedvalue: flow context is not available")
        }
        val, _ := r.FlowContext.Get(v.Value)
        return val, nil

    case TypeGlobal:
        if r.GlobalContext == nil {
            return nil, fmt.Errorf("typedvalue: global context is not available")
        }
        val, _ := r.GlobalContext.Get(v.Value)
        return val, nil

    default:
        return nil, fmt.Errorf("typedvalue: unsupported type %q", v.Type)
    }
}

// lookupPath resolves a dot-separated path ("payload.foo.bar") against
// nested map[string]interface{} values, as produced by decoding JSON. A
// missing path (at any depth, or because an intermediate value isn't a
// map) resolves to (nil, false), mirroring JavaScript's "undefined" rather
// than being an error.
func lookupPath(msg map[string]interface{}, path string) (interface{}, bool) {
    if path == "" {
        return msg, msg != nil
    }

    var current interface{} = msg
    for _, segment := range strings.Split(path, ".") {
        m, ok := current.(map[string]interface{})
        if !ok {
            return nil, false
        }
        current, ok = m[segment]
        if !ok {
            return nil, false
        }
    }
    return current, true
}
