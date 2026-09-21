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
	"fmt"
	"regexp"
	"strings"

	"github.com/GrimbiXcode/Go-RED/internal/nodes/base"
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
	output := base.CloneMap(input)
	valueResolver, propResolver := base.Resolvers(ctx, output)

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
			"target":    base.PropertyRefToConfig(r.Target),
			"value":     base.ValueToConfig(r.Value),
			"moveTo":    base.PropertyRefToConfig(r.MoveTo),
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
			Target:    base.ParsePropertyRef(rm["target"], typedvalue.PropertyRef{}),
			Value:     base.ParseValue(rm["value"]),
			MoveTo:    base.ParsePropertyRef(rm["moveTo"], typedvalue.PropertyRef{}),
			From:      from,
			To:        to,
			FromRegex: fromRegex,
		})
	}
	return n.Validate()
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
					Label:       "Rules",
					Order:       1,
					Widget:      "list",
					Items: &registry.Schema{
						Properties: map[string]registry.Property{
							"action": {
								Type:    "string",
								Default: "set",
								Label:   "Action",
								Order:   1,
								Widget:  "select",
								Options: []registry.Option{{Value: "set", Label: "Set"}, {Value: "change", Label: "Change"}, {Value: "delete", Label: "Delete"}, {Value: "move", Label: "Move"}},
							},
							"target": {
								Type:       "object",
								Default:    map[string]interface{}{"type": "msg", "path": "payload"},
								Label:      "Property",
								Order:      2,
								Widget:     "typedInput",
								TypedInput: &registry.TypedInputOptions{Types: registry.PropertyRefTypes, Default: "msg"},
							},
							"value": {
								Type:        "object",
								Default:     map[string]interface{}{"type": "str", "value": ""},
								Label:       "to",
								Order:       3,
								Widget:      "typedInput",
								TypedInput:  &registry.TypedInputOptions{Types: registry.ValueTypes, Default: "str"},
								VisibleWhen: &registry.Condition{Property: "action", Values: []string{"set"}},
							},
							"from": {
								Type:        "string",
								Default:     "",
								Label:       "Search for",
								Order:       4,
								VisibleWhen: &registry.Condition{Property: "action", Values: []string{"change"}},
							},
							"to": {
								Type:        "string",
								Default:     "",
								Label:       "Replace with",
								Order:       5,
								VisibleWhen: &registry.Condition{Property: "action", Values: []string{"change"}},
							},
							"fromRegex": {
								Type:        "boolean",
								Default:     false,
								Label:       "Use regular expression",
								Order:       6,
								VisibleWhen: &registry.Condition{Property: "action", Values: []string{"change"}},
							},
							"moveTo": {
								Type:        "object",
								Default:     map[string]interface{}{"type": "msg", "path": ""},
								Label:       "to",
								Order:       7,
								Widget:      "typedInput",
								TypedInput:  &registry.TypedInputOptions{Types: registry.PropertyRefTypes, Default: "msg"},
								VisibleWhen: &registry.Condition{Property: "action", Values: []string{"move"}},
							},
						},
						Required: []string{"action", "target"},
					},
				},
			},
		},
		Help: "**Sets, changes, deletes or moves message properties** and flow/global context values. Rules run in order.\n\n* **Set** writes a value or copies another property.\n* **Change** replaces text (optionally with a regular expression).\n* **Delete** removes the property.\n* **Move** renames it.",
		Icon: "replace",
		Tags: []string{"function", "change", "set", "context"},
	})
	if err != nil {
		panic(err)
	}
}
