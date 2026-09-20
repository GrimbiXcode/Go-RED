// Package jsonnode provides the JSON node implementation (Node-RED type ID
// "json" - named jsonnode here to avoid shadowing the imported
// encoding/json package within this file).
//
// JSON converts msg.payload bidirectionally based on its current type: a
// string is parsed as JSON, anything else is marshaled to a JSON string.
package jsonnode

import (
	"encoding/json"
	"fmt"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds a JSON node's configuration.
type Node struct {
	// Pretty indents the output when stringifying.
	Pretty bool
}

// Execute parses input["payload"] if it's a string, or marshals it to a
// JSON string otherwise.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	output := cloneMap(input)

	switch payload := input["payload"].(type) {
	case string:
		var parsed interface{}
		if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		output["payload"] = parsed
	default:
		var (
			data []byte
			err  error
		)
		if n.Pretty {
			data, err = json.MarshalIndent(payload, "", "  ")
		} else {
			data, err = json.Marshal(payload)
		}
		if err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		output["payload"] = string(data)
	}
	return output, nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
	return map[string]interface{}{"pretty": n.Pretty}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
	if p, ok := config["pretty"].(bool); ok {
		n.Pretty = p
	}
	return n.Validate()
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func init() {
	reg := registry.GetGlobalRegistry()
	err := reg.RegisterFactory("json", func() registry.NodeExecutor {
		return &Node{}
	}, registry.NodeMetadata{
		ID:          "json",
		Type:        "json",
		Name:        "JSON",
		Description: "Converts msg.payload between a JSON string and an object, based on its current type",
		Category:    "parser",
		Inputs: []registry.Port{
			{ID: "input", Name: "Input", Description: "Message with a JSON string or object payload", Required: true},
		},
		Outputs: []registry.Port{
			{ID: "output", Name: "Output", Description: "Message with the converted payload", Required: true},
		},
		ConfigSchema: registry.Schema{
			Properties: map[string]registry.Property{
				"pretty": {
					Type:        "boolean",
					Description: "Indent the output when stringifying",
					Default:     false,
					Label:       "Pretty-print output",
					Order:       1,
					Widget:      "boolean",
				},
			},
		},
		Help: "**Converts between JSON text and objects** in `msg.payload`: a string is parsed, anything else is serialised.",
		Icon: "braces",
		Tags: []string{"parser", "json"},
	})
	if err != nil {
		panic(err)
	}
}
