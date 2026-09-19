package engine

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Runtime events.
//
// The engine publishes what happens at runtime (a flow starting or stopping,
// a node reporting its status, a Debug node emitting a message, a node
// failing, per-node message counters) to subscribers registered with
// SubscribeEvents. The WebSocket hub is the main subscriber and forwards the
// events to browser clients. Publishing never blocks a node: events go
// through a bounded queue drained by a dispatcher goroutine, and are dropped
// (and counted) if the queue is full.

// Event is a runtime event. The concrete types below implement it.
type Event interface {
	// EventFlowID returns the flow the event belongs to.
	EventFlowID() string
}

// FlowStatusEvent reports a flow's lifecycle status change (deployed,
// undeployed, failed to deploy).
type FlowStatusEvent struct {
	FlowID     string
	Status     FlowStatus
	UpdatedAt  time.Time
	DeployedAt time.Time
	// Error carries the deploy error when Status is FlowStatusError.
	Error string
}

// EventFlowID implements Event.
func (e FlowStatusEvent) EventFlowID() string { return e.FlowID }

// NodeStatus is the small status indicator shown under a node in the editor,
// modelled after Node-RED's node.status({fill, shape, text}).
type NodeStatus struct {
	// Fill is the indicator color: red, green, yellow, blue or grey.
	Fill string
	// Shape is "dot" or "ring".
	Shape string
	// Text is the short label next to the indicator.
	Text string
	// Timestamp is when the status was reported.
	Timestamp time.Time
}

// NodeStatusEvent reports that a node changed its status.
type NodeStatusEvent struct {
	FlowID   string
	NodeID   string
	NodeType string
	Status   NodeStatus
}

// EventFlowID implements Event.
func (e NodeStatusEvent) EventFlowID() string { return e.FlowID }

// DebugLevel classifies debug sidebar entries.
type DebugLevel string

const (
	// DebugLevelDebug is regular Debug node output.
	DebugLevelDebug DebugLevel = "debug"
	// DebugLevelWarn is a non-fatal problem a node reported.
	DebugLevelWarn DebugLevel = "warn"
	// DebugLevelError is a node execution failure.
	DebugLevelError DebugLevel = "error"
)

// DebugEvent is one entry of the debug sidebar: Debug node output or a node
// error. Payload is JSON-serializable data (a map, a string, a number...).
type DebugEvent struct {
	ID        string
	FlowID    string
	NodeID    string
	NodeName  string
	NodeType  string
	Level     DebugLevel
	Topic     string
	Payload   interface{}
	Timestamp time.Time
}

// EventFlowID implements Event.
func (e DebugEvent) EventFlowID() string { return e.FlowID }

// NodeMetrics are a node's counters since its flow was deployed.
type NodeMetrics struct {
	Messages uint64
	Errors   uint64
}

// FlowMetricsEvent carries the counters of every node of a flow that
// processed messages since the flow was deployed. Published once per second
// while counters change.
type FlowMetricsEvent struct {
	FlowID    string
	Nodes     map[string]NodeMetrics
	Timestamp time.Time
}

// EventFlowID implements Event.
func (e FlowMetricsEvent) EventFlowID() string { return e.FlowID }

const (
	// eventQueueSize bounds how many events may wait for the dispatcher.
	eventQueueSize = 4096
	// debugLogSize is how many debug entries are kept per flow for clients
	// that connect later.
	debugLogSize = 200
	// metricsInterval is how often changed node counters are published.
	metricsInterval = time.Second
)

// eventHub fans events out to subscribers from its own goroutine.
type eventHub struct {
	mu      sync.RWMutex
	subs    map[uint64]func(Event)
	nextID  uint64
	queue   chan Event
	dropped atomic.Uint64
}

func newEventHub() *eventHub {
	return &eventHub{
		subs:  make(map[uint64]func(Event)),
		queue: make(chan Event, eventQueueSize),
	}
}

func (h *eventHub) subscribe(fn func(Event)) func() {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.nextID
	h.nextID++
	h.subs[id] = fn
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs, id)
	}
}

// publish enqueues ev without blocking. When the queue is full the event is
// dropped; the drop is counted and logged at most once per second.
func (h *eventHub) publish(ev Event) {
	select {
	case h.queue <- ev:
	default:
		if n := h.dropped.Add(1); n == 1 || n%1000 == 0 {
			slog.Warn("runtime event queue full, dropping events", "dropped", n)
		}
	}
}

// dispatch delivers queued events to subscribers until ctx is cancelled.
func (h *eventHub) dispatch(ctx context.Context) {
	for {
		select {
		case ev := <-h.queue:
			h.mu.RLock()
			subs := make([]func(Event), 0, len(h.subs))
			for _, fn := range h.subs {
				subs = append(subs, fn)
			}
			h.mu.RUnlock()
			for _, fn := range subs {
				fn(ev)
			}
		case <-ctx.Done():
			return
		}
	}
}

// nodeCounter holds a node's runtime counters.
type nodeCounter struct {
	messages atomic.Uint64
	errors   atomic.Uint64
}

// SubscribeEvents registers fn for every runtime event. The returned
// function removes the subscription. fn runs on the engine's dispatcher
// goroutine and must not block.
func (e *FlowEngine) SubscribeEvents(fn func(Event)) func() {
	return e.events.subscribe(fn)
}

// publishFlowStatus publishes def's current lifecycle status.
func (e *FlowEngine) publishFlowStatus(def *Flow, errMsg string) {
	e.events.publish(FlowStatusEvent{
		FlowID:     def.ID,
		Status:     def.Status,
		UpdatedAt:  def.UpdatedAt,
		DeployedAt: def.DeployedAt,
		Error:      errMsg,
	})
}

// recordDebug appends ev to its flow's ring buffer and publishes it.
func (e *FlowEngine) recordDebug(ev DebugEvent) {
	if ev.ID == "" {
		ev.ID = uuid.New().String()
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	if ev.Level == "" {
		ev.Level = DebugLevelDebug
	}

	e.debugMu.Lock()
	entries := append(e.debugLogs[ev.FlowID], ev)
	if len(entries) > debugLogSize {
		entries = entries[len(entries)-debugLogSize:]
	}
	e.debugLogs[ev.FlowID] = entries
	e.debugMu.Unlock()

	e.events.publish(ev)
}

// GetDebugLog returns the recent debug entries of a flow, oldest first.
func (e *FlowEngine) GetDebugLog(flowID string) []DebugEvent {
	e.debugMu.Lock()
	defer e.debugMu.Unlock()
	entries := make([]DebugEvent, len(e.debugLogs[flowID]))
	copy(entries, e.debugLogs[flowID])
	return entries
}

// ClearDebugLog forgets a flow's debug entries.
func (e *FlowEngine) ClearDebugLog(flowID string) {
	e.debugMu.Lock()
	defer e.debugMu.Unlock()
	delete(e.debugLogs, flowID)
}

// setNodeStatus records and publishes a node's status.
func (e *FlowEngine) setNodeStatus(flowID, nodeID, nodeType string, status NodeStatus) {
	if status.Timestamp.IsZero() {
		status.Timestamp = time.Now().UTC()
	}
	if status.Shape == "" {
		status.Shape = "dot"
	}

	e.statusMu.Lock()
	flowStatus, ok := e.nodeStatus[flowID]
	if !ok {
		flowStatus = make(map[string]NodeStatus)
		e.nodeStatus[flowID] = flowStatus
	}
	flowStatus[nodeID] = status
	e.statusMu.Unlock()

	e.events.publish(NodeStatusEvent{FlowID: flowID, NodeID: nodeID, NodeType: nodeType, Status: status})
}

// clearNodeStatus forgets every node status of a flow (its nodes are gone
// or about to be recreated).
func (e *FlowEngine) clearNodeStatus(flowID string) {
	e.statusMu.Lock()
	delete(e.nodeStatus, flowID)
	e.statusMu.Unlock()
}

// GetNodeStatuses returns the latest status of every node of a flow that
// reported one.
func (e *FlowEngine) GetNodeStatuses(flowID string) map[string]NodeStatus {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	out := make(map[string]NodeStatus, len(e.nodeStatus[flowID]))
	for id, status := range e.nodeStatus[flowID] {
		out[id] = status
	}
	return out
}

// GetMetrics returns the counters of every node of a running flow that
// processed at least one message. Empty for flows that are not running.
func (e *FlowEngine) GetMetrics(flowID string) map[string]NodeMetrics {
	e.mu.RLock()
	activeFlow, ok := e.active[flowID]
	e.mu.RUnlock()
	if !ok {
		return map[string]NodeMetrics{}
	}
	return activeFlow.metricsSnapshot()
}

// metricsSnapshot reads every node counter of the flow.
func (af *ActiveFlow) metricsSnapshot() map[string]NodeMetrics {
	out := make(map[string]NodeMetrics)
	for nodeID, counter := range af.counters {
		messages := counter.messages.Load()
		errors := counter.errors.Load()
		if messages == 0 && errors == 0 {
			continue
		}
		out[nodeID] = NodeMetrics{Messages: messages, Errors: errors}
	}
	return out
}

func (af *ActiveFlow) countMessage(nodeID string) {
	if counter, ok := af.counters[nodeID]; ok {
		counter.messages.Add(1)
	}
}

func (af *ActiveFlow) countError(nodeID string) {
	if counter, ok := af.counters[nodeID]; ok {
		counter.errors.Add(1)
	}
}

// metricsLoop publishes the counters of every running flow whose counters
// changed since the last tick, until the engine stops.
func (e *FlowEngine) metricsLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(metricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.mu.RLock()
			flows := make([]*ActiveFlow, 0, len(e.active))
			for _, af := range e.active {
				flows = append(flows, af)
			}
			e.mu.RUnlock()

			for _, af := range flows {
				snapshot := af.metricsSnapshot()
				if metricsEqual(snapshot, af.lastMetrics) {
					continue
				}
				af.lastMetrics = snapshot
				e.events.publish(FlowMetricsEvent{FlowID: af.Flow.ID, Nodes: snapshot, Timestamp: time.Now().UTC()})
			}
		case <-e.ctx.Done():
			return
		}
	}
}

func metricsEqual(a, b map[string]NodeMetrics) bool {
	if len(a) != len(b) {
		return false
	}
	for id, m := range a {
		if b[id] != m {
			return false
		}
	}
	return true
}

// fillForStatus picks an indicator color for a free-form status string
// reported through registry.NodeRuntime.ReportStatus.
func fillForStatus(status string) string {
	s := strings.ToLower(status)
	switch {
	case strings.Contains(s, "error"), strings.Contains(s, "fail"), strings.Contains(s, "disconnect"), strings.Contains(s, "closed"):
		return "red"
	case strings.Contains(s, "connecting"), strings.Contains(s, "waiting"), strings.Contains(s, "pending"), strings.Contains(s, "retry"):
		return "yellow"
	case strings.Contains(s, "connected"), strings.Contains(s, "listening"), strings.Contains(s, "ready"), strings.Contains(s, "ok"), strings.Contains(s, "running"):
		return "green"
	default:
		return "blue"
	}
}
