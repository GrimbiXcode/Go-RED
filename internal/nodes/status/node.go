// Package status provides the Status node implementation.
//
// Status has no input port - like Node-RED, it never runs via a normal
// wire. Instead it subscribes to its flow's status events (see
// registry.NodeRuntime.OnStatus) and emits a message describing the change
// whenever a node in its scope reports one via
// registry.NodeRuntime.ReportStatus.
package status

import (
    "context"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Node holds the set of node IDs to watch. An empty Scope watches every
// node in the flow.
type Node struct {
    Scope []string
}

// Execute returns input unchanged; only runs if something is mistakenly
// wired directly into Status, which Node-RED's editor doesn't allow either.
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

// EagerlyReady marks Status for the engine's ready-wait (see
// registry.EagerlyReadyNode) - Deploy won't return until Start below has
// actually subscribed, so a message injected right after deploying can't
// race ahead of it and have its status report silently missed.
func (n *Node) EagerlyReady() {}

// Start subscribes to the flow's status events for the lifetime of ctx,
// emitting a message for every status report that falls within this Status
// node's scope.
func (n *Node) Start(ctx context.Context, emit func(map[string]interface{})) error {
    rt, ok := registry.RuntimeFromContext(ctx)
    if !ok {
        registry.SignalReady(ctx)
        <-ctx.Done()
        return nil
    }

    rt.OnStatus(func(evt registry.NodeStatusEvent) {
        if evt.NodeID == rt.NodeID || !n.inScope(evt.NodeID) {
            return
        }

        emit(map[string]interface{}{
            "status": map[string]interface{}{
                "text":  evt.Detail,
                "value": evt.Status,
            },
            "source": map[string]interface{}{
                "id":   evt.NodeID,
                "type": evt.NodeType,
            },
        })
    })

    registry.SignalReady(ctx)
    <-ctx.Done()
    return nil
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
    err := reg.RegisterFactory("status", func() registry.NodeExecutor {
        return &Node{}
    }, registry.NodeMetadata{
        ID:          "status",
        Type:        "status",
        Name:        "Status",
        Description: "Fires when another node in this flow reports a status change",
        Category:    "flow-control",
        Inputs:      []registry.Port{},
        Outputs: []registry.Port{
            {ID: "output", Name: "Output", Description: "Message describing the status change", Required: true},
        },
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "scope": {Type: "array", Description: "Node IDs to watch; empty watches every node in this flow", Default: []interface{}{}},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#4CAF50"><circle cx="12" cy="12" r="8"/></svg>`,
        Tags: []string{"flow-control", "status"},
    })
    if err != nil {
        panic(err)
    }
}
