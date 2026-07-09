package dto

import (
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

// FlowStatusFromEngine translates the engine's internal FlowStatus enum to
// the canonical wire enum. This is the only place that translation happens;
// previously it was duplicated (and had to be kept in sync by hand) between
// main.go's convertFlowStatusAPI and websocket/integration.go's convertFlowStatus.
func FlowStatusFromEngine(s engine.FlowStatus) FlowStatus {
	switch s {
	case engine.FlowStatusInactive:
		return FlowStatusDraft
	case engine.FlowStatusActive:
		return FlowStatusRunning
	case engine.FlowStatusError:
		return FlowStatusError
	case engine.FlowStatusDeploying:
		return FlowStatusDeploying
	case engine.FlowStatusUndeploying:
		return FlowStatusUndeploying
	default:
		return FlowStatus(s)
	}
}

// NodeToWire converts an engine Node to its canonical wire representation.
func NodeToWire(n *engine.Node) Node {
	return Node{
		ID:       n.ID,
		Type:     n.Type,
		Name:     n.Name,
		Position: Position{X: n.X, Y: n.Y},
		Config:   n.Config,
		Status:   NodeStatus{State: "idle"},
		Disabled: n.Disabled,
	}
}

// NodeFromWire converts a wire Node back to an engine Node, given its map key ID.
func NodeFromWire(id string, n Node) *engine.Node {
	return &engine.Node{
		ID:       id,
		Type:     n.Type,
		Name:     n.Name,
		Config:   n.Config,
		X:        n.Position.X,
		Y:        n.Position.Y,
		Disabled: n.Disabled,
	}
}

// ConnectionToWire converts an engine NodeConnection to its canonical wire representation.
func ConnectionToWire(c engine.NodeConnection) Connection {
	return Connection{
		ID:         c.ID,
		SourceNode: c.SourceNode,
		SourcePort: c.SourcePort,
		TargetNode: c.TargetNode,
		TargetPort: c.TargetPort,
	}
}

// ConnectionFromWire converts a wire Connection back to an engine NodeConnection.
func ConnectionFromWire(c Connection) engine.NodeConnection {
	return engine.NodeConnection{
		ID:         c.ID,
		SourceNode: c.SourceNode,
		SourcePort: c.SourcePort,
		TargetNode: c.TargetNode,
		TargetPort: c.TargetPort,
	}
}

// ToWire converts an engine Flow to its canonical wire representation.
// This is the only flow-to-wire conversion in the codebase; it replaces
// main.go's convertFlowToFrontendAPI and websocket/integration.go's
// convertFlowToFrontend, which were hand-duplicated copies of each other.
func ToWire(f *engine.Flow) Flow {
	nodes := make(map[string]Node, len(f.Nodes))
	for id, n := range f.Nodes {
		nodes[id] = NodeToWire(n)
	}

	connections := make([]Connection, len(f.Connections))
	for i, c := range f.Connections {
		connections[i] = ConnectionToWire(c)
	}

	return Flow{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		Nodes:       nodes,
		Connections: connections,
		Status:      FlowStatusFromEngine(f.Status),
		Config: FlowConfig{
			Timeout:        int(f.Config.Timeout.Seconds()),
			MaxConcurrency: f.Config.MaxConcurrency,
			RetryPolicy: RetryPolicy{
				MaxRetries: f.Config.RetryPolicy.MaxRetries,
				Backoff:    int(f.Config.RetryPolicy.Backoff.Seconds()),
				MaxBackoff: int(f.Config.RetryPolicy.MaxBackoff.Seconds()),
				RetryOn:    f.Config.RetryPolicy.RetryOn,
			},
			Environment: f.Config.Environment,
		},
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
		UpdatedAt: f.UpdatedAt.Format(time.RFC3339),
		Version:   f.Version,
	}
}

// ToWireSummary converts an engine Flow to its canonical wire summary
// representation, used by flow-listing endpoints. Replaces the anonymous
// flowResponse struct (main.go), the anonymous flowSummary struct
// (websocket/integration.go), and the formerly-dead engine.FlowSummary type.
func ToWireSummary(f *engine.Flow) FlowSummary {
	return FlowSummary{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		Status:      FlowStatusFromEngine(f.Status),
		NodeCount:   len(f.Nodes),
		CreatedAt:   f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   f.UpdatedAt.Format(time.RFC3339),
	}
}

// ApplyTo applies the request's fields onto an existing engine Flow.
// Only fields present in the request are applied: nodes are merged
// (existing nodes not mentioned in the request are left untouched;
// mentioned nodes are created or updated), connections fully replace the
// flow's connection list when provided, and config fields are merged into
// the existing FlowConfig. This mirrors the merge semantics main.go's
// handleUpdateFlow implemented by hand; it replaces that hand-parsing plus
// websocket/integration.go's handleFlowUpdate (which previously, and
// inconsistently, cleared all nodes on every update).
func (req *FlowUpdateRequest) ApplyTo(f *engine.Flow) {
	if req.Name != nil && *req.Name != "" {
		f.Name = *req.Name
	}
	if req.Description != nil {
		f.Description = *req.Description
	}
	if req.Nodes != nil {
		for id, n := range req.Nodes {
			if existing, ok := f.Nodes[id]; ok {
				existing.Type = n.Type
				existing.Config = n.Config
				existing.X = n.Position.X
				existing.Y = n.Position.Y
				existing.Disabled = n.Disabled
			} else {
				f.Nodes[id] = NodeFromWire(id, n)
			}
		}
	}
	if req.Connections != nil {
		conns := make([]engine.NodeConnection, len(req.Connections))
		for i, c := range req.Connections {
			conns[i] = ConnectionFromWire(c)
		}
		f.Connections = conns
	}
	if req.Config != nil {
		f.Config.Timeout = time.Duration(req.Config.Timeout) * time.Second
		f.Config.MaxConcurrency = req.Config.MaxConcurrency
		for k, v := range req.Config.Environment {
			f.Config.Environment[k] = v
		}
	}
	f.UpdatedAt = time.Now().UTC()
}

// PopulateFromWire replaces f's nodes, connections, config, and description
// with the given wire Flow. Unlike ApplyTo, this always fully replaces
// (rather than merges) — it's used for import, where f is a freshly created
// flow with nothing to merge against.
func PopulateFromWire(f *engine.Flow, wire Flow) {
	f.Description = wire.Description

	f.Nodes = make(map[string]*engine.Node, len(wire.Nodes))
	for id, n := range wire.Nodes {
		f.Nodes[id] = NodeFromWire(id, n)
	}

	f.Connections = make([]engine.NodeConnection, len(wire.Connections))
	for i, c := range wire.Connections {
		f.Connections[i] = ConnectionFromWire(c)
	}

	f.Config.Timeout = time.Duration(wire.Config.Timeout) * time.Second
	f.Config.MaxConcurrency = wire.Config.MaxConcurrency
	f.Config.RetryPolicy = engine.RetryPolicy{
		MaxRetries: wire.Config.RetryPolicy.MaxRetries,
		Backoff:    time.Duration(wire.Config.RetryPolicy.Backoff) * time.Second,
		MaxBackoff: time.Duration(wire.Config.RetryPolicy.MaxBackoff) * time.Second,
		RetryOn:    wire.Config.RetryPolicy.RetryOn,
	}
	if wire.Config.Environment != nil {
		f.Config.Environment = wire.Config.Environment
	}
}

// MessageToWire converts an engine Message to its canonical wire
// representation.
func MessageToWire(m *engine.Message) Message {
	return Message{
		ID:        m.ID,
		FlowID:    m.FlowID,
		Payload:   m.Payload,
		Metadata:  m.Metadata,
		Path:      m.Path,
		Timestamp: m.Timestamp.Format(time.RFC3339),
	}
}

// MessagesToWire converts a slice of engine Messages to their canonical
// wire representation.
func MessagesToWire(msgs []engine.Message) []Message {
	wire := make([]Message, len(msgs))
	for i, m := range msgs {
		wire[i] = MessageToWire(&m)
	}
	return wire
}
