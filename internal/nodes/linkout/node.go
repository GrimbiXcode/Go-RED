// Package linkout provides the Link Out node implementation.
//
// Link Out has no output port of its own in Node-RED - it delivers its
// input directly to one or more Link In nodes (see
// registry.NodeRuntime.SubmitToNode) instead of a drawn wire. Cross-flow
// linking (Node-RED's usual cross-tab use case) is not supported yet: a
// target must be a Link In node ID in the same flow. See
// docs/NODE_PALETTE_PLAN.md, Phase 1.
package linkout

import (
    "context"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds the target Link In node IDs to deliver to.
type Node struct {
    Links []string
}

// ExecuteMulti delivers input to every configured target via
// NodeRuntime.SubmitToNode and always sends on no output port - Link Out
// has none. If no NodeRuntime is available (e.g. Execute called outside the
// engine), it is a no-op rather than an error.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    if c, ok := ctx.(context.Context); ok {
        if rt, ok := registry.RuntimeFromContext(c); ok {
            for _, target := range n.Links {
                rt.SubmitToNode(target, input)
            }
        }
    }
    return map[string]map[string]interface{}{}, nil
}

// Execute exists only to satisfy registry.NodeExecutor (embedded in
// registry.MultiOutputExecutor) - the engine always calls ExecuteMulti for
// a node implementing it, never this.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    _, err := n.ExecuteMulti(ctx, input)
    return map[string]interface{}{}, err
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
    links := make([]interface{}, len(n.Links))
    for i, l := range n.Links {
        links[i] = l
    }
    return map[string]interface{}{"links": links}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Links = nil
    if raw, ok := config["links"].([]interface{}); ok {
        for _, v := range raw {
            if s, ok := v.(string); ok {
                n.Links = append(n.Links, s)
            }
        }
    }
    return nil
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("link out", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "link out",
        Type:        "link out",
        Name:        "Link Out",
        Description: "Delivers its input to one or more Link In nodes in the same flow; has no output port of its own",
        Category:    "flow-control",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message to forward", Required: true},
        },
        Outputs: []registry.Port{},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "links": {Type: "array", Description: "Target Link In node IDs in the same flow", Default: []interface{}{}},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#607D8B"><path d="M4 11v2h12l-5.5 5.5 1.42 1.42L19.84 12l-7.92-7.92L10.5 5.5 16 11H4z"/></svg>`,
        Tags: []string{"flow-control", "link"},
    })
    if err != nil {
        panic(err)
    }
}
