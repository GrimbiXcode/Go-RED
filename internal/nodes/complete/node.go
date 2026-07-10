// Package complete provides the Complete node implementation.
//
// Complete has no input port - like Node-RED, it never runs via a normal
// wire. Instead it subscribes to its flow's completion events (see
// registry.NodeRuntime.OnComplete, published by the engine after every
// successful Execute/ExecuteMulti) and emits a copy of the finished node's
// output whenever a node in its scope finishes successfully.
//
// Feedback-loop warning: unlike Catch/Status (which react to error/status
// events that a normal successful message never produces), Complete reacts
// to the exact same "node finished successfully" event every node
// generates. If Complete's own output is wired - directly or transitively -
// into a node inside its own Scope, each message it emits will eventually
// trigger Complete again, forever. An empty Scope ("watch every node in
// this flow") is especially risky for this reason: scope Complete to the
// specific node(s) you want to observe, not to nodes downstream of Complete
// itself.
package complete

import (
    "context"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds the set of node IDs to watch. An empty Scope watches every
// node in the flow - see the feedback-loop warning in the package doc
// before using that.
type Node struct {
    Scope []string
}

// Execute returns input unchanged; only runs if something is mistakenly
// wired directly into Complete, which Node-RED's editor doesn't allow
// either.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return input, nil
}

func (n *Node) Validate() error { return nil }

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{"scope": scopeToConfig(n.Scope)}
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.Scope = scopeFromConfig(config["scope"])
    return nil
}

func (n *Node) inScope(nodeID string) bool {
    if len(n.Scope) == 0 {
        return true
    }
    for _, id := range n.Scope {
        if id == nodeID {
            return true
        }
    }
    return false
}

// EagerlyReady marks Complete for the engine's ready-wait (see
// registry.EagerlyReadyNode) - Deploy won't return until Start below has
// actually subscribed, so a message injected right after deploying can't
// race ahead of it and have its completion silently missed.
func (n *Node) EagerlyReady() {}

// Start subscribes to the flow's complete events for the lifetime of ctx,
// emitting a copy of the finished node's output for every completion that
// falls within this Complete node's scope.
func (n *Node) Start(ctx context.Context, emit func(map[string]interface{})) error {
    rt, ok := registry.RuntimeFromContext(ctx)
    if !ok {
        registry.SignalReady(ctx)
        <-ctx.Done()
        return nil
    }

    rt.OnComplete(func(evt registry.NodeCompleteEvent) {
        if evt.NodeID == rt.NodeID || !n.inScope(evt.NodeID) {
            return
        }
        emit(clonePayload(evt.Payload))
    })

    registry.SignalReady(ctx)
    <-ctx.Done()
    return nil
}

func clonePayload(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src))
    for k, v := range src {
        dst[k] = v
    }
    return dst
}

func scopeToConfig(scope []string) []interface{} {
    out := make([]interface{}, len(scope))
    for i, s := range scope {
        out[i] = s
    }
    return out
}

func scopeFromConfig(raw interface{}) []string {
    items, ok := raw.([]interface{})
    if !ok {
        return nil
    }
    var scope []string
    for _, v := range items {
        if s, ok := v.(string); ok {
            scope = append(scope, s)
        }
    }
    return scope
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("complete", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "complete",
        Type:        "complete",
        Name:        "Complete",
        Description: "Fires with the resulting message when another node in this flow finishes successfully",
        Category:    "flow-control",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "The completed node's output message", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "scope": {Type: "array", Description: "Node IDs to watch; empty watches every node in this flow", Default: []interface{}{}},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#2196F3"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>`,
        Tags: []string{"flow-control", "complete"},
    })
    if err != nil {
        panic(err)
    }
}
