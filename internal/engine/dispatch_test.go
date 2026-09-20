package engine

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingNode holds every execution until its context is cancelled or the
// test releases it, and tracks how many executions run at the same time.
type blockingNode struct {
	release chan struct{}
	running atomic.Int64
	peak    atomic.Int64
	started chan struct{}
}

func newBlockingNode() *blockingNode {
	return &blockingNode{release: make(chan struct{}), started: make(chan struct{}, 1024)}
}

func (n *blockingNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	now := n.running.Add(1)
	for {
		peak := n.peak.Load()
		if now <= peak || n.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	defer n.running.Add(-1)
	n.started <- struct{}{}

	c := ctx.(context.Context)
	select {
	case <-n.release:
		return input, nil
	case <-c.Done():
		return nil, c.Err()
	}
}
func (n *blockingNode) Validate() error                               { return nil }
func (n *blockingNode) GetConfig() map[string]interface{}             { return nil }
func (n *blockingNode) SetConfig(config map[string]interface{}) error { return nil }

// contextProbeNode records whether its context was still alive a little
// while after the upstream node finished.
type contextProbeNode struct {
	mu   sync.Mutex
	errs []error
}

func (n *contextProbeNode) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	c := ctx.(context.Context)
	time.Sleep(50 * time.Millisecond)
	n.mu.Lock()
	n.errs = append(n.errs, c.Err())
	n.mu.Unlock()
	return input, nil
}
func (n *contextProbeNode) Validate() error                               { return nil }
func (n *contextProbeNode) GetConfig() map[string]interface{}             { return nil }
func (n *contextProbeNode) SetConfig(config map[string]interface{}) error { return nil }

func twoNodeFlow(id, sourceType, targetType string) *Flow {
	flow := NewFlow(id, id)
	flow.Nodes["src"] = &Node{ID: "src", Type: sourceType, Config: map[string]interface{}{}}
	flow.Nodes["dst"] = &Node{ID: "dst", Type: targetType, Config: map[string]interface{}{}}
	flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "src", SourcePort: "output", TargetNode: "dst", TargetPort: "input"}}
	return flow
}

func TestDispatch_BoundsInflightExecutionsAndUndeployWaitsForThem(t *testing.T) {
	reg := registry.NewNodeRegistry()
	blocker := newBlockingNode()
	require.NoError(t, reg.RegisterFactory("pass", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "pass"}))
	require.NoError(t, reg.RegisterFactory("block", func() registry.NodeExecutor { return blocker }, registry.NodeMetadata{Type: "block"}))

	e := NewFlowEngine(EngineConfig{MessageBufferSize: 100, DefaultTimeout: 2 * time.Second}, reg)
	require.NoError(t, e.Start())
	defer e.Stop()

	flow := twoNodeFlow("bounded", "pass", "block")
	flow.Config.MaxConcurrency = 2
	require.NoError(t, e.Deploy(flow))

	for i := 0; i < 6; i++ {
		require.NoError(t, e.InjectMessage("bounded", "src", map[string]interface{}{"i": i}))
	}

	// Two executions start, the other four wait in the queue for a slot.
	for i := 0; i < 2; i++ {
		select {
		case <-blocker.started:
		case <-time.After(2 * time.Second):
			t.Fatal("blocked executions did not start")
		}
	}
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int64(2), blocker.running.Load(), "only MaxConcurrency executions run at once")

	// Undeploy cancels the running executions and returns once they are gone.
	started := time.Now()
	require.NoError(t, e.Undeploy("bounded"))
	assert.Less(t, time.Since(started), time.Second, "undeploy waits for cancelled executions, not for the timeout")
	assert.Equal(t, int64(0), blocker.running.Load())
	assert.LessOrEqual(t, blocker.peak.Load(), int64(2))
	assert.False(t, e.IsDeployed("bounded"))
}

func TestDispatch_DownstreamNodeKeepsALiveContext(t *testing.T) {
	reg := registry.NewNodeRegistry()
	probe := &contextProbeNode{}
	require.NoError(t, reg.RegisterFactory("pass", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "pass"}))
	require.NoError(t, reg.RegisterFactory("probe", func() registry.NodeExecutor { return probe }, registry.NodeMetadata{Type: "probe"}))

	e := NewFlowEngine(EngineConfig{MessageBufferSize: 10, DefaultTimeout: 5 * time.Second}, reg)
	require.NoError(t, e.Start())
	defer e.Stop()

	require.NoError(t, e.Deploy(twoNodeFlow("ctx", "pass", "probe")))
	require.NoError(t, e.InjectMessage("ctx", "src", map[string]interface{}{"x": 1}))

	require.Eventually(t, func() bool {
		probe.mu.Lock()
		defer probe.mu.Unlock()
		return len(probe.errs) == 1
	}, 2*time.Second, 10*time.Millisecond)
	probe.mu.Lock()
	defer probe.mu.Unlock()
	assert.NoError(t, probe.errs[0], "a node's context must not be cancelled because the upstream node finished")
}

func TestDispatch_FullQueueDropsAndCounts(t *testing.T) {
	reg := registry.NewNodeRegistry()
	blocker := newBlockingNode()
	require.NoError(t, reg.RegisterFactory("pass", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "pass"}))
	require.NoError(t, reg.RegisterFactory("block", func() registry.NodeExecutor { return blocker }, registry.NodeMetadata{Type: "block"}))

	e := NewFlowEngine(EngineConfig{MessageBufferSize: 2, DefaultTimeout: 2 * time.Second}, reg)
	require.NoError(t, e.Start())
	defer e.Stop()

	flow := twoNodeFlow("drops", "pass", "block")
	flow.Config.MaxConcurrency = 1
	require.NoError(t, e.Deploy(flow))

	// One execution holds the only slot; the dispatcher then blocks on the
	// second message, so the queue (2) fills and everything beyond is dropped.
	for i := 0; i < 20; i++ {
		e.SubmitMessage(Message{FlowID: "drops", Path: []string{"src"}, Payload: map[string]interface{}{"i": i}, Context: context.Background()})
	}
	assert.Greater(t, e.MessagesDropped(), uint64(0))

	// Messages for flows that are not deployed are dropped silently.
	before := e.MessagesDropped()
	e.SubmitMessage(Message{FlowID: "nope", Payload: map[string]interface{}{}})
	assert.Equal(t, before, e.MessagesDropped())

	close(blocker.release)
	require.NoError(t, e.Undeploy("drops"))
}

func TestDispatch_RedeployCyclesLeakNoGoroutines(t *testing.T) {
	reg := registry.NewNodeRegistry()
	sink := &captureSinkNode{}
	require.NoError(t, reg.RegisterFactory("pass", func() registry.NodeExecutor { return passthroughNode{} }, registry.NodeMetadata{Type: "pass"}))
	require.NoError(t, reg.RegisterFactory("sink", func() registry.NodeExecutor { return sink }, registry.NodeMetadata{Type: "sink"}))

	e := NewFlowEngine(EngineConfig{MessageBufferSize: 100, DefaultTimeout: time.Second}, reg)
	require.NoError(t, e.Start())
	defer e.Stop()

	flow := twoNodeFlow("cycle", "pass", "sink")
	require.NoError(t, e.Deploy(flow))
	require.NoError(t, e.InjectMessage("cycle", "src", map[string]interface{}{"warm": true}))
	require.Eventually(t, func() bool { return sink.receivedCount() == 1 }, 2*time.Second, 10*time.Millisecond)

	runtime.GC()
	baseline := runtime.NumGoroutine()

	// Each redeploy stops the previous runtime (which cancels whatever it
	// still had queued), so wait for every message before the next cycle.
	for i := 0; i < 25; i++ {
		require.NoError(t, e.Deploy(flow.Clone()))
		require.NoError(t, e.InjectMessage("cycle", "src", map[string]interface{}{"i": i}))
		want := i + 2
		require.Eventually(t, func() bool { return sink.receivedCount() == want }, 2*time.Second, 5*time.Millisecond)
	}
	require.NoError(t, e.Undeploy("cycle"))
	require.NoError(t, e.Deploy(flow.Clone()))

	require.Eventually(t, func() bool {
		runtime.GC()
		return runtime.NumGoroutine() <= baseline+2
	}, 3*time.Second, 50*time.Millisecond, "goroutines after 25 redeploys: %d, baseline %d", runtime.NumGoroutine(), baseline)
	assert.Equal(t, 26, sink.receivedCount())
}

func TestMessageLog_IsARingBufferAndOffByDefault(t *testing.T) {
	e := NewFlowEngine(EngineConfig{}, registry.NewNodeRegistry())

	e.AddMessageToLog(NewMessage(map[string]interface{}{"n": 0}, "f"))
	assert.Empty(t, e.GetMessageLog(), "disabled by default")

	e.SetMaxMessageLog(3)
	for i := 1; i <= 5; i++ {
		e.AddMessageToLog(NewMessage(map[string]interface{}{"n": i}, "f"))
	}
	logged := e.GetMessageLog()
	require.Len(t, logged, 3)
	assert.Equal(t, 3, logged[0].Payload["n"])
	assert.Equal(t, 5, logged[2].Payload["n"])
	assert.Len(t, e.GetMessageLogForFlow("f"), 3)
	assert.Empty(t, e.GetMessageLogForFlow("other"))

	e.ClearMessageLog()
	assert.Empty(t, e.GetMessageLog())

	e.SetMaxMessageLog(0)
	e.AddMessageToLog(NewMessage(map[string]interface{}{"n": 9}, "f"))
	assert.Empty(t, e.GetMessageLog())
}
