// Package comment provides the Comment node implementation.
//
// A comment is an editor annotation with no runtime effect: it has no
// input or output ports, so its Execute is never actually invoked by the
// engine in a well-formed flow. It still needs to be a registered node type
// so a flow containing one can deploy.
package comment

import (
    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds the annotation text. It has no meaningful runtime behavior.
type Node struct {
    Text string
}

// Execute returns input unchanged; never actually invoked since a comment
// has no wired ports.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return input, nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"text": n.Text}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    if text, ok := config["text"].(string); ok {
        n.Text = text
    }
    return nil
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("comment", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "comment",
        Type:        "comment",
        Name:        "Comment",
        Description: "An annotation with no runtime effect; has no input or output ports",
        Category:    "flow-control",
        Inputs:      []registry.Port{},
        Outputs:     []registry.Port{},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "text": {Type: "string", Description: "Comment text", Default: ""},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#FFC107"><path d="M3 3h18v14H5.17L3 19.17V3z"/></svg>`,
        Tags: []string{"flow-control", "comment", "annotation"},
    })
    if err != nil {
        panic(err)
    }
}
