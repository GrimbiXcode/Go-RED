package engine

import (
    "context"
    "errors"
    "sync"
    "testing"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This file exercises the Phase 0 engine additions from
// docs/NODE_PALETTE_PLAN.md: multi-output routing by port, node
// Close()/lifecycle, EmittingNode, and the error/status EventBus + flow-
// /global ContextStore reachable via NodeRuntime. It uses isolated
// registries (per internal/registry/AGENTS.md's testing guidance) with
// small hand-rolled mock nodes rather than the real inject/debug/function
// nodes, since those self-register only into the global registry.

// passthroughNode returns its input unchanged. Used as a simple upstream
// node to route a message into the node actually under test.
type passthroughNode struct{}

func (passthroughNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return input, nil
}
func (passthroughNode) Validate() error                             { return nil }
func (passthroughNode) GetConfig() map[string]interface{}           { return nil }
func (passthroughNode) SetConfig(config map[string]interface{}) error { return nil }

// captureSinkNode records every payload it receives. Tests register the
// *same* pointer as the factory's return value so they can inspect it after
// Deploy without reaching into engine internals.
type captureSinkNode struct {
    mu       sync.Mutex
    received []map[string]interface{}
}

func (n *captureSinkNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    n.mu.Lock()
    n.received = append(n.received, input)
    n.mu.Unlock()
    return input, nil
}
func (n *captureSinkNode) Validate() error                             { return nil }
func (n *captureSinkNode) GetConfig() map[string]interface{}           { return nil }
func (n *captureSinkNode) SetConfig(config map[string]interface{}) error { return nil }

func (n *captureSinkNode) receivedCount() int {
    n.mu.Lock()
    defer n.mu.Unlock()
    return len(n.received)
}

func (n *captureSinkNode) last() map[string]interface{} {
    n.mu.Lock()
    defer n.mu.Unlock()
    if len(n.received) == 0 {
        return nil
    }
    return n.received[len(n.received)-1]
}

// routerNode implements MultiOutputExecutor: it sends to "outputA" unless
// input["route"] == "b", in which case it sends to "outputB" instead.
type routerNode struct{ passthroughNode }

func (routerNode) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    if route, _ := input["route"].(string); route == "b" {
        return map[string]map[string]interface{}{"outputB": input}, nil
    }
    return map[string]map[string]interface{}{"outputA": input}, nil
}

func newTestEngine(reg *registry.NodeRegistry) *FlowEngine {
    e := NewFlowEngine(EngineConfig{
        WorkerPoolSize:    5,
        MessageBufferSize: 100,
        DefaultTimeout:    5 * time.Second,
    }, reg)
    e.Start()
    return e
}

func TestMultiOutputRouting(t *testing.T) {
    t.Run("routes only to the connection matching the emitted port", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        sinkA := &captureSinkNode{}
        sinkB := &captureSinkNode{}
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("router", func() registry.NodeExecutor { return routerNode{} }, registry.NodeMetadata{Type: "router"}))
        require.NoError(t, reg.RegisterFactory("sink-a", func() registry.NodeExecutor { return sinkA }, registry.NodeMetadata{Type: "sink-a"}))
        require.NoError(t, reg.RegisterFactory("sink-b", func() registry.NodeExecutor { return sinkB }, registry.NodeMetadata{Type: "sink-b"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("multi-output-flow", "Multi Output")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["router"] = &Node{ID: "router", Type: "router"}
        flow.Nodes["sinkA"] = &Node{ID: "sinkA", Type: "sink-a"}
        flow.Nodes["sinkB"] = &Node{ID: "sinkB", Type: "sink-b"}
        flow.Connections = []NodeConnection{
            {ID: "c1", SourceNode: "up", TargetNode: "router"},
            {ID: "c2", SourceNode: "router", SourcePort: "outputA", TargetNode: "sinkA"},
            {ID: "c3", SourceNode: "router", SourcePort: "outputB", TargetNode: "sinkB"},
        }

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{"route": "b"}))

        require.Eventually(t, func() bool { return sinkB.receivedCount() == 1 }, time.Second, 5*time.Millisecond)
        time.Sleep(20 * time.Millisecond) // give a stray delivery to sinkA a chance to show up
        assert.Equal(t, 0, sinkA.receivedCount(), "sinkA should not receive a message routed to outputB")
        assert.Equal(t, "b", sinkB.last()["route"])
    })

    t.Run("plain single-output nodes still fan out to every connection regardless of port", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        sinkA := &captureSinkNode{}
        sinkB := &captureSinkNode{}
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("sink-a", func() registry.NodeExecutor { return sinkA }, registry.NodeMetadata{Type: "sink-a"}))
        require.NoError(t, reg.RegisterFactory("sink-b", func() registry.NodeExecutor { return sinkB }, registry.NodeMetadata{Type: "sink-b"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("fanout-flow", "Fan Out")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["sinkA"] = &Node{ID: "sinkA", Type: "sink-a"}
        flow.Nodes["sinkB"] = &Node{ID: "sinkB", Type: "sink-b"}
        // Neither connection sets SourcePort - matches every real flow/test
        // predating per-port routing.
        flow.Connections = []NodeConnection{
            {ID: "c1", SourceNode: "up", TargetNode: "sinkA"},
            {ID: "c2", SourceNode: "up", TargetNode: "sinkB"},
        }

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{"hello": "world"}))

        require.Eventually(t, func() bool { return sinkA.receivedCount() == 1 && sinkB.receivedCount() == 1 }, time.Second, 5*time.Millisecond)
    })
}

// closeableNode records how many times Close was called.
type closeableNode struct {
    passthroughNode
    mu     sync.Mutex
    closed int
}

func (n *closeableNode) Close() error {
    n.mu.Lock()
    n.closed++
    n.mu.Unlock()
    return nil
}

func (n *closeableNode) closeCount() int {
    n.mu.Lock()
    defer n.mu.Unlock()
    return n.closed
}

func TestCloseableNodeLifecycle(t *testing.T) {
    t.Run("Close is called for every Closeable node on Undeploy", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        node := &closeableNode{}
        require.NoError(t, reg.RegisterFactory("closeable", func() registry.NodeExecutor { return node }, registry.NodeMetadata{Type: "closeable"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("closeable-flow", "Closeable")
        flow.Nodes["n1"] = &Node{ID: "n1", Type: "closeable"}

        require.NoError(t, e.Deploy(flow))
        assert.Equal(t, 0, node.closeCount())

        require.NoError(t, e.Undeploy(flow.ID))
        assert.Equal(t, 1, node.closeCount())
    })

    t.Run("closeNodeExecutors closes every Closeable executor exactly once and ignores the rest", func(t *testing.T) {
        a := &closeableNode{}
        b := &closeableNode{}
        plain := passthroughNode{}

        closeNodeExecutors(map[string]registry.NodeExecutor{
            "a":     a,
            "b":     b,
            "plain": plain,
        })

        assert.Equal(t, 1, a.closeCount())
        assert.Equal(t, 1, b.closeCount())
    })
}

// emittingNode is a Start-based source: it emits one message, signals it has
// started, then blocks until ctx is cancelled and signals it has stopped.
type emittingNode struct {
    passthroughNode
    started chan struct{}
    stopped chan struct{}
}

func (n *emittingNode) Start(ctx context.Context, emit func(map[string]interface{})) error {
    emit(map[string]interface{}{"tick": float64(1)})
    close(n.started)
    <-ctx.Done()
    close(n.stopped)
    return nil
}

func TestEmittingNode(t *testing.T) {
    t.Run("Start runs on deploy, emits are routed like normal output, Start returns on undeploy", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        source := &emittingNode{started: make(chan struct{}), stopped: make(chan struct{})}
        sink := &captureSinkNode{}
        require.NoError(t, reg.RegisterFactory("source", func() registry.NodeExecutor { return source }, registry.NodeMetadata{Type: "source"}))
        require.NoError(t, reg.RegisterFactory("sink", func() registry.NodeExecutor { return sink }, registry.NodeMetadata{Type: "sink"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("emitting-flow", "Emitting")
        flow.Nodes["source"] = &Node{ID: "source", Type: "source"}
        flow.Nodes["sink"] = &Node{ID: "sink", Type: "sink"}
        flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "source", TargetNode: "sink"}}

        require.NoError(t, e.Deploy(flow))

        select {
        case <-source.started:
        case <-time.After(time.Second):
            t.Fatal("Start was never called")
        }

        require.Eventually(t, func() bool { return sink.receivedCount() == 1 }, time.Second, 5*time.Millisecond)
        assert.Equal(t, float64(1), sink.last()["tick"])

        require.NoError(t, e.Undeploy(flow.ID))

        select {
        case <-source.stopped:
        default:
            t.Fatal("Undeploy returned before Start observed context cancellation")
        }
    })
}

// failingNode always returns an error from Execute.
type failingNode struct{ passthroughNode }

func (failingNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, errors.New("mock node failure")
}

func TestEventBus_ErrorPropagation(t *testing.T) {
    t.Run("a node execution error is published on the flow EventBus", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("failing", func() registry.NodeExecutor { return failingNode{} }, registry.NodeMetadata{Type: "failing"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("error-flow", "Error")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["bad"] = &Node{ID: "bad", Type: "failing"}
        flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "up", TargetNode: "bad"}}

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        _, bus, err := e.GetFlowContext(flow.ID)
        require.NoError(t, err)
        require.NotNil(t, bus)

        var mu sync.Mutex
        var events []registry.NodeErrorEvent
        bus.OnError(func(evt registry.NodeErrorEvent) {
            mu.Lock()
            events = append(events, evt)
            mu.Unlock()
        })

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{}))

        require.Eventually(t, func() bool {
            mu.Lock()
            defer mu.Unlock()
            return len(events) == 1
        }, time.Second, 5*time.Millisecond)

        mu.Lock()
        defer mu.Unlock()
        assert.Equal(t, "bad", events[0].NodeID)
        assert.Equal(t, "failing", events[0].NodeType)
        assert.EqualError(t, events[0].Err, "mock node failure")
    })
}

func TestEventBus_CompletePropagation(t *testing.T) {
    t.Run("a successful node execution is published on the flow EventBus", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("ok", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "ok"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("complete-flow", "Complete")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["ok"] = &Node{ID: "ok", Type: "ok"}
        flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "up", TargetNode: "ok"}}

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        _, bus, err := e.GetFlowContext(flow.ID)
        require.NoError(t, err)

        var mu sync.Mutex
        var events []registry.NodeCompleteEvent
        bus.OnComplete(func(evt registry.NodeCompleteEvent) {
            mu.Lock()
            events = append(events, evt)
            mu.Unlock()
        })

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{"hello": "world"}))

        require.Eventually(t, func() bool {
            mu.Lock()
            defer mu.Unlock()
            return len(events) == 1
        }, time.Second, 5*time.Millisecond)

        mu.Lock()
        defer mu.Unlock()
        assert.Equal(t, "ok", events[0].NodeID)
        assert.Equal(t, "ok", events[0].NodeType)
        assert.Equal(t, "world", events[0].Payload["hello"])
    })

    t.Run("a MultiOutputExecutor that sends on no port still publishes one complete event", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("silent", func() registry.NodeExecutor { return silentMultiOutputNode{} }, registry.NodeMetadata{Type: "silent"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("silent-complete-flow", "Silent Complete")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["silent"] = &Node{ID: "silent", Type: "silent"}
        flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "up", TargetNode: "silent"}}

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        _, bus, err := e.GetFlowContext(flow.ID)
        require.NoError(t, err)

        var mu sync.Mutex
        var events []registry.NodeCompleteEvent
        bus.OnComplete(func(evt registry.NodeCompleteEvent) {
            mu.Lock()
            events = append(events, evt)
            mu.Unlock()
        })

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{}))

        require.Eventually(t, func() bool {
            mu.Lock()
            defer mu.Unlock()
            return len(events) == 1
        }, time.Second, 5*time.Millisecond)
    })
}

// silentMultiOutputNode always sends on no port, like Link Out.
type silentMultiOutputNode struct{ passthroughNode }

func (silentMultiOutputNode) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    return map[string]map[string]interface{}{}, nil
}

// runtimeUsingNode reads/writes flow and global context and reports a
// status, exercising NodeRuntime end-to-end.
type runtimeUsingNode struct{ passthroughNode }

func (runtimeUsingNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, errors.New("expected context.Context")
    }
    rt, ok := registry.RuntimeFromContext(c)
    if !ok {
        return nil, errors.New("no NodeRuntime in context")
    }
    rt.FlowContext.Set("seen", true)
    rt.GlobalContext.Set("global-seen", true)
    rt.ReportStatus("ok", "ready")
    return input, nil
}

func TestNodeRuntime_ContextAndStatus(t *testing.T) {
    t.Run("Execute can reach flow/global context and report status via NodeRuntime", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("runtime-node", func() registry.NodeExecutor { return runtimeUsingNode{} }, registry.NodeMetadata{Type: "runtime-node"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("runtime-flow", "Runtime")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["rt"] = &Node{ID: "rt", Type: "runtime-node"}
        flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "up", TargetNode: "rt"}}

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        flowCtx, bus, err := e.GetFlowContext(flow.ID)
        require.NoError(t, err)

        var mu sync.Mutex
        var statuses []registry.NodeStatusEvent
        bus.OnStatus(func(evt registry.NodeStatusEvent) {
            mu.Lock()
            statuses = append(statuses, evt)
            mu.Unlock()
        })

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{}))

        require.Eventually(t, func() bool {
            _, ok := flowCtx.Get("seen")
            return ok
        }, time.Second, 5*time.Millisecond)

        seen, _ := flowCtx.Get("seen")
        assert.Equal(t, true, seen)

        globalSeen, _ := e.GlobalContext().Get("global-seen")
        assert.Equal(t, true, globalSeen)

        mu.Lock()
        defer mu.Unlock()
        require.Len(t, statuses, 1)
        assert.Equal(t, "rt", statuses[0].NodeID)
        assert.Equal(t, "ok", statuses[0].Status)
        assert.Equal(t, "ready", statuses[0].Detail)
    })
}

// linkOutLikeNode delivers its input directly to a configured node ID via
// NodeRuntime.SubmitToNode instead of a normal wire, like the future Link
// Out node (docs/NODE_PALETTE_PLAN.md, Phase 1), and sends on no port.
type linkOutLikeNode struct {
    passthroughNode
    target string
}

func (n linkOutLikeNode) ExecuteMulti(ctx interface{}, input map[string]interface{}) (map[string]map[string]interface{}, error) {
    c, ok := ctx.(context.Context)
    if !ok {
        return nil, errors.New("expected context.Context")
    }
    rt, ok := registry.RuntimeFromContext(c)
    if !ok {
        return nil, errors.New("no NodeRuntime in context")
    }
    rt.SubmitToNode(n.target, input)
    return map[string]map[string]interface{}{}, nil
}

func TestNodeRuntime_SubmitToNode(t *testing.T) {
    t.Run("delivers a message to the target's own wires, like Link Out jumping to a Link In", func(t *testing.T) {
        // SubmitToNode treats the target as having just produced payload
        // itself (mirroring Link In, which has no input port in Node-RED -
        // you can only ever jump to its output) - so it must be wired
        // *from* linkIn to sink, and the target's own Execute is never
        // called; only its downstream connections receive the message.
        reg := registry.NewNodeRegistry()
        sink := &captureSinkNode{}
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("jumper", func() registry.NodeExecutor { return linkOutLikeNode{target: "linkIn"} }, registry.NodeMetadata{Type: "jumper"}))
        require.NoError(t, reg.RegisterFactory("link-in", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "link-in"}))
        require.NoError(t, reg.RegisterFactory("sink", func() registry.NodeExecutor { return sink }, registry.NodeMetadata{Type: "sink"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("submit-to-node-flow", "Submit To Node")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["jumper"] = &Node{ID: "jumper", Type: "jumper"}
        flow.Nodes["linkIn"] = &Node{ID: "linkIn", Type: "link-in"}
        flow.Nodes["sink"] = &Node{ID: "sink", Type: "sink"}
        flow.Connections = []NodeConnection{
            {ID: "c1", SourceNode: "up", TargetNode: "jumper"},
            // No connection into "linkIn" - delivery happens via
            // SubmitToNode, not a wire. Only its outgoing wire to "sink"
            // is a normal connection.
            {ID: "c2", SourceNode: "linkIn", TargetNode: "sink"},
        }

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{"via": "jump"}))

        require.Eventually(t, func() bool { return sink.receivedCount() == 1 }, time.Second, 5*time.Millisecond)
        assert.Equal(t, "jump", sink.last()["via"])
    })

    t.Run("targeting a node ID that does not exist in the flow is a silent no-op", func(t *testing.T) {
        reg := registry.NewNodeRegistry()
        require.NoError(t, reg.RegisterFactory("up", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "up"}))
        require.NoError(t, reg.RegisterFactory("jumper", func() registry.NodeExecutor { return linkOutLikeNode{target: "does-not-exist"} }, registry.NodeMetadata{Type: "jumper"}))

        e := newTestEngine(reg)
        defer e.Stop()

        flow := NewFlow("submit-to-missing-node-flow", "Submit To Missing Node")
        flow.Nodes["up"] = &Node{ID: "up", Type: "up"}
        flow.Nodes["jumper"] = &Node{ID: "jumper", Type: "jumper"}
        flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "up", TargetNode: "jumper"}}

        require.NoError(t, e.Deploy(flow))
        defer e.Undeploy(flow.ID)

        require.NoError(t, e.InjectMessage(flow.ID, "up", map[string]interface{}{}))
        time.Sleep(20 * time.Millisecond) // just needs to not panic/hang
    })
}
