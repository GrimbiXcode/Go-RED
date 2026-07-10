// Package linkin provides the Link In node implementation.
//
// Link In has no input port of its own in Node-RED - it's a virtual entry
// point that a Link Out node jumps to directly (see
// registry.NodeRuntime.SubmitToNode), which delivers straight to Link In's
// outgoing wires without ever calling Link In's own Execute. Execute exists
// only so the type satisfies registry.NodeExecutor and behaves safely if
// something is wired into it despite that not being the intended usage.
package linkin

import (
    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node is a pure passthrough - see the package doc.
type Node struct{}

func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return input, nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} { return map[string]interface{}{} }

func (n *Node) SetConfig(config map[string]interface{}) error { return nil }

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("link in", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "link in",
        Type:        "link in",
        Name:        "Link In",
        Description: "Entry point for a Link Out node in the same flow; has no input port of its own",
        Category:    "flow-control",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message forwarded from a Link Out node", Required: true},
        },
        ConfigSchema: registry.Schema{},
        Icon:         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#607D8B"><path d="M4 11v2h12l-5.5 5.5 1.42 1.42L19.84 12l-7.92-7.92L10.5 5.5 16 11H4z"/></svg>`,
        Tags:         []string{"flow-control", "link"},
    })
    if err != nil {
        panic(err)
    }
}
