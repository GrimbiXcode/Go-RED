// Package sortnode provides the Sort node implementation (Node-RED type ID
// "sort" - named sortnode here to avoid shadowing the imported stdlib sort
// package within this file).
//
// Sort buffers a full msg.parts sequence (see internal/nodes/split's
// package doc for that convention), the same way internal/nodes/join does,
// but instead of combining the messages into one, it re-emits all of them
// once the group is complete, reordered by a configured property and with
// parts.index corrected to match the new order (so a Join downstream still
// works). Like Join, there is no timeout to flush an incomplete group.
//
// Since one completed group becomes N re-ordered output messages on the
// same port - the same fan-out shape internal/nodes/split has - only the
// first is returned as ExecuteMulti's normal result; the rest go through
// registry.NodeRuntime.SubmitToNode(rt.NodeID, ...).
package sortnode

import (
    "context"
    "fmt"
    "sort"
    "strconv"
    "sync"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

type group struct {
    count int
    items map[int]map[string]interface{}
}

// Node holds a Sort node's configuration and in-progress groups.
type Node struct {
    // Property is compared across messages to order them; default
    // msg.payload.
    Property   typedvalue.PropertyRef
    Descending bool

    mu     sync.Mutex
    groups map[string]*group
}

// ExecuteMulti buffers input into its msg.parts group; once complete, all N
// messages are re-emitted in sorted order (first via the normal return,
// the rest via SubmitToNode).
func (n *Node) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    info, ok := input["parts"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("sort: message has no \"parts\" metadata (msg.parts) - not part of a Split sequence")
    }
    id, _ := info["id"].(string)
    if id == "" {
        return nil, fmt.Errorf("sort: parts.id is required")
    }
    index := intFrom(info["index"])
    count := intFrom(info["count"])

    n.mu.Lock()
    if n.groups == nil {
        n.groups = make(map[string]*group)
    }
    g := n.groups[id]
    if g == nil {
        g = &group{count: count, items: make(map[int]map[string]interface{})}
        n.groups[id] = g
    }
    g.items[index] = cloneMap(input)

    var ordered []map[string]interface{}
    if g.count > 0 && len(g.items) >= g.count {
        ordered = make([]map[string]interface{}, g.count)
        for i := 0; i < g.count; i++ {
            ordered[i] = g.items[i]
        }
        delete(n.groups, id)
    }
    n.mu.Unlock()

    if ordered == nil {
        return map[string]map[string]interface{}{}, nil
    }

    flowCtx, globalCtx := contexts(ctx)
    n.sortMessages(ordered, flowCtx, globalCtx)
    for i, msg := range ordered {
        updatePartsIndex(msg, i)
    }

    if rt, ok := runtimeFrom(ctx); ok {
        for _, msg := range ordered[1:] {
            rt.SubmitToNode(rt.NodeID, msg)
        }
    }
    return map[string]map[string]interface{}{"output": ordered[0]}, nil
}

func (n *Node) sortMessages(msgs []map[string]interface{}, flowCtx, globalCtx *registry.ContextStore) {
    valueOf := func(msg map[string]interface{}) interface{} {
        r := typedvalue.PropertyResolver{Message: msg}
        if flowCtx != nil {
            r.FlowContext = flowCtx
        }
        if globalCtx != nil {
            r.GlobalContext = globalCtx
        }
        v, _ := n.Property.Get(r)
        return v
    }

    sort.SliceStable(msgs, func(i, j int) bool {
        less := compare(valueOf(msgs[i]), valueOf(msgs[j])) < 0
        if n.Descending {
            return !less
        }
        return less
    })
}

// updatePartsIndex returns msg with a freshly-copied "parts" map (not a
// mutation of the original) reflecting msg's new position, since the
// original parts map may be one Split allocated once per message and
// shouldn't be mutated in place.
func updatePartsIndex(msg map[string]interface{}, index int) {
    info, ok := msg["parts"].(map[string]interface{})
    if !ok {
        return
    }
    updated := make(map[string]interface{}, len(info))
    for k, v := range info {
        updated[k] = v
    }
    updated["index"] = float64(index)
    msg["parts"] = updated
}

// compare returns -1/0/1, comparing numerically if both sides parse as a
// number, otherwise lexicographically as strings.
func compare(a, b interface{}) int {
    if af, aok := toFloat(a); aok {
        if bf, bok := toFloat(b); bok {
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
    as, bs := toStr(a), toStr(b)
    switch {
    case as < bs:
        return -1
    case as > bs:
        return 1
    default:
        return 0
    }
}

func toFloat(v interface{}) (float64, bool) {
    switch n := v.(type) {
    case float64:
        return n, true
    case int:
        return float64(n), true
    case int64:
        return float64(n), true
    case string:
        f, err := strconv.ParseFloat(n, 64)
        return f, err == nil
    default:
        return 0, false
    }
}

func toStr(v interface{}) string {
    if s, ok := v.(string); ok {
        return s
    }
    return fmt.Sprintf("%v", v)
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

func contexts(ctx interface{}) (*registry.ContextStore, *registry.ContextStore) {
    rt, ok := runtimeFrom(ctx)
    if !ok {
        return nil, nil
    }
    return rt.FlowContext, rt.GlobalContext
}

func runtimeFrom(ctx interface{}) (*registry.NodeRuntime, bool) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, false
    }
    return registry.RuntimeFromContext(c)
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

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "property":   propertyRefToConfig(n.Property),
        "descending": n.Descending,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Property = parsePropertyRef(config["property"], typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"})
    if d, ok := config["descending"].(bool); ok {
        n.Descending = d
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

func parsePropertyRef(raw interface{}, def typedvalue.PropertyRef) typedvalue.PropertyRef {
    m, ok := raw.(map[string]interface{})
    if !ok {
        return def
    }
    t, _ := m["type"].(string)
    p, _ := m["path"].(string)
    if t == "" {
        return def
    }
    return typedvalue.PropertyRef{Type: typedvalue.Type(t), Path: p}
}

func propertyRefToConfig(ref typedvalue.PropertyRef) map[string]interface{} {
    return map[string]interface{}{"type": string(ref.Type), "path": ref.Path}
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("sort", func() registry.NodeExecutor {
        return &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    }, registry.NodeMetadata{
        ID:          "sort",
        Type:        "sort",
        Name:        "Sort",
        Description: "Buffers a full msg.parts sequence (from a Split node) and re-emits it ordered by a property",
        Category:    "flow-control",
        Inputs: []registry.Port{
            {ID: "input", Name: "Input", Description: "One message per part", Required: true},
        },
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "The same messages, re-ordered, once every part has arrived", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "property":   {Type: "object", Description: `Property to sort by, e.g. {"type":"msg","path":"payload"}`, Default: map[string]interface{}{"type": "msg", "path": "payload"}},
                "descending": {Type: "boolean", Description: "Sort largest/last first", Default: false},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M3 6h12v2H3zm0 5h9v2H3zm0 5h6v2H3zm14-9l4 4h-3v6h-2v-6h-3z"/></svg>`,
        Tags: []string{"flow-control", "sort", "sequence"},
    })
    if err != nil {
        panic(err)
    }
}
