// Package change provides the Change node implementation.
//
// Change applies an ordered list of rules to a message: set a property to a
// value, delete a property, move a property to another location, or
// search/replace within a string property. Each rule's target (and Move's
// destination) can be a message property path or a flow/global context key
// via typedvalue.PropertyRef, so rules can read and write flow/global
// context, not just msg.
package change

import (
    "context"
    "fmt"
    "regexp"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Rule is one entry in a Change node's rule list. Which fields are used
// depends on Action:
//   - "set": Target = Value
//   - "delete": remove Target
//   - "move": Target's value moves to MoveTo (Target is removed)
//   - "change": within Target's current string value, replace From with To
//     (regexp if FromRegex)
type Rule struct {
    Action    string
    Target    typedvalue.PropertyRef
    Value     typedvalue.Value
    MoveTo    typedvalue.PropertyRef
    From      string
    To        string
    FromRegex bool
}

// Node holds a Change node's ordered rule list.
type Node struct {
    Rules []Rule
}

// Execute applies Rules in order to a copy of input.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    output := cloneMap(input)
    valueResolver, propResolver := resolvers(ctx, output)

    for i, rule := range n.Rules {
        if err := applyRule(rule, valueResolver, propResolver); err != nil {
            return nil, fmt.Errorf("change: rule %d (%s): %w", i, rule.Action, err)
        }
    }
    return output, nil
}

func applyRule(rule Rule, valueResolver typedvalue.Resolver, propResolver typedvalue.PropertyResolver) error {
    switch rule.Action {
    case "set":
        val, err := rule.Value.Resolve(valueResolver)
        if err != nil {
            return err
        }
        return rule.Target.Set(propResolver, val)

    case "delete":
        return rule.Target.Delete(propResolver)

    case "move":
        val, exists := rule.Target.Get(propResolver)
        if !exists {
            return nil
        }
        if err := rule.Target.Delete(propResolver); err != nil {
            return err
        }
        return rule.MoveTo.Set(propResolver, val)

    case "change":
        val, exists := rule.Target.Get(propResolver)
        if !exists {
            return nil
        }
        str, ok := val.(string)
        if !ok {
            // Search/replace only applies to strings, matching Node-RED;
            // silently skip non-string targets rather than erroring.
            return nil
        }

        var replaced string
        if rule.FromRegex {
            re, err := regexp.Compile(rule.From)
            if err != nil {
                return fmt.Errorf("invalid regex %q: %w", rule.From, err)
            }
            replaced = re.ReplaceAllString(str, rule.To)
        } else {
            replaced = strings.ReplaceAll(str, rule.From, rule.To)
        }
        return rule.Target.Set(propResolver, replaced)

    default:
        return fmt.Errorf("unsupported action %q", rule.Action)
    }
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
    rules := make([]interface{}, len(n.Rules))
    for i, r := range n.Rules {
        rules[i] = map[string]interface{}{
            "action":    r.Action,
            "target":    propertyRefToConfig(r.Target),
            "value":     valueToConfig(r.Value),
            "moveTo":    propertyRefToConfig(r.MoveTo),
            "from":      r.From,
            "to":        r.To,
            "fromRegex": r.FromRegex,
        }
    }
    return map[string]interface{}{"rules": rules}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Rules = nil
    rawRules, ok := config["rules"].([]interface{})
    if !ok {
        return n.Validate()
    }
    for _, rr := range rawRules {
        rm, ok := rr.(map[string]interface{})
        if !ok {
            continue
        }
        action, _ := rm["action"].(string)
        from, _ := rm["from"].(string)
        to, _ := rm["to"].(string)
        fromRegex, _ := rm["fromRegex"].(bool)

        n.Rules = append(n.Rules, Rule{
            Action:    action,
            Target:    parsePropertyRef(rm["target"], typedvalue.PropertyRef{}),
            Value:     parseValue(rm["value"]),
            MoveTo:    parsePropertyRef(rm["moveTo"], typedvalue.PropertyRef{}),
            From:      from,
            To:        to,
            FromRegex: fromRegex,
        })
    }
    return n.Validate()
}

// resolvers builds the typedvalue Resolver/PropertyResolver pair for this
// invocation, nil-safe against registry.NodeRuntime's FlowContext/
// GlobalContext being nil *registry.ContextStore pointers (see
// switchnode.resolvers for why this can't be a naive field assignment).
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
    err := reg.RegisterFactory("change", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "change",
        Type:        "change",
        Name:        "Change",
        Description: "Sets, changes, deletes, or moves message properties and flow/global context values",
        Category:    "function",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to modify", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Modified message", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "rules": {
                    Type:        "array",
                    Description: `Ordered [{"action","target","value","moveTo","from","to","fromRegex"}] rules, applied in order`,
                    Default:     []interface{}{},
                },
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04a1 1 0 000-1.41l-2.34-2.34a1 1 0 00-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"/></svg>`,
        Tags: []string{"function", "change", "set", "context"},
    })
    if err != nil {
        panic(err)
    }
}
