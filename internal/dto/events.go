package dto

import (
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

// Runtime events pushed over the WebSocket (see docs/PROTOCOL.md). Every
// struct here is generated into web/src/types/generated.ts.

// FlowStatusEvent is the payload of flow:status.
type FlowStatusEvent struct {
	FlowID     string     `json:"flowId"`
	Status     FlowStatus `json:"status"`
	UpdatedAt  string     `json:"updatedAt,omitempty"`
	DeployedAt string     `json:"deployedAt,omitempty"`
	// Error is the deploy error when Status is "error".
	Error string `json:"error,omitempty"`
}

// NodeStatusEvent is the payload of node:status.
type NodeStatusEvent struct {
	FlowID string     `json:"flowId"`
	NodeID string     `json:"nodeId"`
	Status NodeStatus `json:"status"`
}

// DebugMessage is one debug sidebar entry: the payload of debug:message and
// the elements of FlowSnapshot.Debug.
type DebugMessage struct {
	ID       string `json:"id"`
	FlowID   string `json:"flowId"`
	NodeID   string `json:"nodeId"`
	NodeName string `json:"nodeName,omitempty"`
	NodeType string `json:"nodeType,omitempty"`
	// Level is "debug", "warn" or "error".
	Level     string      `json:"level"`
	Topic     string      `json:"topic,omitempty"`
	Payload   interface{} `json:"payload"`
	Timestamp string      `json:"timestamp"`
}

// NodeMetrics are a node's counters since its flow was deployed.
type NodeMetrics struct {
	Messages uint64 `json:"messages"`
	Errors   uint64 `json:"errors"`
}

// FlowMetricsEvent is the payload of flow:metrics.
type FlowMetricsEvent struct {
	FlowID    string                 `json:"flowId"`
	Nodes     map[string]NodeMetrics `json:"nodes"`
	Timestamp string                 `json:"timestamp"`
}

// FlowSnapshot is the payload of flow:snapshot, sent right after a client
// subscribes to a flow: everything it needs to catch up.
type FlowSnapshot struct {
	FlowID     string                 `json:"flowId"`
	Status     FlowStatus             `json:"status"`
	NodeStatus map[string]NodeStatus  `json:"nodeStatus"`
	Metrics    map[string]NodeMetrics `json:"metrics"`
	Debug      []DebugMessage         `json:"debug"`
}

// SubscribeRequest is the payload of subscribe and unsubscribe.
type SubscribeRequest struct {
	FlowID string `json:"flowId"`
}

// FlowStatusEventToWire converts an engine event.
func FlowStatusEventToWire(ev engine.FlowStatusEvent) FlowStatusEvent {
	return FlowStatusEvent{
		FlowID:     ev.FlowID,
		Status:     FlowStatusFromEngine(ev.Status),
		UpdatedAt:  formatOptionalTime(ev.UpdatedAt),
		DeployedAt: formatOptionalTime(ev.DeployedAt),
		Error:      ev.Error,
	}
}

// NodeStatusToWire converts an engine node status.
func NodeStatusToWire(status engine.NodeStatus) NodeStatus {
	return NodeStatus{
		Fill:      status.Fill,
		Shape:     status.Shape,
		Text:      status.Text,
		Timestamp: formatOptionalTime(status.Timestamp),
	}
}

// NodeStatusesToWire converts a flow's node status map.
func NodeStatusesToWire(statuses map[string]engine.NodeStatus) map[string]NodeStatus {
	out := make(map[string]NodeStatus, len(statuses))
	for id, status := range statuses {
		out[id] = NodeStatusToWire(status)
	}
	return out
}

// DebugToWire converts an engine debug entry.
func DebugToWire(ev engine.DebugEvent) DebugMessage {
	return DebugMessage{
		ID:        ev.ID,
		FlowID:    ev.FlowID,
		NodeID:    ev.NodeID,
		NodeName:  ev.NodeName,
		NodeType:  ev.NodeType,
		Level:     string(ev.Level),
		Topic:     ev.Topic,
		Payload:   ev.Payload,
		Timestamp: ev.Timestamp.UTC().Format(time.RFC3339Nano),
	}
}

// DebugLogToWire converts a flow's debug history.
func DebugLogToWire(entries []engine.DebugEvent) []DebugMessage {
	out := make([]DebugMessage, len(entries))
	for i, entry := range entries {
		out[i] = DebugToWire(entry)
	}
	return out
}

// MetricsToWire converts a flow's node counters.
func MetricsToWire(metrics map[string]engine.NodeMetrics) map[string]NodeMetrics {
	out := make(map[string]NodeMetrics, len(metrics))
	for id, m := range metrics {
		out[id] = NodeMetrics{Messages: m.Messages, Errors: m.Errors}
	}
	return out
}
