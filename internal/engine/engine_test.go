package engine

import (
    "errors"
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

        // The flow must stay visible (as inactive) after undeploy - it's
        // still the same draft the user can edit and redeploy, not a
        // deleted flow. Only DeleteFlow actually removes it.
        undeployed, err := engine.GetFlow("undeploy-test")
        require.NoError(t, err)
        assert.Equal(t, FlowStatusInactive, undeployed.Status)

        status, err := engine.GetFlowStatus("undeploy-test")
        require.NoError(t, err)
        assert.Equal(t, FlowStatusInactive, status)
    })

    t.Run("should fail to undeploy non-existent flow", func(t *testing.T) {
        engine := createTestEngine()
        engine.Start()
        defer engine.Stop()

        err := engine.Undeploy("non-existent")
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "not found")
    })

    t.Run("should allow redeploying a flow after it was undeployed", func(t *testing.T) {
        engine := createTestEngine()
        engine.Start()
        defer engine.Stop()

        flow := NewFlow("redeploy-test", "Redeploy Test")
        flow.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}

        require.NoError(t, engine.Deploy(flow))
        require.NoError(t, engine.Undeploy("redeploy-test"))

        // Deploying the same flow again must succeed, not fail with
        // "already deployed" - Undeploy leaves an inactive placeholder
        // behind (so the flow stays visible), and Deploy must recognize
        // that placeholder isn't a running flow.
        err := engine.Deploy(flow)
        require.NoError(t, err)

        status, err := engine.GetFlowStatus("redeploy-test")
        require.NoError(t, err)
        assert.Equal(t, FlowStatusActive, status)
    })
}

func TestFlowEngineDeleteFlow(t *testing.T) {
    t.Run("should remove a running flow entirely", func(t *testing.T) {
        engine := createTestEngine()
        engine.Start()
        defer engine.Stop()

        flow := NewFlow("delete-test", "Delete Test")
        flow.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}
        require.NoError(t, engine.Deploy(flow))

        require.NoError(t, engine.DeleteFlow("delete-test"))

        _, err := engine.GetFlow("delete-test")
        assert.Error(t, err)
    })

    t.Run("should remove a never-deployed draft without panicking", func(t *testing.T) {
        engine := createTestEngine()
        engine.Start()
        defer engine.Stop()

        _, err := engine.CreateFlow("draft-delete-test", "Draft Delete Test")
        require.NoError(t, err)

        require.NoError(t, engine.DeleteFlow("draft-delete-test"))

        _, err = engine.GetFlow("draft-delete-test")
        assert.Error(t, err)
    })
}

// inMemoryStateManager is a minimal StateManager for exercising
// LoadAllFlows' handling of persisted status without touching disk.
type inMemoryStateManager struct {
    flows map[string]*Flow
}

func newInMemoryStateManager() *inMemoryStateManager {
    return &inMemoryStateManager{flows: make(map[string]*Flow)}
}

func (m *inMemoryStateManager) SaveFlow(flow *Flow) error {
    m.flows[flow.ID] = flow
    return nil
}

func (m *inMemoryStateManager) LoadFlow(flowID string) (*Flow, error) {
    flow, ok := m.flows[flowID]
    if !ok {
        return nil, errors.New("flow not found")
    }
    return flow, nil
}

func (m *inMemoryStateManager) LoadAllFlows() ([]*Flow, error) {
    flows := make([]*Flow, 0, len(m.flows))
    for _, f := range m.flows {
        flows = append(flows, f)
    }
    return flows, nil
}

func (m *inMemoryStateManager) DeleteFlow(flowID string) error {
    delete(m.flows, flowID)
    return nil
}

func TestFlowEngineLoadAllFlows(t *testing.T) {
    t.Run("should not redeploy a draft that was never deployed", func(t *testing.T) {
        engine := createTestEngine()
        sm := newInMemoryStateManager()
        engine.SetStateManager(sm)
        engine.Start()
        defer engine.Stop()

        draft := NewFlow("never-deployed-draft", "Never Deployed Draft")
        draft.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}
        require.NoError(t, sm.SaveFlow(draft))

        require.NoError(t, engine.LoadAllFlows())

        status, err := engine.GetFlowStatus("never-deployed-draft")
        require.NoError(t, err)
        assert.Equal(t, FlowStatusInactive, status, "a draft must stay a draft across a restart")
    })

    t.Run("should redeploy a flow that was genuinely running", func(t *testing.T) {
        engine := createTestEngine()
        sm := newInMemoryStateManager()
        engine.SetStateManager(sm)
        engine.Start()
        defer engine.Stop()

        running := NewFlow("was-running", "Was Running")
        running.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}
        running.Status = FlowStatusActive
        require.NoError(t, sm.SaveFlow(running))

        require.NoError(t, engine.LoadAllFlows())

        status, err := engine.GetFlowStatus("was-running")
        require.NoError(t, err)
        assert.Equal(t, FlowStatusActive, status, "a flow that was running must come back running")
    })

    t.Run("should not redeploy a flow that was explicitly stopped", func(t *testing.T) {
        engine := createTestEngine()
        sm := newInMemoryStateManager()
        engine.SetStateManager(sm)
        engine.Start()

        flow := NewFlow("stop-then-restart", "Stop Then Restart")
        flow.Nodes["node-1"] = &Node{ID: "node-1", Type: "debug"}
        require.NoError(t, engine.Deploy(flow))
        require.NoError(t, engine.Undeploy("stop-then-restart"))
        engine.Stop()

        // Simulate a restart against the same persisted state.
        restarted := createTestEngine()
        restarted.SetStateManager(sm)
        restarted.Start()
        defer restarted.Stop()

        require.NoError(t, restarted.LoadAllFlows())

        status, err := restarted.GetFlowStatus("stop-then-restart")
        require.NoError(t, err)
        assert.Equal(t, FlowStatusInactive, status, "Stop must survive a restart instead of the flow silently starting again")
    })
}


