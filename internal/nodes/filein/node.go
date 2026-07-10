// Package filein provides the "File in" node implementation (Node-RED type
// ID "file in" - reads a file into the message).
//
// Format controls how the file becomes message(s):
//   - "" (default): the whole file as raw bytes (msg.payload is []byte -
//     Node-RED's "buffer" output).
//   - "utf8": the whole file decoded as a UTF-8 string.
//   - "lines": the file split on "\n" into one message per line, each
//     carrying msg.parts grouping metadata (see internal/nodes/split for
//     the same convention). One input message becomes N output messages on
//     the same port, so - like Split - only the first line is returned via
//     ExecuteMulti's normal result; the rest go through
//     registry.NodeRuntime.SubmitToNode(rt.NodeID, ...).
//
// Node-RED's "stream" format (chunked reads honoring the OS read
// buffer's high-water mark, for very large files) is not implemented -
// this reads the whole file into memory - see docs/NODE_PALETTE_PLAN.md
// for the documented scope cut. Node-RED's optional per-run "send an
// error-shaped message on read failure" (sendError) is also not
// implemented: a read failure is reported as a Go error the same way
// every other node in this codebase reports one.
package filein

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/google/uuid"
)

// Node holds a File-in node's configuration.
type Node struct {
    // Filename resolves to the path to read.
    Filename typedvalue.Value
    // Format is "" (raw bytes), "utf8" (string), or "lines" (one message
    // per line, with msg.parts).
    Format string
    // AllProps, in "lines" mode, clones the whole input message onto each
    // line's message instead of only topic/filename.
    AllProps bool
}

// ExecuteMulti resolves Filename, reads it, and returns the file's content
// per Format - a single message on "output", or (in "lines" mode) the
// first line via the normal return with the rest dispatched through
// SubmitToNode.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    valueResolver, _ := resolvers(ctx, input)
    raw, err := n.Filename.Resolve(valueResolver)
    if err != nil {
        return nil, fmt.Errorf("filein: %w", err)
    }
    filename := toFilename(raw)
    if filename == "" {
        return map[string]map[string]interface{}{}, nil
    }

    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("filein: read %s: %w", filename, err)
    }

    switch n.Format {
    case "lines":
        return n.executeLines(ctx, input, filename, data)
    case "utf8":
        out := cloneMap(input)
        out["filename"] = filename
        out["payload"] = string(data)
        return map[string]map[string]interface{}{"output": out}, nil
    default:
        out := cloneMap(input)
        out["filename"] = filename
        out["payload"] = data
        return map[string]map[string]interface{}{"output": out}, nil
    }
}

func (n *Node) executeLines(ctx interface{}, input map[string]interface{}, filename string, data []byte) (map[string]map[string]interface{}, error) {
    lines := strings.Split(string(data), "\n")
    groupID := uuid.New().String()

    msgs := make([]map[string]interface{}, len(lines))
    for i, line := range lines {
        var m map[string]interface{}
        if n.AllProps {
            m = cloneMap(input)
        } else {
            m = map[string]interface{}{}
            if topic, ok := input["topic"]; ok {
                m["topic"] = topic
            }
        }
        m["filename"] = filename
        m["payload"] = line
        m["parts"] = map[string]interface{}{
            "id": groupID, "index": float64(i), "count": float64(len(lines)), "type": "string", "ch": "\n",
        }
        msgs[i] = m
    }

    if len(msgs) == 0 {
        return map[string]map[string]interface{}{}, nil
    }

    if rt, ok := runtimeFrom(ctx); ok {
        for _, m := range msgs[1:] {
            rt.SubmitToNode(rt.NodeID, m)
        }
    }
    return map[string]map[string]interface{}{"output": msgs[0]}, nil
}

func toFilename(v interface{}) string {
    switch s := v.(type) {
    case string:
        return s
    case nil:
        return ""
    default:
        return fmt.Sprint(s)
    }
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
    switch n.Format {
    case "", "utf8", "lines":
        return nil
    default:
        return fmt.Errorf("filein: unsupported format %q", n.Format)
    }
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "filename": valueToConfig(n.Filename),
        "format":   n.Format,
        "allProps": n.AllProps,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Filename = parseValue(config["filename"])
    if f, ok := config["format"].(string); ok {
        n.Format = f
    }
    if ap, ok := config["allProps"].(bool); ok {
        n.AllProps = ap
    }
    return n.Validate()
}

func runtimeFrom(ctx interface{}) (*registry.NodeRuntime, bool) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, false
    }
    return registry.RuntimeFromContext(c)
}

func resolvers(ctx interface{}, input map[string]interface{}) (typedvalue.Resolver, typedvalue.PropertyResolver) {
    valueResolver := typedvalue.Resolver{Message: input}
    propResolver := typedvalue.PropertyResolver{Message: input}

    rt, ok := runtimeFrom(ctx)
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
    err := reg.RegisterFactory("file in", func() registry.NodeExecutor {
        return &Node{Filename: typedvalue.Value{Type: typedvalue.TypeString, Value: ""}}
    }, registry.NodeMetadata{
        ID:          "file in",
        Type:        "file in",
        Name:        "File in",
        Description: "Reads a file into msg.payload, as raw bytes, a UTF-8 string, or one message per line",
        Category:    "storage",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "Message that triggers the read", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "File content (single message, or one per line in lines mode)", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "filename": {Type: "object", Description: `Path to read, e.g. {"type":"str","value":"/tmp/in.txt"}`, Default: map[string]interface{}{"type": "str", "value": ""}},
                "format":   {Type: "string", Description: "\"\" (raw bytes), utf8 (string), or lines (one message per line)", Default: "", Enum: []string{"", "utf8", "lines"}},
                "allProps": {Type: "boolean", Description: "lines mode: clone the whole input message onto each line instead of only topic/filename", Default: false},
            },
            Required: []string{"filename"},
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M6 2h9l5 5v15H6zm8 1.5V8h4.5zM8 12h8v2H8zm0 4h8v2H8z"/></svg>`,
        Tags: []string{"storage", "file", "read"},
    })
    if err != nil {
        panic(err)
    }
}
