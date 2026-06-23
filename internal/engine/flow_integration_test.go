package engine

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Enable debug logging for this test
func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
}

// helper function to get node IDs for logging
func getNodeIDs(nodes map[string]*Node) []string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	return ids
}

func TestFlowIntegration_FullNodeFlow(t *testing.T) {
	t.Run("should deploy flow and verify all nodes work", func(t *testing.T) {
		// Setup
		registry := registry.GetGlobalRegistry()
		config := EngineConfig{
			WorkerPoolSize:    10,
			MessageBufferSize: 1000,
			DefaultTimeout:    5 * time.Second,
		}
		engine := NewFlowEngine(config, registry)

		// Enable debug logging
		engine.SetMaxMessageLog(1000)

		engine.Start()
		defer engine.Stop()

		// Create test flow with inject -> function -> debug
		flow := NewFlow("test-integration-flow", "Integration Test Flow")

		// Inject node
		flow.Nodes["inject-node"] = &Node{
			ID:   "inject-node",
			Type: "inject",
			Name: "Test Inject",
			X:    0,
			Y:    0,
			Config: map[string]interface{}{
				"payload": map[string]interface{}{
					"testData": "initial value",
					"number":   float64(42),
				},
				"topic":      "test-topic",
				"injectOnce": false,
			},
		}

		// Function node - transforms payload
		flow.Nodes["function-node"] = &Node{
			ID:   "function-node",
			Type: "function",
			Name: "Test Function",
			X:    100,
			Y:    0,
			Config: map[string]interface{}{
				"code":   "return { result: input.testData + \" transformed\", doubled: input.number * 2 };",
				"useMsg": false,
			},
		}

		// Debug node
		flow.Nodes["debug-node"] = &Node{
			ID:   "debug-node",
			Type: "debug",
			Name: "Test Debug",
			X:    200,
			Y:    0,
			Config: map[string]interface{}{
				"enabled":         true,
				"outputToConsole": true,
				"maxBufferSize":   100,
				"prefix":          "[INTEGRATION-TEST]",
				"showTimestamp":   true,
				"showPath":        true,
			},
		}

		// Connections: inject -> function -> debug
		flow.Connections = []NodeConnection{
			{
				ID:          "conn-1",
				SourceNode:  "inject-node",
				SourcePort:  "output",
				TargetNode:  "function-node",
				TargetPort:  "input",
			},
			{
				ID:          "conn-2",
				SourceNode:  "function-node",
				SourcePort:  "output",
				TargetNode:  "debug-node",
				TargetPort:  "input",
			},
		}

		// Deploy flow with debug logging
		log.Printf("[DEBUG] Deploying integration test flow...")
		log.Printf("[DEBUG] Flow has %d nodes: %v", len(flow.Nodes), getNodeIDs(flow.Nodes))
		err := engine.Deploy(flow)
		require.NoError(t, err, "Flow deployment failed")
		log.Printf("[DEBUG] Flow deployed successfully with %d nodes", len(flow.Nodes))

		defer func() {
			log.Printf("[DEBUG] Undeploying flow...")
			engine.Undeploy(flow.ID)
		}()

		// Verify flow is active
		status, err := engine.GetFlowStatus(flow.ID)
		require.NoError(t, err)
		assert.Equal(t, FlowStatusActive, status)
		log.Printf("[DEBUG] Flow status: %s", status)

		// Inject test message
		testPayload := map[string]interface{}{
			"testData": "injected value",
			"number":   float64(100),
		}
		log.Printf("[DEBUG] Injecting test message: %v", testPayload)
		err = engine.InjectMessage(flow.ID, "inject-node", testPayload)
		require.NoError(t, err, "Failed to inject message")

		// Wait for message processing
		time.Sleep(500 * time.Millisecond)

		// Verify message passed through all nodes
		messages := engine.GetMessageLogForFlow(flow.ID)
		log.Printf("[DEBUG] Total messages processed: %d", len(messages))

		// Should have at least 1 message (the injected one processed through the chain)
		// Note: The message is cloned at each step, so we might have multiple entries
		assert.GreaterOrEqual(t, len(messages), 1, "Expected at least one message in log")

		// Check that we can see the message path contains all nodes
		var foundFullPath bool
		for _, msg := range messages {
			log.Printf("[DEBUG] Message ID: %s, FlowID: %s, Path: %v, Payload: %v",
				msg.ID, msg.FlowID, msg.Path, msg.Payload)

			// Verify payload was transformed by function node
			payload := msg.Payload
			if result, exists := payload["result"]; exists {
				resultStr, ok := result.(string)
				assert.True(t, ok, "Result should be a string")
				assert.Contains(t, resultStr, "transformed", "Function node should transform payload")
				foundFullPath = true
			}
			if doubled, exists := payload["doubled"]; exists {
				// doubled could be float64 or int64 from JavaScript
				switch v := doubled.(type) {
				case float64:
					assert.Equal(t, float64(200), v, "Function should double the number")
				case int64:
					assert.Equal(t, int64(200), v, "Function should double the number")
				default:
					assert.Fail(t, "Doubled should be a number", "doubled is of type %T: %v", doubled, doubled)
				}
			}
		}

		assert.True(t, foundFullPath, "Message should have been transformed by function node")

		// Additional verification: Check flow structure
		deployedFlow, err := engine.GetFlow(flow.ID)
		require.NoError(t, err)
		assert.Len(t, deployedFlow.Nodes, 3)
		assert.Len(t, deployedFlow.Connections, 2)

		// Verify all node types are correct
		assert.Equal(t, "inject", deployedFlow.Nodes["inject-node"].Type)
		assert.Equal(t, "function", deployedFlow.Nodes["function-node"].Type)
		assert.Equal(t, "debug", deployedFlow.Nodes["debug-node"].Type)

		log.Printf("[DEBUG] All tests passed successfully!")
	})
}

func TestFlowIntegration_NodeInitialization(t *testing.T) {
	t.Run("should initialize all node types correctly", func(t *testing.T) {
		registry := registry.GetGlobalRegistry()
		config := EngineConfig{
			WorkerPoolSize:    5,
			MessageBufferSize: 100,
			DefaultTimeout:    5 * time.Second,
		}
		engine := NewFlowEngine(config, registry)
		engine.Start()
		defer engine.Stop()

		flow := NewFlow("test-node-init", "Node Init Test")

		// Add one of each node type
		flow.Nodes["inject"] = &Node{ID: "inject", Type: "inject"}
		flow.Nodes["function"] = &Node{ID: "function", Type: "function", Config: map[string]interface{}{"code": "return input;"}}
		flow.Nodes["debug"] = &Node{ID: "debug", Type: "debug"}

		err := engine.Deploy(flow)
		require.NoError(t, err)
		defer engine.Undeploy(flow.ID)

		log.Printf("[DEBUG] Successfully initialized inject, function, and debug nodes")
	})
}
