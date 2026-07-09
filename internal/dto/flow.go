// Package dto defines the canonical wire format shared between the Go
// backend and the TypeScript frontend for REST and WebSocket payloads.
//
// These types (plus internal/registry's NodeMetadata/Port/Property/Schema
// and cmd/go-red/websocket's WebSocketMessage/MessageType) are the single
// source of truth for the frontend/backend contract. web/src/types/generated.ts
// is generated from them via `go generate ./internal/dto/...` (see
// cmd/gentypes). Do not hand-describe this shape in TypeScript or in
// documentation — change it here and regenerate.
package dto

//go:generate go run ../../cmd/gentypes -out ../../web/src/types/generated.ts

// FlowStatus is the wire representation of a flow's lifecycle state.
type FlowStatus string

const (
	FlowStatusDraft       FlowStatus = "draft"
	FlowStatusRunning     FlowStatus = "running"
	FlowStatusError       FlowStatus = "error"
	FlowStatusDeploying   FlowStatus = "deploying"
	FlowStatusUndeploying FlowStatus = "undeploying"
)

// Position is a node's location on the flow editor canvas.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeStatus is the runtime status of a node, as shown in the UI.
type NodeStatus struct {
	State           string `json:"state"`
	Message         string `json:"message,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
	ProcessingCount int    `json:"processingCount,omitempty"`
	ErrorCount      int    `json:"errorCount,omitempty"`
}

// Node is the wire representation of a flow node.
type Node struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name,omitempty"`
	Position Position               `json:"position"`
	Config   map[string]interface{} `json:"config"`
	Status   NodeStatus             `json:"status"`
	Disabled bool                   `json:"disabled"`
}

// Connection is the wire representation of a connection between two nodes.
type Connection struct {
	ID         string `json:"id"`
	SourceNode string `json:"sourceNode"`
	SourcePort string `json:"sourcePort,omitempty"`
	TargetNode string `json:"targetNode"`
	TargetPort string `json:"targetPort,omitempty"`
}

// RetryPolicy is the wire representation of a flow's retry behavior.
// Backoff/MaxBackoff are whole seconds (not the nanosecond time.Duration
// used internally by internal/engine).
type RetryPolicy struct {
	MaxRetries int      `json:"maxRetries"`
	Backoff    int      `json:"backoff"`
	MaxBackoff int      `json:"maxBackoff"`
	RetryOn    []string `json:"retryOn"`
}

// FlowConfig is the wire representation of a flow's configuration.
// Timeout is whole seconds (not the nanosecond time.Duration used
// internally by internal/engine).
type FlowConfig struct {
	Timeout        int               `json:"timeout"`
	MaxConcurrency int               `json:"maxConcurrency"`
	RetryPolicy    RetryPolicy       `json:"retryPolicy"`
	Environment    map[string]string `json:"environment"`
}

// Flow is the canonical wire representation of a flow, used by every
// REST endpoint and WebSocket message that carries a full flow (GET/PUT/POST
// /api/flows, flow:get/flow:create/flow:update, export/import, state:sync).
type Flow struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Nodes       map[string]Node `json:"nodes"`
	Connections []Connection    `json:"connections"`
	Status      FlowStatus      `json:"status"`
	Config      FlowConfig      `json:"config"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
	Version     string          `json:"version"`
}

// FlowSummary is the canonical wire representation of a flow listing entry,
// used by GET /api/flows and the WebSocket flow:list broadcast.
type FlowSummary struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Status      FlowStatus `json:"status"`
	NodeCount   int        `json:"nodeCount"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
}

// FlowCreateRequest is the request body for POST /api/flows and the
// flow:create WebSocket message.
type FlowCreateRequest struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// FlowUpdateRequest is the request body for PUT /api/flows/{id} and the
// flow:update WebSocket message. All fields are optional; only fields
// present in the request are applied (see ApplyTo).
type FlowUpdateRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Nodes       map[string]Node `json:"nodes,omitempty"`
	Connections []Connection    `json:"connections,omitempty"`
	Config      *FlowConfig     `json:"config,omitempty"`
}
