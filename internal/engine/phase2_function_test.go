package engine

import (
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/change"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/switchnode"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This file demonstrates the Phase 2 milestone from docs/NODE_PALETTE_PLAN.md
// ("Switch routet korrekt auf mehrere Outputs, Change liest/schreibt
// Flow-Context") against the real switch/change node implementations - not
// mocks - deployed through a real FlowEngine, using an isolated registry
// (per internal/registry/AGENTS.md's testing guidance).

func TestPhase2_SwitchAndChangeEndToEnd(t *testing.T) {
    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
    require.NoError(t, reg.RegisterFactory("switch", func() registry.NodeExecutor { return &switchnode.Node{} }, registry.NodeMetadata{Type: "switch"}))
    require.NoError(t, reg.RegisterFactory("change", func() registry.NodeExecutor { return &change.Node{} }, registry.NodeMetadata{Type: "change"}))

    lowSink := &captureSinkNode{}
    highSink := &captureSinkNode{}
    require.NoError(t, reg.RegisterFactory("low-sink", func() registry.NodeExecutor { return lowSink }, registry.NodeMetadata{Type: "low-sink"}))
    require.NoError(t, reg.RegisterFactory("high-sink", func() registry.NodeExecutor { return highSink }, registry.NodeMetadata{Type: "high-sink"}))

    e := newTestEngine(reg)
    defer e.Stop()

    flow := NewFlow("phase2-flow", "Phase 2 Function")
    flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
    // Change: count visits in flow context, then set msg.payload to the
    // running count read back from flow context - proves both write and
    // read against real flow context through the deployed engine.
    flow.Nodes["counter"] = &Node{ID: "counter", Type: "change", Config: map[string]interface{}{
        "rules": []interface{}{
            map[string]interface{}{
                "action": "set",
                "target": map[string]interface{}{"type": "flow", "path": "visits"},
                "value":  map[string]interface{}{"type": "num", "value": "1"},
            },
            map[string]interface{}{
                "action": "set",
                "target": map[string]interface{}{"type": "msg", "path": "payload"},
                "value":  map[string]interface{}{"type": "flow", "value": "visits"},
            },
        },
    }}
    // Switch: routes >= 1 to "high" output (rule index 0), else "low"
    // (rule index 1, "else").
    flow.Nodes["switch"] = &Node{ID: "switch", Type: "switch", Config: map[string]interface{}{
        "property": map[string]interface{}{"type": "msg", "path": "payload"},
        "checkAll": false,
        "rules": []interface{}{
            map[string]interface{}{"operator": "gte", "value": map[string]interface{}{"type": "num", "value": "1"}},
            map[string]interface{}{"operator": "else"},
        },
    }}
    flow.Nodes["lowSink"] = &Node{ID: "lowSink", Type: "low-sink"}
    flow.Nodes["highSink"] = &Node{ID: "highSink", Type: "high-sink"}

    flow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "up", TargetNode: "counter"},
        {ID: "c2", SourceNode: "counter", TargetNode: "switch"},
        {ID: "c3", SourceNode: "switch", SourcePort: "0", TargetNode: "highSink"},
        {ID: "c4", SourceNode: "switch", SourcePort: "1", TargetNode: "lowSink"},
    }

    require.NoError(t, e.Deploy(flow))
    defer e.Undeploy(flow.ID)

    require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{}))

    require.Eventually(t, func() bool { return highSink.receivedCount() == 1 }, time.Second, 5*time.Millisecond)
    time.Sleep(20 * time.Millisecond) // give a stray delivery to lowSink a chance to show up
    assert.Equal(t, 0, lowSink.receivedCount(), "switch should route only to the matching port")
    assert.Equal(t, float64(1), highSink.last()["payload"])

    flowCtx, _, err := e.GetFlowContext(flow.ID)
    require.NoError(t, err)
    visits, ok := flowCtx.Get("visits")
    require.True(t, ok)
    assert.Equal(t, float64(1), visits, "change wrote to real flow context, readable outside the node too")
}
