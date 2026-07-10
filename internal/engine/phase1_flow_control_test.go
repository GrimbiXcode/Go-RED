package engine

import (
    "context"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/nodes/catch"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/comment"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/complete"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/junction"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/linkin"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/linkout"
    "github.com/GrimbiXcode/Go-RED/internal/nodes/status"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This file demonstrates the Phase 1 milestone from docs/NODE_PALETTE_PLAN.md
// ("Fehlerbehandlung (catch) und Status-Propagation end-to-end in einem
// Test-Flow nachweisbar") against the real catch/status/complete/junction/
// comment/link-in/link-out node implementations - not mocks - deployed
// through a real FlowEngine, using an isolated registry (per
// internal/registry/AGENTS.md's testing guidance).

// reportingNode reports a status via NodeRuntime and passes its input
// through unchanged, standing in for a real node like a future "mqtt in"
// reporting connection status.
type reportingNode struct{ passthroughNode }

func (reportingNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    if c, ok := ctx.(context.Context); ok {
        if rt, ok := registry.RuntimeFromContext(c); ok {
            rt.ReportStatus("connected", "broker.local:1883")
        }
    }
    return input, nil
}

func TestPhase1_FlowControlNodesEndToEnd(t *testing.T) {
    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("trigger", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "trigger"}))
    require.NoError(t, reg.RegisterFactory("reporter", func() registry.NodeExecutor { return reportingNode{} }, registry.NodeMetadata{Type: "reporter"}))
    require.NoError(t, reg.RegisterFactory("failer", func() registry.NodeExecutor { return failingNode{} }, registry.NodeMetadata{Type: "failer"}))
    require.NoError(t, reg.RegisterFactory("junction", func() registry.NodeExecutor { return &junction.Node{} }, registry.NodeMetadata{Type: "junction"}))
    require.NoError(t, reg.RegisterFactory("comment", func() registry.NodeExecutor { return &comment.Node{} }, registry.NodeMetadata{Type: "comment"}))
    require.NoError(t, reg.RegisterFactory("catch", func() registry.NodeExecutor { return &catch.Node{} }, registry.NodeMetadata{Type: "catch"}))
    require.NoError(t, reg.RegisterFactory("status", func() registry.NodeExecutor { return &status.Node{} }, registry.NodeMetadata{Type: "status"}))
    require.NoError(t, reg.RegisterFactory("complete", func() registry.NodeExecutor { return &complete.Node{} }, registry.NodeMetadata{Type: "complete"}))
    require.NoError(t, reg.RegisterFactory("link in", func() registry.NodeExecutor { return &linkin.Node{} }, registry.NodeMetadata{Type: "link in"}))
    require.NoError(t, reg.RegisterFactory("link out", func() registry.NodeExecutor { return &linkout.Node{} }, registry.NodeMetadata{Type: "link out"}))

    catchSink := &captureSinkNode{}
    statusSink := &captureSinkNode{}
    completeSink := &captureSinkNode{}
    linkSink := &captureSinkNode{}
    require.NoError(t, reg.RegisterFactory("catch-sink", func() registry.NodeExecutor { return catchSink }, registry.NodeMetadata{Type: "catch-sink"}))
    require.NoError(t, reg.RegisterFactory("status-sink", func() registry.NodeExecutor { return statusSink }, registry.NodeMetadata{Type: "status-sink"}))
    require.NoError(t, reg.RegisterFactory("complete-sink", func() registry.NodeExecutor { return completeSink }, registry.NodeMetadata{Type: "complete-sink"}))
    require.NoError(t, reg.RegisterFactory("link-sink", func() registry.NodeExecutor { return linkSink }, registry.NodeMetadata{Type: "link-sink"}))

    e := newTestEngine(reg)
    defer e.Stop()

    flow := NewFlow("phase1-flow-control", "Phase 1 Flow Control")
    flow.Nodes["trigger"] = &Node{ID: "trigger", Type: "junction"}
    flow.Nodes["note"] = &Node{ID: "note", Type: "comment", Config: map[string]interface{}{"text": "demonstrates Phase 1 flow-control nodes"}}
    flow.Nodes["reporter"] = &Node{ID: "reporter", Type: "reporter"}
    flow.Nodes["failer"] = &Node{ID: "failer", Type: "failer"}
    // Scoped to the one node each should watch, not left empty ("catch
    // all") - since each of these is itself wired to a sink whose own
    // completion would otherwise re-trigger it, an empty scope here would
    // create a feedback loop (sink succeeds -> complete event -> Complete
    // node fires -> sink succeeds again -> ...). See the scope doc comment
    // on registry.NodeCompleteEvent-consuming nodes.
    flow.Nodes["catchNode"] = &Node{ID: "catchNode", Type: "catch", Config: map[string]interface{}{"scope": []interface{}{"failer"}}}
    flow.Nodes["catchSink"] = &Node{ID: "catchSink", Type: "catch-sink"}
    flow.Nodes["statusNode"] = &Node{ID: "statusNode", Type: "status", Config: map[string]interface{}{"scope": []interface{}{"reporter"}}}
    flow.Nodes["statusSink"] = &Node{ID: "statusSink", Type: "status-sink"}
    flow.Nodes["completeNode"] = &Node{ID: "completeNode", Type: "complete", Config: map[string]interface{}{"scope": []interface{}{"reporter"}}}
    flow.Nodes["completeSink"] = &Node{ID: "completeSink", Type: "complete-sink"}
    flow.Nodes["linkOut"] = &Node{ID: "linkOut", Type: "link out", Config: map[string]interface{}{"links": []interface{}{"linkIn"}}}
    flow.Nodes["linkIn"] = &Node{ID: "linkIn", Type: "link in"}
    flow.Nodes["linkSink"] = &Node{ID: "linkSink", Type: "link-sink"}
    // "note" is deliberately not wired to anything, matching how a real
    // Comment node has no ports.
    flow.Connections = []NodeConnection{
        {ID: "c1", SourceNode: "trigger", TargetNode: "reporter"},
        {ID: "c2", SourceNode: "trigger", TargetNode: "failer"},
        {ID: "c3", SourceNode: "trigger", TargetNode: "linkOut"},
        {ID: "c4", SourceNode: "catchNode", TargetNode: "catchSink"},
        {ID: "c5", SourceNode: "statusNode", TargetNode: "statusSink"},
        {ID: "c6", SourceNode: "completeNode", TargetNode: "completeSink"},
        {ID: "c7", SourceNode: "linkIn", TargetNode: "linkSink"},
    }

    require.NoError(t, e.Deploy(flow))
    defer e.Undeploy(flow.ID)

    require.NoError(t, e.InjectMessage(flow.ID, "trigger", map[string]interface{}{"run": "1"}))

    require.Eventually(t, func() bool {
        return catchSink.receivedCount() >= 1 &&
            statusSink.receivedCount() >= 1 &&
            completeSink.receivedCount() >= 1 &&
            linkSink.receivedCount() >= 1
    }, 2*time.Second, 10*time.Millisecond)

    t.Run("catch received the failing node's error", func(t *testing.T) {
        errInfo, ok := catchSink.last()["error"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "mock node failure", errInfo["message"])
        source, ok := errInfo["source"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "failer", source["id"])
    })

    t.Run("status received the reporter's status change", func(t *testing.T) {
        st, ok := statusSink.last()["status"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, "connected", st["value"])
        assert.Equal(t, "broker.local:1883", st["text"])
    })

    t.Run("complete received a successful node's output", func(t *testing.T) {
        assert.Equal(t, "1", completeSink.last()["run"])
    })

    t.Run("link out delivered to link in without a drawn wire, which then reached its own wire", func(t *testing.T) {
        assert.Equal(t, "1", linkSink.last()["run"])
    })
}
