// Package join provides the Join node implementation.
//
// Join is Split's counterpart: it accumulates messages sharing the same
// msg.parts.id (see internal/nodes/split's package doc for that
// convention) until parts.count of them have arrived, then emits one
// combined message and forgets the group. Messages are held in memory
// per group id; there is no timeout to flush an incomplete group (a
// documented scope cut - see docs/NODE_PALETTE_PLAN.md, Phase 4), so a
// group whose parts never all arrive stays buffered for the life of the
// flow.
package join

import (
    "fmt"
    "strings"
    "sync"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

type group struct {
    partType  string
    count     int
    separator string
    values    map[int]interface{}
    keys      map[int]string
}

// Node holds Join's in-progress groups.
type Node struct {
    mu     sync.Mutex
    groups map[string]*group
}

// ExecuteMulti accumulates input into its msg.parts group, sending the
// combined message on "output" once the group is complete, or on no port
// otherwise.
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    info, ok := input["parts"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("join: message has no \"parts\" metadata (msg.parts) - not part of a Split sequence")
    }

    id, _ := info["id"].(string)
    if id == "" {
        return nil, fmt.Errorf("join: parts.id is required")
    }
    index := intFrom(info["index"])
    count := intFrom(info["count"])
    partType, _ := info["type"].(string)

    n.mu.Lock()
    if n.groups == nil {
        n.groups = make(map[string]*group)
    }
    g := n.groups[id]
    if g == nil {
        g = &group{partType: partType, count: count, values: make(map[int]interface{}), keys: make(map[int]string)}
        n.groups[id] = g
    }
    g.values[index] = input["payload"]
    if partType == "object" {
        key, _ := info["key"].(string)
        g.keys[index] = key
    }
    if ch, ok := info["ch"].(string); ok {
        g.separator = ch
    }

    var result map[string]interface{}
    if g.count > 0 && len(g.values) >= g.count {
        result = assemble(g, input)
        delete(n.groups, id)
    }
    n.mu.Unlock()

    if result == nil {
        return map[string]map[string]interface{}{}, nil
    }
    return map[string]map[string]interface{}{"output": result}, nil
}

func assemble(g *group, sample map[string]interface{}) map[string]interface{} {
    out := cloneMap(sample)
    delete(out, "parts")

    switch g.partType {
    case "string":
        pieces := make([]string, g.count)
        for i := 0; i < g.count; i++ {
            pieces[i] = fmt.Sprintf("%v", g.values[i])
        }
        out["payload"] = strings.Join(pieces, g.separator)
    case "object":
        obj := make(map[string]interface{}, g.count)
        for i := 0; i < g.count; i++ {
            obj[g.keys[i]] = g.values[i]
        }
        out["payload"] = obj
        delete(out, "topic")
    default: // "array" and anything else defaults to array reassembly
        arr := make([]interface{}, g.count)
        for i := 0; i < g.count; i++ {
            arr[i] = g.values[i]
        }
        out["payload"] = arr
    }
    return out
}

func intFrom(v interface{}) int {
    switch n := v.(type) {
    case float64:
        return int(n)
    case int:
        return n
    case int64:
        return int(n)
    default:
        return 0
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

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} { return map[string]interface{}{} }

func (n *Node) SetConfig(config map[string]interface{}) error { return nil }

func cloneMap(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("join", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "join",
        Type:        "join",
        Name:        "Join",
        Description: "Reassembles messages sharing msg.parts (from a Split node) back into one array, string, or object",
        Category:    "flow-control",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "One message per part", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "The reassembled message, once every part has arrived", Required: true},
        },
        ConfigSchema: registry.Schema{},
        Icon:         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M4 4h7v7H4zm9 0h7v7h-7zM4 13h7v7H4zm9 0h7v7h-7z" opacity=".4"/><path d="M11 11h2v2h-2z"/></svg>`,
        Tags:         []string{"flow-control", "join", "sequence"},
    })
    if err != nil {
        panic(err)
    }
}
