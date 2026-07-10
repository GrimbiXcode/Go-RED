package engine

import (
    "context"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/require"
)

// This file guards against the regression that broke CI:
// TestPhase1_FlowControlNodesEndToEnd (phase1_flow_control_test.go) failed
// intermittently because Deploy returned - and the test's InjectMessage
// call fired - before catch/status/complete's own Start goroutine had
// actually subscribed to the flow's EventBus. The fix
// (registry.EagerlyReadyNode/WithReady/SignalReady, wired into
// startEmittingNodes) is deterministic, not timing-dependent, so these
// tests prove it with an explicit gate instead of hoping to reproduce the
// original race by chance.

// gatedReadyNode is a registry.EagerlyReadyNode whose Start blocks on gate
// before calling registry.SignalReady, letting a test control exactly when
// "setup" completes relative to when it asserts on Deploy's return.
type gatedReadyNode struct {
    passthroughNode
    gate chan struct{}
}

func (n *gatedReadyNode) EagerlyReady() {}

func (n *gatedReadyNode) Start(ctx context.Context, emit func(map[string]interface{})) error {
    <-n.gate
    registry.SignalReady(ctx)
    <-ctx.Done()
    return nil
}

func TestDeploy_WaitsForEagerlyReadyNode(t *testing.T) {
    reg := registry.NewNodeRegistry()
    node := &gatedReadyNode{gate: make(chan struct{})}
    require.NoError(t, reg.RegisterFactory("gated", func() registry.NodeExecutor { return node }, registry.NodeMetadata{Type: "gated"}))

    e := newTestEngine(reg)
    defer e.Stop()

    flow := NewFlow("gated-flow", "Gated")
    flow.Nodes["gated"] = &Node{ID: "gated", Type: "gated"}

    deployDone := make(chan error, 1)
    go func() { deployDone <- e.Deploy(flow) }()

    // Deploy must not return while the node is deliberately withheld from
    // calling SignalReady - proves the wait is real, not a no-op.
    select {
    case <-deployDone:
        t.Fatal("Deploy returned before the EagerlyReadyNode signaled ready")
    case <-time.After(150 * time.Millisecond):
    }

    close(node.gate) // let Start proceed to SignalReady

    select {
    case err := <-deployDone:
        require.NoError(t, err)
    case <-time.After(2 * time.Second):
        t.Fatal("Deploy never returned after the node signaled ready")
    }

    require.NoError(t, e.Undeploy(flow.ID))
}

// neverReadyNode implements registry.EagerlyReadyNode but never calls
// SignalReady - a buggy node, or one that should not have implemented the
// marker. Deploy must not hang forever on it.
type neverReadyNode struct {
    passthroughNode
}

func (n *neverReadyNode) EagerlyReady() {}

func (n *neverReadyNode) Start(ctx context.Context, emit func(map[string]interface{})) error {
    <-ctx.Done()
    return nil
}

func TestDeploy_ProceedsAfterTimeoutWhenNeverSignaled(t *testing.T) {
    reg := registry.NewNodeRegistry()
    require.NoError(t, reg.RegisterFactory("never-ready", func() registry.NodeExecutor { return &neverReadyNode{} }, registry.NodeMetadata{Type: "never-ready"}))

    e := newTestEngine(reg)
    defer e.Stop()

    flow := NewFlow("never-ready-flow", "Never Ready")
    flow.Nodes["node"] = &Node{ID: "node", Type: "never-ready"}

    deployDone := make(chan error, 1)
    go func() { deployDone <- e.Deploy(flow) }()

    select {
    case err := <-deployDone:
        require.NoError(t, err)
    case <-time.After(eagerReadyTimeout + time.Second):
        t.Fatal("Deploy hung well past eagerReadyTimeout for a node that never signals ready")
    }

    require.NoError(t, e.Undeploy(flow.ID))
}
