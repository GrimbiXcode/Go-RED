// Package junction provides the Junction node implementation.
//
// A junction has no runtime behavior of its own - in Node-RED it exists
// purely so the editor can route several wires through a single visual
// point. Go-RED's editor has no equivalent concept yet, but the node type
// still needs to exist so a flow containing one can deploy: it passes its
// input through unchanged.
package junction

import (
    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node is a pure passthrough - see the package doc.
type Node struct{}

// Execute returns input unchanged.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return input, nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} { return map[string]interface{}{} }

func (n *Node) SetConfig(config map[string]interface{}) error { return nil }

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("junction", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "junction",
        Type:        "junction",
        Name:        "Junction",
        Description: "Passes messages through unchanged; a routing point for wires",
        Category:    "flow-control",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message in", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message out, unchanged", Required: true},
        },
        ConfigSchema: registry.Schema{},
        Icon:         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#9E9E9E"><circle cx="12" cy="12" r="4"/></svg>`,
        Tags:         []string{"flow-control", "junction"},
    })
    if err != nil {
        panic(err)
    }
}
