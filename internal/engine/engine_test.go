package engine

import (
	"testing"
	"time"

	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestEngine() *FlowEngine {
	config := EngineConfig{
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

		flow, err := engine.CreateFlow("", "Test Flow", "")
		require.NoError(t, err)
		assert.NotNil(t, flow)
		assert.NotEmpty(t, flow.ID)
		assert.Equal(t, "Test Flow", flow.Name)
	})

	t.Run("should create flow with provided ID", func(t *testing.T) {
		engine := createTestEngine()

		flow, err := engine.CreateFlow("flow-123", "Test Flow", "")
		require.NoError(t, err)
		assert.NotNil(t, flow)
		assert.Equal(t, "flow-123", flow.ID)
		assert.Equal(t, "Test Flow", flow.Name)
	})
}

func TestFlowEngineGetFlow(t *testing.T) {
	t.Run("should get existing flow", func(t *testing.T) {
		engine := createTestEngine()
		flow, _ := engine.CreateFlow("flow-1", "Test Flow", "")

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
		engine.CreateFlow("flow-1", "Flow 1", "")
		engine.CreateFlow("flow-2", "Flow 2", "")
		engine.CreateFlow("flow-3", "Flow 3", "")

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

	t.Run("should redeploy an already deployed flow", func(t *testing.T) {
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
		assert.True(t, engine.IsDeployed("duplicate-deploy"))

		// Deploying again replaces the running instance instead of failing.
		flow.Nodes["node-2"] = &Node{ID: "node-2", Type: "debug"}
		err = engine.Deploy(flow)
		require.NoError(t, err)
		assert.True(t, engine.IsDeployed("duplicate-deploy"))

		deployed, err := engine.GetFlow("duplicate-deploy")
		require.NoError(t, err)
		assert.Equal(t, FlowStatusActive, deployed.Status)
		assert.Len(t, deployed.Nodes, 2)
	})

	t.Run("should reject invalid flow IDs", func(t *testing.T) {
		engine := createTestEngine()

		flow := NewFlow("../etc/passwd", "Bad ID")
		flow.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}

		err := engine.Deploy(flow)
		assert.ErrorIs(t, err, ErrInvalidFlowID)

		_, err = engine.CreateFlow("a/b", "Bad ID", "")
		assert.ErrorIs(t, err, ErrInvalidFlowID)
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
				ID:         "conn-1",
				SourceNode: "node-1",
				TargetNode: "node-2",
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
	t.Run("should undeploy flow but keep its definition", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("undeploy-test", "Undeploy Test")
		flow.Nodes["node-1"] = &Node{
			ID:   "node-1",
			Type: "debug",
		}

		err := engine.Deploy(flow)
		require.NoError(t, err)
		assert.True(t, engine.IsDeployed("undeploy-test"))

		err = engine.Undeploy("undeploy-test")
		require.NoError(t, err)
		assert.False(t, engine.IsDeployed("undeploy-test"))

		// The definition is still known, just not running.
		kept, err := engine.GetFlow("undeploy-test")
		require.NoError(t, err)
		assert.Equal(t, FlowStatusInactive, kept.Status)
		assert.Len(t, engine.GetAllFlows(), 1)

		// Undeploying a known but idle flow is a no-op.
		assert.NoError(t, engine.Undeploy("undeploy-test"))

		// Injecting into an idle flow is refused.
		err = engine.InjectMessage("undeploy-test", "node-1", map[string]interface{}{"payload": 1})
		assert.ErrorIs(t, err, ErrFlowNotDeployed)
	})

	t.Run("should fail to undeploy non-existent flow", func(t *testing.T) {
		engine := createTestEngine()
		engine.Start()
		defer engine.Stop()

		err := engine.Undeploy("non-existent")
		assert.ErrorIs(t, err, ErrFlowNotFound)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFlowEngineCreateDeployLifecycle(t *testing.T) {
	engine := createTestEngine()
	engine.Start()
	defer engine.Stop()

	created, err := engine.CreateFlow("lifecycle", "Lifecycle", "created via API")
	require.NoError(t, err)
	assert.Equal(t, "created via API", created.Description)
	assert.Equal(t, FlowStatusInactive, created.Status)
	assert.False(t, engine.IsDeployed("lifecycle"))

	// A second create with the same ID is rejected.
	_, err = engine.CreateFlow("lifecycle", "Again", "")
	assert.ErrorIs(t, err, ErrFlowExists)

	// GetFlow returns a copy: mutating it must not touch the engine.
	created.Name = "mutated copy"
	again, err := engine.GetFlow("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, "Lifecycle", again.Name)

	// Edits go through UpdateFlow.
	updated, err := engine.UpdateFlow("lifecycle", func(f *Flow) error {
		return f.AddNode(&Node{ID: "node-1", Type: "debug"})
	})
	require.NoError(t, err)
	assert.Len(t, updated.Nodes, 1)

	// A flow created through the engine can be deployed by ID ...
	require.NoError(t, engine.DeployFlow("lifecycle"))
	assert.True(t, engine.IsDeployed("lifecycle"))
	status, err := engine.GetFlowStatus("lifecycle")
	require.NoError(t, err)
	assert.Equal(t, FlowStatusActive, status)

	// ... edited while running without affecting the runtime ...
	_, err = engine.UpdateFlow("lifecycle", func(f *Flow) error {
		return f.AddNode(&Node{ID: "node-2", Type: "debug"})
	})
	require.NoError(t, err)
	engine.mu.RLock()
	runtimeNodes := len(engine.active["lifecycle"].Flow.Nodes)
	engine.mu.RUnlock()
	assert.Equal(t, 1, runtimeNodes, "runtime keeps the deploy-time snapshot")

	// ... redeployed to pick up the edit ...
	require.NoError(t, engine.DeployFlow("lifecycle"))
	engine.mu.RLock()
	runtimeNodes = len(engine.active["lifecycle"].Flow.Nodes)
	engine.mu.RUnlock()
	assert.Equal(t, 2, runtimeNodes)

	// ... undeployed without panicking, and deleted.
	require.NoError(t, engine.Undeploy("lifecycle"))
	require.NoError(t, engine.DeleteFlow("lifecycle"))
	_, err = engine.GetFlow("lifecycle")
	assert.ErrorIs(t, err, ErrFlowNotFound)
	assert.Len(t, engine.GetAllFlows(), 0)

	assert.ErrorIs(t, engine.DeployFlow("missing"), ErrFlowNotFound)
}

// memoryStateManager is an in-memory StateManager for engine tests.
type memoryStateManager struct {
	flows map[string]*Flow
}

func newMemoryStateManager() *memoryStateManager {
	return &memoryStateManager{flows: make(map[string]*Flow)}
}

func (m *memoryStateManager) SaveFlow(flow *Flow) error {
	m.flows[flow.ID] = flow.Clone()
	return nil
}

func (m *memoryStateManager) LoadFlow(flowID string) (*Flow, error) {
	flow, ok := m.flows[flowID]
	if !ok {
		return nil, ErrFlowNotFound
	}
	return flow.Clone(), nil
}

func (m *memoryStateManager) LoadAllFlows() ([]*Flow, error) {
	flows := make([]*Flow, 0, len(m.flows))
	for _, flow := range m.flows {
		flows = append(flows, flow.Clone())
	}
	return flows, nil
}

func (m *memoryStateManager) DeleteFlow(flowID string) error {
	if _, ok := m.flows[flowID]; !ok {
		return ErrFlowNotFound
	}
	delete(m.flows, flowID)
	return nil
}

func TestFlowEngineLoadAllFlows(t *testing.T) {
	t.Run("should load every flow and deploy only those that were running", func(t *testing.T) {
		sm := newMemoryStateManager()

		running := NewFlow("was-running", "Was Running")
		running.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}
		running.Status = FlowStatusActive
		sm.SaveFlow(running)

		idle := NewFlow("was-idle", "Was Idle")
		idle.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}
		sm.SaveFlow(idle)

		broken := NewFlow("was-broken", "Was Broken")
		broken.Status = FlowStatusActive // no nodes: fails validation on deploy
		sm.SaveFlow(broken)

		engine := createTestEngine()
		engine.SetStateManager(sm)
		engine.Start()
		defer engine.Stop()

		require.NoError(t, engine.LoadAllFlows())

		assert.Len(t, engine.GetAllFlows(), 3, "every persisted flow is known")
		assert.True(t, engine.IsDeployed("was-running"))
		assert.False(t, engine.IsDeployed("was-idle"))
		assert.False(t, engine.IsDeployed("was-broken"))

		status, _ := engine.GetFlowStatus("was-broken")
		assert.Equal(t, FlowStatusError, status)

		// Status changes are persisted so the next start restores them.
		require.NoError(t, engine.Undeploy("was-running"))
		assert.Equal(t, FlowStatusInactive, sm.flows["was-running"].Status)
		require.NoError(t, engine.DeployFlow("was-idle"))
		assert.Equal(t, FlowStatusActive, sm.flows["was-idle"].Status)
	})

	t.Run("should fail without a state manager", func(t *testing.T) {
		engine := createTestEngine()
		assert.Error(t, engine.LoadAllFlows())
	})
}
