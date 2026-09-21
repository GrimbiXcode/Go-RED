package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// eventRecorder collects runtime events for assertions.
type eventRecorder struct {
	mu     sync.Mutex
	events []Event
}

func (r *eventRecorder) record(ev Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
}

func (r *eventRecorder) find(match func(Event) bool) (Event, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ev := range r.events {
		if match(ev) {
			return ev, true
		}
	}
	return nil, false
}

func (r *eventRecorder) has(match func(Event) bool) bool {
	_, ok := r.find(match)
	return ok
}

func injectDebugFlow(id string) *Flow {
	flow := NewFlow(id, "Events")
	flow.Nodes["n1"] = &Node{ID: "n1", Type: "inject", Name: "Trigger"}
	flow.Nodes["n2"] = &Node{ID: "n2", Type: "debug", Name: "Out", Config: map[string]interface{}{"outputToConsole": false}}
	flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "n1", SourcePort: "output", TargetNode: "n2", TargetPort: "input"}}
	return flow
}

func TestRuntimeEvents_DeployDebugMetricsUndeploy(t *testing.T) {
	engine := createTestEngine()
	require.NoError(t, engine.Start())
	defer engine.Stop()

	rec := &eventRecorder{}
	unsubscribe := engine.SubscribeEvents(rec.record)
	defer unsubscribe()

	require.NoError(t, engine.Deploy(injectDebugFlow("ev-1")))

	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			fs, ok := ev.(FlowStatusEvent)
			return ok && fs.FlowID == "ev-1" && fs.Status == FlowStatusActive && !fs.DeployedAt.IsZero()
		})
	}, 2*time.Second, 10*time.Millisecond, "deploy publishes a running flow status")

	require.NoError(t, engine.InjectMessage("ev-1", "n1", map[string]interface{}{"payload": "hi", "topic": "greet"}))

	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			d, ok := ev.(DebugEvent)
			return ok && d.FlowID == "ev-1" && d.NodeID == "n2" && d.NodeName == "Out" && d.Level == DebugLevelDebug && d.Payload == "hi" && d.Topic == "greet"
		})
	}, 2*time.Second, 10*time.Millisecond, "the debug node's output is published")

	log := engine.GetDebugLog("ev-1")
	require.Len(t, log, 1)
	assert.NotEmpty(t, log[0].ID)
	assert.Equal(t, "hi", log[0].Payload)

	assert.Eventually(t, func() bool {
		return engine.GetMetrics("ev-1")["n2"].Messages >= 1
	}, 2*time.Second, 10*time.Millisecond, "counters follow processed messages")

	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			m, ok := ev.(FlowMetricsEvent)
			return ok && m.FlowID == "ev-1" && m.Nodes["n2"].Messages >= 1
		})
	}, 3*time.Second, 20*time.Millisecond, "metrics are published periodically")

	require.NoError(t, engine.Undeploy("ev-1"))
	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			fs, ok := ev.(FlowStatusEvent)
			return ok && fs.FlowID == "ev-1" && fs.Status == FlowStatusInactive
		})
	}, 2*time.Second, 10*time.Millisecond, "undeploy publishes an inactive flow status")
	assert.Empty(t, engine.GetMetrics("ev-1"), "an idle flow has no counters")
	assert.Len(t, engine.GetDebugLog("ev-1"), 1, "the debug history survives undeploy")

	require.NoError(t, engine.DeleteFlow("ev-1"))
	assert.Empty(t, engine.GetDebugLog("ev-1"), "deleting the flow drops its history")
}

func TestRuntimeEvents_NodeErrorsBecomeDebugEntries(t *testing.T) {
	engine := createTestEngine()
	require.NoError(t, engine.Start())
	defer engine.Stop()

	rec := &eventRecorder{}
	defer engine.SubscribeEvents(rec.record)()

	flow := NewFlow("ev-err", "Errors")
	flow.Nodes["n1"] = &Node{ID: "n1", Type: "inject"}
	flow.Nodes["n2"] = &Node{ID: "n2", Type: "function", Name: "Boom", Config: map[string]interface{}{"code": "throw new Error('boom');", "useMsg": true}}
	flow.Connections = []NodeConnection{{ID: "c1", SourceNode: "n1", TargetNode: "n2"}}
	require.NoError(t, engine.Deploy(flow))
	require.NoError(t, engine.InjectMessage("ev-err", "n1", map[string]interface{}{"payload": 1}))

	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			d, ok := ev.(DebugEvent)
			if !ok || d.NodeID != "n2" || d.Level != DebugLevelError {
				return false
			}
			text, _ := d.Payload.(string)
			return assert.ObjectsAreEqual("Boom", d.NodeName) && len(text) > 0
		})
	}, 2*time.Second, 10*time.Millisecond, "a failing node produces an error entry")

	assert.Eventually(t, func() bool {
		return engine.GetMetrics("ev-err")["n2"].Errors == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestRuntimeEvents_NodeStatusReportedByNodes(t *testing.T) {
	engine := createTestEngine()
	require.NoError(t, engine.Start())
	defer engine.Stop()

	rec := &eventRecorder{}
	defer engine.SubscribeEvents(rec.record)()

	require.NoError(t, engine.Deploy(injectDebugFlow("ev-status")))
	_, bus, err := engine.GetFlowContext("ev-status")
	require.NoError(t, err)

	// What a node does through registry.NodeRuntime.ReportStatus.
	bus.PublishStatus(registry.NodeStatusEvent{FlowID: "ev-status", NodeID: "n1", NodeType: "inject", Status: "connected", Detail: "broker-1", Timestamp: time.Now()})

	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			ns, ok := ev.(NodeStatusEvent)
			return ok && ns.NodeID == "n1" && ns.Status.Fill == "green" && ns.Status.Shape == "dot" && ns.Status.Text == "connected broker-1"
		})
	}, 2*time.Second, 10*time.Millisecond)

	statuses := engine.GetNodeStatuses("ev-status")
	require.Contains(t, statuses, "n1")
	assert.Equal(t, "green", statuses["n1"].Fill)

	// A redeploy starts from a clean slate.
	require.NoError(t, engine.DeployFlow("ev-status"))
	assert.Empty(t, engine.GetNodeStatuses("ev-status"))
}

func TestRuntimeEvents_FailedDeployIsReported(t *testing.T) {
	engine := createTestEngine()
	require.NoError(t, engine.Start())
	defer engine.Stop()

	rec := &eventRecorder{}
	defer engine.SubscribeEvents(rec.record)()

	flow := NewFlow("ev-bad", "Bad")
	flow.Nodes["n1"] = &Node{ID: "n1", Type: "no-such-type"}
	assert.Error(t, engine.Deploy(flow))

	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			fs, ok := ev.(FlowStatusEvent)
			return ok && fs.FlowID == "ev-bad" && fs.Status == FlowStatusError && fs.Error != ""
		})
	}, 2*time.Second, 10*time.Millisecond)

	log := engine.GetDebugLog("ev-bad")
	require.Len(t, log, 1)
	assert.Equal(t, DebugLevelError, log[0].Level)
}

func TestFillForStatus(t *testing.T) {
	assert.Equal(t, "green", fillForStatus("connected"))
	assert.Equal(t, "green", fillForStatus("listening"))
	assert.Equal(t, "yellow", fillForStatus("connecting"))
	assert.Equal(t, "red", fillForStatus("disconnected"))
	assert.Equal(t, "red", fillForStatus("error: timeout"))
	assert.Equal(t, "blue", fillForStatus("42 msgs"))
}

func TestDisabledNodes_AreNeitherStartedNorRoutedTo(t *testing.T) {
	engine := createTestEngine()
	require.NoError(t, engine.Start())
	defer engine.Stop()

	rec := &eventRecorder{}
	unsubscribe := engine.SubscribeEvents(rec.record)
	defer unsubscribe()

	flow := injectDebugFlow("dis-1")
	flow.Nodes["n2"].Disabled = true
	flow.Nodes["n3"] = &Node{ID: "n3", Type: "debug", Name: "Alive", Config: map[string]interface{}{"outputToConsole": false}}
	flow.Connections = append(flow.Connections, NodeConnection{ID: "c2", SourceNode: "n1", SourcePort: "output", TargetNode: "n3", TargetPort: "input"})
	require.NoError(t, engine.Deploy(flow))

	engine.mu.RLock()
	_, started := engine.active["dis-1"].nodeExecutors["n2"]
	engine.mu.RUnlock()
	assert.False(t, started, "a disabled node gets no executor")

	require.NoError(t, engine.InjectMessage("dis-1", "n1", map[string]interface{}{"payload": "hi"}))
	assert.Eventually(t, func() bool {
		return rec.has(func(ev Event) bool {
			d, ok := ev.(DebugEvent)
			return ok && d.NodeID == "n3"
		})
	}, 2*time.Second, 10*time.Millisecond, "the enabled sibling still receives the message")
	assert.False(t, rec.has(func(ev Event) bool {
		d, ok := ev.(DebugEvent)
		return ok && d.NodeID == "n2"
	}), "nothing is routed to the disabled node")

	err := engine.InjectMessage("dis-1", "n2", map[string]interface{}{"payload": "hi"})
	assert.ErrorContains(t, err, "disabled")
}
