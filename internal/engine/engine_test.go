package engine

import (
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestEngine() *FlowEngine {
	config := EngineConfig{
		WorkerPoolSize:    10,
		MessageBufferSize: 1000,
		DefaultTimeout:    30 * time.Second,
	}
	// Use global registry which has built-in nodes registered
	nodeRegistry := registry.GetGlobalRegistry()
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

func TestFlowEngineDeploy(t *testing.T) {
	t.Run("should deploy valid flow", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("deploy-test-1", "Deploy Test Flow")
		flow.Nodes["node-1"] = &Node{
			ID:   "node-1",
			Type: "debug",
			X:    0,
			Y:    0,
		}

		err := engine.Deploy(flow)
		require.NoError(t, err)

		deployed, err := engine.GetFlow("deploy-test-1")
		require.NoError(t, err)
		assert.Equal(t, "deploy-test-1", deployed.ID)
		assert.Equal(t, FlowStatusActive, deployed.Status)
	})

	t.Run("should fail to deploy flow with no nodes", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("empty-flow", "Empty Flow")

		err := engine.Deploy(flow)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid flow")
	})

	t.Run("should fail to deploy already deployed flow", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("duplicate-deploy", "Duplicate Deploy")
		flow.Nodes["node-1"] = &Node{
			ID:   "node-1",
			Type: "debug",
		}

		err := engine.Deploy(flow)
		require.NoError(t, err)

		// Try to deploy again
		err = engine.Deploy(flow)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already deployed")
	})

	t.Run("should deploy flow with connections", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("flow-with-connections", "Flow with Connections")
		flow.Nodes["node-1"] = &Node{
			ID:   "node-1",
			Type: "inject",
			X:    0,
			Y:    0,
		}
		flow.Nodes["node-2"] = &Node{
			ID:   "node-2",
			Type: "debug",
			X:    100,
			Y:    0,
		}
		flow.Connections = []NodeConnection{
			{
				ID:          "conn-1",
				SourceNode:  "node-1",
				TargetNode:  "node-2",
			},
		}

		err := engine.Deploy(flow)
		require.NoError(t, err)

		deployed, err := engine.GetFlow("flow-with-connections")
		require.NoError(t, err)
		assert.Len(t, deployed.Nodes, 2)
		assert.Len(t, deployed.Connections, 1)
	})
}

func TestFlowEngineUndeploy(t *testing.T) {
	t.Run("should undeploy flow", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("undeploy-test", "Undeploy Test")
		flow.Nodes["node-1"] = &Node{
			ID:   "node-1",
			Type: "debug",
		}

		// Deploy first
		err := engine.Deploy(flow)
		require.NoError(t, err)

		// Verify deployed
		_, err = engine.GetFlow("undeploy-test")
		require.NoError(t, err)

		// Undeploy
		err = engine.Undeploy("undeploy-test")
		require.NoError(t, err)

		// Verify not found
		_, err = engine.GetFlow("undeploy-test")
		assert.Error(t, err)
	})

	t.Run("should fail to undeploy non-existent flow", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		err := engine.Undeploy("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFlowEngineLoadAllFlows(t *testing.T) {
	// For now, skip these tests as they require a more complex mock setup
	// These scenarios are tested in the cmd/go-red/main_test.go tests
	t.Skip("LoadAllFlows tests require state manager mock - tested in cmd/go-red package")
}


