// Package switchnode provides the Switch node implementation (Node-RED type
// ID "switch" - named switchnode here since "switch" is a Go keyword and
// can't be a package name).
//
// Switch tests one property of the message against an ordered list of
// rules and routes to the output port matching each rule that matches (rule
// index N -> output port "N"), via registry.MultiOutputExecutor. Comparison
// semantics are a simplified, Go-idiomatic subset of Node-RED's JS-based
// rules: numbers compare numerically if both sides parse as one, otherwise
// as strings; there is no jsonata support (see docs/NODE_PALETTE_PLAN.md).
//
// registry.NodeMetadata.Outputs below is a display-only placeholder (2
// ports) - the actual number of live output ports is determined per node
// instance by len(Rules), which NodeMetadata (registered once per node
// *type*, not per instance) has no way to express yet. Routing itself does
// not depend on NodeMetadata at all: it works purely from the output port
// ID a ExecuteMulti result key and the matching connection's SourcePort.
package switchnode

import (
    "context"
    "fmt"
    "regexp"
    "strconv"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Rule is one entry in a Switch node's rule list. Value2 is only used by
// the "btwn" operator.
type Rule struct {
    Operator string
    Value    typedvalue.Value
    Value2   typedvalue.Value
}

// Node holds a Switch node's configuration.
type Node struct {
    Property typedvalue.PropertyRef
    Rules    []Rule
    // CheckAll, if true (the default, matching Node-RED), evaluates every
    // rule so a message can be routed to multiple outputs; if false,
    // evaluation stops at the first match.
    CheckAll bool
}

// ExecuteMulti evaluates Rules against Property and routes to output port
// "<rule index>" for each match.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    valueResolver, propResolver := resolvers(ctx, input)
    testVal, testExists := n.Property.Get(propResolver)

    outputs := make(map[string]map[string]interface{})
    for i, rule := range n.Rules {
        matched, err := evaluateRule(rule, testVal, testExists, valueResolver)
        if err != nil {
            return nil, fmt.Errorf("switch: rule %d: %w", i, err)
        }
        if !matched {
            continue
        }
        outputs[strconv.Itoa(i)] = cloneMap(input)
        if !n.CheckAll {
            break
        }
    }
    return outputs, nil
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.MultiOutputExecutor) - the engine always calls ExecuteMulti for
// a node implementing it, never this.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    _, err := n.ExecuteMulti(ctx, input)
    return input, err
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
    rules := make([]interface{}, len(n.Rules))
    for i, r := range n.Rules {
        rules[i] = map[string]interface{}{
            "operator": r.Operator,
            "value":    valueToConfig(r.Value),
            "value2":   valueToConfig(r.Value2),
        }
    }
    return map[string]interface{}{
        "property": propertyRefToConfig(n.Property),
        "checkAll": n.CheckAll,
        "rules":    rules,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Property = parsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})

    n.CheckAll = true
    if checkAll, ok := config["checkAll"].(bool); ok {
        n.CheckAll = checkAll
    }

    n.Rules = nil
    if rawRules, ok := config["rules"].([]interface{}); ok {
        for _, rr := range rawRules {
            rm, ok := rr.(map[string]interface{})
            if !ok {
                continue
            }
            operator, _ := rm["operator"].(string)
            n.Rules = append(n.Rules, Rule{
                Operator: operator,
                Value:    parseValue(rm["value"]),
                Value2:   parseValue(rm["value2"]),
            })
        }
    }
    return n.Validate()
}

// resolvers builds the typedvalue Resolver/PropertyResolver pair for this
// invocation. registry.NodeRuntime's FlowContext/GlobalContext are
// *registry.ContextStore, which can be nil (no runtime, e.g. a direct unit
// test) - checking that on the concrete pointer, before it is boxed into an
// interface field, is required: a nil *registry.ContextStore stored in a
// typedvalue.ContextGetter/ContextAccessor interface value is NOT a nil
// interface, so a naive "FlowContext: rt.FlowContext" would make
// PropertyRef/Value's own nil checks useless and panic on first use.
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

func evaluateRule(rule Rule, testVal interface{}, testExists bool, resolver typedvalue.Resolver) (bool, error) {
    switch rule.Operator {
    case "else":
        return true, nil
    case "true":
        b, ok := testVal.(bool)
        return ok && b, nil
    case "false":
        b, ok := testVal.(bool)
        return ok && !b, nil
    case "null":
        return testExists && testVal == nil, nil
    case "nnull":
        return testExists && testVal != nil, nil
    case "empty":
        return isEmpty(testVal), nil
    case "nempty":
        return !isEmpty(testVal), nil
    }

    // Every remaining operator compares testVal against rule.Value, which
    // requires the property to actually be present.
    if !testExists {
        return false, nil
    }

    ruleVal, err := rule.Value.Resolve(resolver)
    if err != nil {
        return false, err
    }

    switch rule.Operator {
    case "eq":
        return looseEqual(testVal, ruleVal), nil
    case "neq":
        return !looseEqual(testVal, ruleVal), nil
    case "lt":
        return compare(testVal, ruleVal) < 0, nil
    case "lte":
        return compare(testVal, ruleVal) <= 0, nil
    case "gt":
        return compare(testVal, ruleVal) > 0, nil
    case "gte":
        return compare(testVal, ruleVal) >= 0, nil
    case "btwn":
        val2, err := rule.Value2.Resolve(resolver)
        if err != nil {
            return false, err
        }
        return compare(testVal, ruleVal) >= 0 && compare(testVal, val2) <= 0, nil
    case "cont":
        return strings.Contains(toStr(testVal), toStr(ruleVal)), nil
    case "regex":
        re, err := regexp.Compile(toStr(ruleVal))
        if err != nil {
            return false, fmt.Errorf("invalid regex %q: %w", toStr(ruleVal), err)
        }
        return re.MatchString(toStr(testVal)), nil
    default:
        return false, fmt.Errorf("unsupported operator %q", rule.Operator)
    }
}

// compare returns -1/0/1. Both sides are compared numerically if they both
// parse as a number, otherwise lexicographically as strings.
func compare(a, b interface{}) int {
    if af, aok := toFloat(a); aok {
        if bf, bok := toFloat(b); bok {
            switch {
            case af < bf:
                return -1
            case af > bf:
                return 1
            default:
                return 0
            }
        }
    }
    return strings.Compare(toStr(a), toStr(b))
}

func looseEqual(a, b interface{}) bool {
    if af, aok := toFloat(a); aok {
        if bf, bok := toFloat(b); bok {
            return af == bf
        }
    }
    return toStr(a) == toStr(b)
}

func toFloat(v interface{}) (float64, bool) {
    switch n := v.(type) {
    case float64:
        return n, true
    case int:
        return float64(n), true
    case int64:
        return float64(n), true
    case string:
        f, err := strconv.ParseFloat(n, 64)
        return f, err == nil
    default:
        return 0, false
    }
}

func toStr(v interface{}) string {
    if s, ok := v.(string); ok {
        return s
    }
    return fmt.Sprintf("%v", v)
}

func isEmpty(v interface{}) bool {
    switch t := v.(type) {
    case nil:
        return true
    case string:
        return t == ""
    case []interface{}:
        return len(t) == 0
    case map[string]interface{}:
        return len(t) == 0
    default:
        return false
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

func parseValue(raw interface{}) typedvalue.Value {
    m, ok := raw.(map[string]interface{})
    if !ok {
        return typedvalue.Value{}
    }
    t, _ := m["type"].(string)
    v, _ := m["value"].(string)
    return typedvalue.Value{Type: typedvalue.Type(t), Value: v}
}

func valueToConfig(v typedvalue.Value) map[string]interface{} {
    return map[string]interface{}{"type": string(v.Type), "value": v.Value}
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("switch", func() registry.NodeExecutor {
        return &Node{
            Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
            CheckAll: true,
        }
    }, registry.NodeMetadata{
        ID:          "switch",
        Type:        "switch",
        Name:        "Switch",
        Description: "Routes a message to one or more outputs based on rules evaluated against a property",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to test", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "0", Name: "1", Description: "Fires when rule 1 matches"},
            {ID: "1", Name: "2", Description: "Fires when rule 2 matches"},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "property": {
                    Type:        "object",
                    Description: `Property to test, e.g. {"type":"msg","path":"payload"}`,
                    Default:     map[string]interface{}{"type": "msg", "path": "payload"},
                },
                "checkAll": {
                    Type:        "boolean",
                    Description: "Evaluate every rule (true) or stop at the first match (false)",
                    Default:     true,
                },
                "rules": {
                    Type:        "array",
                    Description: `Ordered [{"operator","value","value2"}] rules; rule index N routes to output port "N"`,
                    Default:     []interface{}{},
                },
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M9.6 15.6L4.8 12l4.8-3.6v2.4H15V7.2L19.2 12 15 16.8v-2.4H9.6v1.2z"/></svg>`,
        Tags: []string{"function", "switch", "route", "condition"},
    })
    if err != nil {
        panic(err)
    }
}
