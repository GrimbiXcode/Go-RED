// Package catch provides the Catch node implementation.
//
// Catch has no input port - like Node-RED, it never runs via a normal wire.
// Instead it subscribes to its flow's error events (see
// registry.NodeRuntime.OnError) and emits a copy of the failing message,
// augmented with an "error" field, on its single output whenever a node in
// its scope fails.
package catch

import (
    "context"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds the set of node IDs to catch errors from. An empty Scope
// catches every node in the flow.
type Node struct {
    Scope []string
}

// Execute returns input unchanged; only runs if something is mistakenly
// wired directly into Catch, which Node-RED's editor doesn't allow either.
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

// EagerlyReady marks Catch for the engine's ready-wait (see
// registry.EagerlyReadyNode) - Deploy won't return until Start below has
// actually subscribed, so a message injected right after deploying can't
// race ahead of it and have its error silently missed.
func (n *Node) EagerlyReady() {}

// Start subscribes to the flow's error events for the lifetime of ctx,
// emitting an error-augmented copy of the failing message for every error
// that falls within this Catch node's scope.
func (n *Node) Start(ctx context.Context, emit func(map[string]interface{})) error {
    rt, ok := registry.RuntimeFromContext(ctx)
    if !ok {
        registry.SignalReady(ctx)
        <-ctx.Done()
        return nil
    }

    rt.OnError(func(evt registry.NodeErrorEvent) {
        if evt.NodeID == rt.NodeID || !n.inScope(evt.NodeID) {
            return
        }

        payload := clonePayload(evt.Payload)
        payload["error"] = map[string]interface{}{
            "message": evt.Err.Error(),
            "source": map[string]interface{}{
                "id":   evt.NodeID,
                "type": evt.NodeType,
            },
        }
        emit(payload)
    })

    registry.SignalReady(ctx)
    <-ctx.Done()
    return nil
}

func clonePayload(src map[string]interface{}) map[string]interface{} {
    dst := make(map[string]interface{}, len(src)+1)
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
    err := reg.RegisterFactory("catch", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "catch",
        Type:        "catch",
        Name:        "Catch",
        Description: "Fires with an error-augmented message when another node in this flow fails",
        Category:    "flow-control",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message with an added error field", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "scope": {Type: "array", Description: "Node IDs to catch errors from; empty catches every node in this flow", Default: []interface{}{}},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#F44336"><path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"/></svg>`,
        Tags: []string{"flow-control", "catch", "error"},
    })
    if err != nil {
        panic(err)
    }
}
