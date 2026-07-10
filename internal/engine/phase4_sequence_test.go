package engine

import (
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/join"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/split"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This file demonstrates the Phase 4 milestone from docs/NODE_PALETTE_PLAN.md
// ("Array -> Split -> Join ergibt wieder das Original-Array in einem
// Integrationstest") against the real split/join node implementations - not
// mocks - deployed through a real FlowEngine, using an isolated registry
// (per internal/registry/AGENTS.md's testing guidance).

func TestPhase4_SplitJoinRoundTrip(t *testing.T) {
    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
    require.NoError(t, reg.RegisterFactory("split", func() registry.NodeExecutor {
        return &split.Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    }, registry.NodeMetadata{Type: "split"}))
    require.NoError(t, reg.RegisterFactory("join", func() registry.NodeExecutor { return &join.Node{} }, registry.NodeMetadata{Type: "join"}))

    resultSink := &captureSinkNode{}
    require.NoError(t, reg.RegisterFactory("result-sink", func() registry.NodeExecutor { return resultSink }, registry.NodeMetadata{Type: "result-sink"}))

    e := newTestEngine(reg)
    defer e.Stop()

    flow := NewFlow("phase4-split-join", "Phase 4 Split/Join")
    flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
    flow.Nodes["splitter"] = &Node{ID: "splitter", Type: "split"}
    flow.Nodes["joiner"] = &Node{ID: "joiner", Type: "join"}
    flow.Nodes["resultSink"] = &Node{ID: "resultSink", Type: "result-sink"}
    flow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "up", TargetNode: "splitter"},
        {ID: "c2", SourceNode: "splitter", TargetNode: "joiner"},
        {ID: "c3", SourceNode: "joiner", TargetNode: "resultSink"},
    }

    require.NoError(t, e.Deploy(flow))
    defer e.Undeploy(flow.ID)

    original := []interface{}{"a", "b", "c", "d"}
    require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{"payload": original}))

    require.Eventually(t, func() bool { return resultSink.receivedCount() == 1 }, time.Second, 5*time.Millisecond)
    assert.Equal(t, original, resultSink.last()["payload"])

    time.Sleep(20 * time.Millisecond) // give any stray extra delivery a chance to show up
    assert.Equal(t, 1, resultSink.receivedCount(), "join should emit exactly once, when the group completes")
}
