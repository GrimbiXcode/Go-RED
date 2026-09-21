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
	"fmt"
	"math"
	"reflect"
	"sync"

	"github.com/GrimbiXcode/Go-RED/internal/nodes/base"
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
	_, propResolver := base.Resolvers(ctx, input)
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
	return map[string]map[string]interface{}{"output": base.CloneMap(input)}, nil
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
		curNum, ok := base.ToFloat(val)
		if !ok {
			return false, fmt.Errorf("rbe: deadband mode requires a numeric property, got %v", val)
		}
		prevNum, prevOk := base.ToFloat(prev)
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
		"property":       base.PropertyRefToConfig(n.Property),
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
	n.Property = base.ParsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})
	if s, ok := config["separateTopics"].(bool); ok {
		n.SeparateTopics = s
	}
	return n.Validate()
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
				"property": {
					Type:        "object",
					Description: `Property to compare, e.g. {"type":"msg","path":"payload"}`,
					Default:     map[string]interface{}{"type": "msg", "path": "payload"},
					Label:       "Property",
					Order:       1,
					Widget:      "typedInput",
					TypedInput:  &registry.TypedInputOptions{Types: registry.PropertyRefTypes, Default: "msg"},
				},
				"mode": {
					Type:        "string",
					Description: "rbe (any change) or deadband (numeric change >= gap)",
					Default:     "rbe",
					Label:       "Mode",
					Order:       2,
					Widget:      "select",
					Options:     []registry.Option{{Value: "rbe", Label: "Block unless the value changes"}, {Value: "deadband", Label: "Block unless the change exceeds a gap"}},
				},
				"gap": {
					Type:        "number",
					Description: "Minimum change required in deadband mode",
					Default:     float64(0),
					Min:         base.FloatPtr(0),
					Label:       "Gap",
					Order:       3,
					Widget:      "number",
					VisibleWhen: &registry.Condition{Property: "mode", Values: []string{"deadband"}},
				},
				"separateTopics": {
					Type:        "boolean",
					Description: "Track a separate last value per msg.topic",
					Default:     false,
					Label:       "Track each topic separately",
					Order:       4,
					Widget:      "boolean",
				},
			},
		},
		Help: "**Report by exception:** passes a message only when the watched value changed since the last one (or changed by more than a gap).",
		Icon: "filter",
		Tags: []string{"function", "filter", "rbe", "deadband"},
	})
	if err != nil {
		panic(err)
	}
}
