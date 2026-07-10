package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/engine"
    "github.com/GrimbiXcode/Go-RED/internal/registry"
    _ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
    _ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
    _ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// MockStateManager implements StateManager interface for testing
type MockStateManager struct {
    flows map[string]*engine.Flow
}

func NewMockStateManager() *MockStateManager {
    return &MockStateManager{
        flows: make(map[string]*engine.Flow),
    }
}

func (m *MockStateManager) SaveFlow(flow *engine.Flow) error {
    m.flows[flow.ID] = flow
    return nil
}

func (m *MockStateManager) LoadFlow(flowID string) (*engine.Flow, error) {
    flow, exists := m.flows[flowID]
    if !exists {
        return nil, &httpPathError{path: flowID, err: "not found"}
    }
    // Return a copy to avoid mutation
    return flow.Clone(), nil
}

func (m *MockStateManager) LoadAllFlows() ([]*engine.Flow, error) {
    flows := make([]*engine.Flow, 0, len(m.flows))
    for _, flow := range m.flows {
        flows = append(flows, flow.Clone())
    }
    return flows, nil
}

func (m *MockStateManager) DeleteFlow(flowID string) error {
    delete(m.flows, flowID)
    return nil
}

// httpPathError is a simple error type for testing
type httpPathError struct {
    path string
    err  string
}

func (e *httpPathError) Error() string {
    return e.path + ": " + e.err
}

func createTestFlowEngine() *engine.FlowEngine {
    config := engine.EngineConfig{
        WorkerPoolSize:    10,
        MessageBufferSize: 1000,
        DefaultTimeout:    30,
        MaxRetries:        3,
        RetryBackoff:      1,
    }
    // Use the global registry which has built-in nodes registered via init()
    nodeRegistry := registry.GetGlobalRegistry()
    
    engine := engine.NewFlowEngine(config, nodeRegistry)
    mockSM := NewMockStateManager()
    engine.SetStateManager(mockSM)
    
    return engine
}

// handleDeployFlowWithID calls the real handleDeployFlow with the given
// flow ID set as the request's path value, exactly as the router would
// for a request to /api/flows/{id}/deploy. A hand-duplicated copy of the
// handler previously lived here and quietly drifted out of sync with the
// real one in main.go (it still had the old "already deployed" no-op
// bug after that was fixed) - calling the real handler is the only way
// these tests can catch a regression there again.
func handleDeployFlowWithID(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine, flowID string) {
    if r == nil {
        r = httptest.NewRequest(http.MethodPost, "/api/flows/"+flowID+"/deploy", nil)
    }
    r.SetPathValue("id", flowID)
    handleDeployFlow(w, r, e)
}

func TestHandleDeployFlow(t *testing.T) {
    t.Run("should deploy flow not in memory", func(t *testing.T) {
        e := createTestFlowEngine()
        e.Start()
        defer e.Stop()

        // Create a flow in the state manager
        flow := engine.NewFlow("test-flow-1", "Test Flow 1")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
            X:    0,
            Y:    0,
        }
        mockSM := e.GetStateManager().(*MockStateManager)
        mockSM.SaveFlow(flow)

        // Create request
        w := httptest.NewRecorder()

        // Call handler with explicit flow ID
        handleDeployFlowWithID(w, nil, e, "test-flow-1")

        // Verify response
        assert.Equal(t, http.StatusOK, w.Code)
        assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

        var response map[string]interface{}
        err := json.Unmarshal(w.Body.Bytes(), &response)
        require.NoError(t, err)
        assert.Equal(t, "running", response["status"])
        assert.Equal(t, "test-flow-1", response["flowId"])

        // Verify flow is now deployed
        deployedFlow, err := e.GetFlow("test-flow-1")
        require.NoError(t, err)
        assert.Equal(t, engine.FlowStatusActive, deployedFlow.Status)
    })

    t.Run("should stop and redeploy an already-running flow with its latest definition", func(t *testing.T) {
        e := createTestFlowEngine()
        e.Start()
        defer e.Stop()

        // Deploy a flow with a single node.
        flow := engine.NewFlow("test-flow-2", "Test Flow 2")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
        }
        err := e.Deploy(flow)
        require.NoError(t, err)

        // Simulate editing the flow while it's running (e.g. dragging a
        // node, or any node:add/node:update over the WebSocket): those
        // handlers fetch the engine's live *Flow via GetFlow (the same
        // pointer activeFlow.Flow holds), mutate it in place, then save
        // it - so reproduce that exact pattern here rather than saving an
        // unrelated cloned object, which wouldn't exercise the same code
        // path the real handlers do.
        liveFlow, err := e.GetFlow("test-flow-2")
        require.NoError(t, err)
        liveFlow.Nodes["node-2"] = &engine.Node{
            ID:   "node-2",
            Type: "debug",
        }
        liveFlow.Connections = []engine.NodeConnection{
            {ID: "conn-1", SourceNode: "node-1", TargetNode: "node-2"},
        }
        mockSM := e.GetStateManager().(*MockStateManager)
        require.NoError(t, mockSM.SaveFlow(liveFlow))

        // Deploying again while still "running" must actually pick up
        // that edit - not silently no-op with a fake "already deployed"
        // response, which previously left the new node without an
        // executor so messages routed to it vanished with a "node not
        // found in flow" error nobody surfaced to the user.
        w := httptest.NewRecorder()
        handleDeployFlowWithID(w, nil, e, "test-flow-2")

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        err = json.Unmarshal(w.Body.Bytes(), &response)
        require.NoError(t, err)
        assert.Equal(t, "running", response["status"])
        assert.Equal(t, "test-flow-2", response["flowId"])

        redeployed, err := e.GetFlow("test-flow-2")
        require.NoError(t, err)
        assert.Len(t, redeployed.Nodes, 2, "redeploy must pick up the node added while the flow was running")
        assert.Len(t, redeployed.Connections, 1)
    })

    t.Run("should return 404 for non-existent flow", func(t *testing.T) {
        e := createTestFlowEngine()
        e.Start()
        defer e.Stop()

        // Try to deploy a flow that doesn't exist
        w := httptest.NewRecorder()

        handleDeployFlowWithID(w, nil, e, "non-existent")

        assert.Equal(t, http.StatusNotFound, w.Code)
    })

    t.Run("should load from state manager and deploy", func(t *testing.T) {
        e := createTestFlowEngine()
        e.Start()
        defer e.Stop()

        // Create a flow in state manager but not in engine memory
        flow := engine.NewFlow("test-flow-3", "Test Flow 3")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
        }
        flow.Nodes["node-2"] = &engine.Node{
            ID:   "node-2",
            Type: "debug",
        }
        flow.Connections = []engine.NodeConnection{
            {
                ID:          "conn-1",
                SourceNode:  "node-1",
                TargetNode:  "node-2",
            },
        }
        mockSM := e.GetStateManager().(*MockStateManager)
        mockSM.SaveFlow(flow)

        // Flow should not be in engine yet
        _, err := e.GetFlow("test-flow-3")
        assert.Error(t, err)

        // Deploy via REST API
        w := httptest.NewRecorder()

        handleDeployFlowWithID(w, nil, e, "test-flow-3")

        assert.Equal(t, http.StatusOK, w.Code)

        var response map[string]interface{}
        err = json.Unmarshal(w.Body.Bytes(), &response)
        require.NoError(t, err)
        assert.Equal(t, "running", response["status"])

        // Verify flow is now deployed with correct node and connection counts
        deployedFlow, err := e.GetFlow("test-flow-3")
        require.NoError(t, err)
        assert.Len(t, deployedFlow.Nodes, 2)
        assert.Len(t, deployedFlow.Connections, 1)
    })
}

func TestHandleDeployFlow_ValidationErrors(t *testing.T) {
    t.Run("should return 500 for invalid flow in state manager", func(t *testing.T) {
        e := createTestFlowEngine()
        e.Start()
        defer e.Stop()

        // Create an invalid flow (no nodes) in state manager
        flow := engine.NewFlow("invalid-flow", "Invalid Flow")
        // Don't add any nodes - will fail validation
        mockSM := e.GetStateManager().(*MockStateManager)
        mockSM.SaveFlow(flow)

        // Try to deploy
        w := httptest.NewRecorder()

        handleDeployFlowWithID(w, nil, e, "invalid-flow")

        // Should return 500 because deployment fails
        assert.Equal(t, http.StatusInternalServerError, w.Code)
    })

    t.Run("should return 500 for flow with invalid connections", func(t *testing.T) {
        e := createTestFlowEngine()
        e.Start()
        defer e.Stop()

        // Create a flow with invalid connection (references non-existent node)
        flow := engine.NewFlow("invalid-conn-flow", "Invalid Connection Flow")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
        }
        // Connection references non-existent node
        flow.Connections = []engine.NodeConnection{
            {
                ID:          "conn-1",
                SourceNode:  "node-1",
                TargetNode:  "node-does-not-exist",
            },
        }
        mockSM := e.GetStateManager().(*MockStateManager)
        mockSM.SaveFlow(flow)

        // Try to deploy
        w := httptest.NewRecorder()

        handleDeployFlowWithID(w, nil, e, "invalid-conn-flow")

        // Should return 500 because validation fails
        assert.Equal(t, http.StatusInternalServerError, w.Code)
    })
}

func TestFlowValidation(t *testing.T) {
    t.Run("should pass validation with valid nodes and connections", func(t *testing.T) {
        flow := engine.NewFlow("valid-flow", "Valid Flow")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
        }
        flow.Nodes["node-2"] = &engine.Node{
            ID:   "node-2",
            Type: "debug",
        }
        flow.Connections = []engine.NodeConnection{
            {
                ID:          "conn-1",
                SourceNode:  "node-1",
                TargetNode:  "node-2",
            },
        }

        err := flow.Validate()
        assert.NoError(t, err)
    })

    t.Run("should fail validation with no nodes", func(t *testing.T) {
        flow := engine.NewFlow("no-nodes-flow", "No Nodes Flow")

        err := flow.Validate()
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "at least one node")
    })

    t.Run("should fail validation with connection to non-existent node", func(t *testing.T) {
        flow := engine.NewFlow("bad-conn-flow", "Bad Connection Flow")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
        }
        flow.Connections = []engine.NodeConnection{
            {
                ID:          "conn-1",
                SourceNode:  "node-1",
                TargetNode:  "node-does-not-exist",
            },
        }

        err := flow.Validate()
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "non-existent")
    })

    t.Run("should fail validation with connection from non-existent node", func(t *testing.T) {
        flow := engine.NewFlow("bad-source-flow", "Bad Source Flow")
        flow.Nodes["node-1"] = &engine.Node{
            ID:   "node-1",
            Type: "debug",
        }
        flow.Connections = []engine.NodeConnection{
            {
                ID:          "conn-1",
                SourceNode:  "node-does-not-exist",
                TargetNode:  "node-1",
            },
        }

        err := flow.Validate()
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "non-existent")
    })
}

// Ensure StateManager interface is satisfied
var _ engine.StateManager = (*MockStateManager)(nil)
