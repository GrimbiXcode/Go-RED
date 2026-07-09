package dto

import (
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowStatusFromEngine(t *testing.T) {
	cases := map[engine.FlowStatus]FlowStatus{
		engine.FlowStatusInactive:    FlowStatusDraft,
		engine.FlowStatusActive:      FlowStatusRunning,
		engine.FlowStatusError:       FlowStatusError,
		engine.FlowStatusDeploying:   FlowStatusDeploying,
		engine.FlowStatusUndeploying: FlowStatusUndeploying,
	}
	for in, want := range cases {
		assert.Equal(t, want, FlowStatusFromEngine(in))
	}
}

func TestToWireAndSummary(t *testing.T) {
	flow := engine.NewFlow("flow-1", "Flow 1")
	flow.Nodes["n1"] = &engine.Node{ID: "n1", Type: "inject", X: 10, Y: 20, Config: map[string]interface{}{"a": 1}}
	flow.Connections = append(flow.Connections, engine.NodeConnection{ID: "c1", SourceNode: "n1", TargetNode: "n1"})
	flow.Config.Timeout = 30 * time.Second
	flow.Config.RetryPolicy.Backoff = 5 * time.Second

	wire := ToWire(flow)
	assert.Equal(t, "flow-1", wire.ID)
	assert.Equal(t, FlowStatusDraft, wire.Status)
	require.Contains(t, wire.Nodes, "n1")
	assert.Equal(t, Position{X: 10, Y: 20}, wire.Nodes["n1"].Position)
	assert.Equal(t, 30, wire.Config.Timeout)
	assert.Equal(t, 5, wire.Config.RetryPolicy.Backoff)

	summary := ToWireSummary(flow)
	assert.Equal(t, "flow-1", summary.ID)
	assert.Equal(t, 1, summary.NodeCount)
	assert.Equal(t, FlowStatusDraft, summary.Status)
}

func TestFlowUpdateRequestApplyTo(t *testing.T) {
	flow := engine.NewFlow("flow-1", "Flow 1")
	flow.Nodes["n1"] = &engine.Node{ID: "n1", Type: "inject"}

	newName := "Renamed"
	req := &FlowUpdateRequest{
		Name: &newName,
		Nodes: map[string]Node{
			"n1": {Type: "inject", Position: Position{X: 5, Y: 6}},
			"n2": {Type: "debug", Position: Position{X: 1, Y: 2}},
		},
		Config: &FlowConfig{Timeout: 60, MaxConcurrency: 42},
	}
	req.ApplyTo(flow)

	assert.Equal(t, "Renamed", flow.Name)
	require.Contains(t, flow.Nodes, "n1")
	require.Contains(t, flow.Nodes, "n2")
	assert.Equal(t, float64(5), flow.Nodes["n1"].X)
	assert.Equal(t, 60*time.Second, flow.Config.Timeout)
	assert.Equal(t, 42, flow.Config.MaxConcurrency)
}

func TestPopulateFromWire(t *testing.T) {
	flow := engine.NewFlow("flow-1", "Imported")
	wire := Flow{
		Description: "an import",
		Nodes: map[string]Node{
			"n1": {Type: "inject", Position: Position{X: 1, Y: 2}},
		},
		Connections: []Connection{{ID: "c1", SourceNode: "n1", TargetNode: "n1"}},
		Config:      FlowConfig{Timeout: 15, MaxConcurrency: 7},
	}
	PopulateFromWire(flow, wire)

	assert.Equal(t, "an import", flow.Description)
	require.Contains(t, flow.Nodes, "n1")
	assert.Len(t, flow.Connections, 1)
	assert.Equal(t, 15*time.Second, flow.Config.Timeout)
	assert.Equal(t, 7, flow.Config.MaxConcurrency)
}

func TestMessageToWire(t *testing.T) {
	msg := engine.NewMessage(map[string]interface{}{"x": 1}, "flow-1")
	msg.AddToPath("n1")

	wire := MessageToWire(&msg)
	assert.Equal(t, msg.ID, wire.ID)
	assert.Equal(t, "flow-1", wire.FlowID)
	assert.Equal(t, []string{"n1"}, wire.Path)
}
