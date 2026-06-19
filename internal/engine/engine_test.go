package engine

import (
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestEngine() *FlowEngine {
	config := EngineConfig{
		WorkerPoolSize:    10,
		MessageBufferSize: 1000,
		DefaultTimeout:    30 * time.Second,
	}
	nodeRegistry := registry.NewNodeRegistry()
	return NewFlowEngine(config, nodeRegistry)
}

func TestFlowEngineCreation(t *testing.T) {
	t.Run("should create flow engine", func(t *testing.T) {
		engine := createTestEngine()
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.flows)
	})
}

func TestFlowEngineCreateFlow(t *testing.T) {
	t.Run("should create flow with generated ID", func(t *testing.T) {
		engine := createTestEngine()

		flow, err := engine.CreateFlow("", "Test Flow")
		require.NoError(t, err)
		assert.NotNil(t, flow)
		assert.NotEmpty(t, flow.ID)
		assert.Equal(t, "Test Flow", flow.Name)
	})

	t.Run("should create flow with provided ID", func(t *testing.T) {
		engine := createTestEngine()

		flow, err := engine.CreateFlow("flow-123", "Test Flow")
		require.NoError(t, err)
		assert.NotNil(t, flow)
		assert.Equal(t, "flow-123", flow.ID)
		assert.Equal(t, "Test Flow", flow.Name)
	})
}

func TestFlowEngineGetFlow(t *testing.T) {
	t.Run("should get existing flow", func(t *testing.T) {
		engine := createTestEngine()
		flow, _ := engine.CreateFlow("flow-1", "Test Flow")

		retrieved, err := engine.GetFlow("flow-1")
		require.NoError(t, err)
		assert.Equal(t, flow.ID, retrieved.ID)
		assert.Equal(t, flow.Name, retrieved.Name)
	})

	t.Run("should fail to get non-existent flow", func(t *testing.T) {
		engine := createTestEngine()

		_, err := engine.GetFlow("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFlowEngineGetAllFlows(t *testing.T) {
	t.Run("should return all flows", func(t *testing.T) {
		engine := createTestEngine()
		engine.CreateFlow("flow-1", "Flow 1")
		engine.CreateFlow("flow-2", "Flow 2")
		engine.CreateFlow("flow-3", "Flow 3")

		flows := engine.GetAllFlows()
		assert.Len(t, flows, 3)

		ids := make([]string, len(flows))
		for i, flow := range flows {
			ids[i] = flow.ID
		}
		assert.Contains(t, ids, "flow-1")
		assert.Contains(t, ids, "flow-2")
		assert.Contains(t, ids, "flow-3")
	})

	t.Run("should return empty for no flows", func(t *testing.T) {
		engine := createTestEngine()

		flows := engine.GetAllFlows()
		assert.Len(t, flows, 0)
	})
}

func TestFlowEngineGetFlowsSummary(t *testing.T) {
	t.Run("should return summary of all flows", func(t *testing.T) {
		engine := createTestEngine()
		engine.CreateFlow("flow-1", "Flow 1")
		engine.CreateFlow("flow-2", "Flow 2")
		engine.CreateFlow("flow-3", "Flow 3")

		summaries := engine.GetAllFlowsSummary()
		assert.Len(t, summaries, 3)

		for _, summary := range summaries {
			assert.Contains(t, summary.ID, "flow-")
			assert.Contains(t, summary.Name, "Flow")
		}
	})

	t.Run("should return empty for no flows", func(t *testing.T) {
		engine := createTestEngine()

		summaries := engine.GetAllFlowsSummary()
		assert.Len(t, summaries, 0)
	})
}


