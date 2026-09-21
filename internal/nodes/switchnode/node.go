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
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/GrimbiXcode/Go-RED/internal/nodes/base"
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
	valueResolver, propResolver := base.Resolvers(ctx, input)
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
		outputs[strconv.Itoa(i)] = base.CloneMap(input)
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
			"value":    base.ValueToConfig(r.Value),
			"value2":   base.ValueToConfig(r.Value2),
		}
	}
	return map[string]interface{}{
		"property": base.PropertyRefToConfig(n.Property),
		"checkAll": n.CheckAll,
		"rules":    rules,
	}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
	n.Property = base.ParsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})

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
				Value:    base.ParseValue(rm["value"]),
				Value2:   base.ParseValue(rm["value2"]),
			})
		}
	}
	return n.Validate()
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
		return strings.Contains(base.ToString(testVal), base.ToString(ruleVal)), nil
	case "regex":
		re, err := regexp.Compile(base.ToString(ruleVal))
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", base.ToString(ruleVal), err)
		}
		return re.MatchString(base.ToString(testVal)), nil
	default:
		return false, fmt.Errorf("unsupported operator %q", rule.Operator)
	}
}

// compare returns -1/0/1. Both sides are compared numerically if they both
// parse as a number, otherwise lexicographically as strings.
func compare(a, b interface{}) int {
	if af, aok := base.ParseFloat(a); aok {
		if bf, bok := base.ParseFloat(b); bok {
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
	return strings.Compare(base.ToString(a), base.ToString(b))
}

func looseEqual(a, b interface{}) bool {
	if af, aok := base.ParseFloat(a); aok {
		if bf, bok := base.ParseFloat(b); bok {
			return af == bf
		}
	}
	return base.ToString(a) == base.ToString(b)
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
					Label:       "Property",
					Order:       1,
					Widget:      "typedInput",
					TypedInput:  &registry.TypedInputOptions{Types: registry.PropertyRefTypes, Default: "msg"},
				},
				"rules": {
					Type:        "array",
					Description: `Ordered [{"operator","value","value2"}] rules; rule index N routes to output port "N"`,
					Default:     []interface{}{},
					Label:       "Rules",
					Order:       2,
					Widget:      "list",
					Items: &registry.Schema{
						Properties: map[string]registry.Property{
							"operator": {
								Type:    "string",
								Default: "eq",
								Label:   "Operator",
								Order:   1,
								Widget:  "select",
								Options: []registry.Option{{Value: "eq", Label: "=="}, {Value: "neq", Label: "!="}, {Value: "lt", Label: "<"}, {Value: "lte", Label: "<="}, {Value: "gt", Label: ">"}, {Value: "gte", Label: ">="}, {Value: "btwn", Label: "is between"}, {Value: "cont", Label: "contains"}, {Value: "regex", Label: "matches regex"}, {Value: "true", Label: "is true"}, {Value: "false", Label: "is false"}, {Value: "null", Label: "is null"}, {Value: "nnull", Label: "is not null"}, {Value: "empty", Label: "is empty"}, {Value: "nempty", Label: "is not empty"}, {Value: "else", Label: "otherwise"}},
							},
							"value": {
								Type:        "object",
								Default:     map[string]interface{}{"type": "str", "value": ""},
								Label:       "Value",
								Order:       2,
								Widget:      "typedInput",
								TypedInput:  &registry.TypedInputOptions{Types: registry.ValueTypes, Default: "str"},
								VisibleWhen: &registry.Condition{Property: "operator", Values: []string{"eq", "neq", "lt", "lte", "gt", "gte", "btwn", "cont", "regex"}},
							},
							"value2": {
								Type:        "object",
								Default:     map[string]interface{}{"type": "str", "value": ""},
								Label:       "and",
								Order:       3,
								Widget:      "typedInput",
								TypedInput:  &registry.TypedInputOptions{Types: registry.ValueTypes, Default: "str"},
								VisibleWhen: &registry.Condition{Property: "operator", Values: []string{"btwn"}},
							},
						},
						Required: []string{"operator"},
					},
				},
				"checkAll": {
					Type:        "boolean",
					Description: "Evaluate every rule (true) or stop at the first match (false)",
					Default:     true,
					Label:       "Check all rules",
					Order:       3,
					Widget:      "boolean",
				},
			},
		},
		Help:        "**Routes messages by rules.** Each rule owns one output port; a message is sent to the port of every rule that matches (or only the first, with *Check all rules* off). *otherwise* matches when no earlier rule did.",
		OutputsFrom: &registry.OutputsFrom{Property: "rules", Label: "{{operator}} {{value.value}}"},
		Icon:        "route",
		Tags:        []string{"function", "switch", "route", "condition"},
	})
	if err != nil {
		panic(err)
	}
}
