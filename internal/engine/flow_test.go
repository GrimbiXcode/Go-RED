package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeCreation(t *testing.T) {
	t.Run("should create node with all fields", func(t *testing.T) {
		node := &Node{
			ID:       "test-node-1",
			Type:     "function",
			Name:     "Test Node",
			Config:   map[string]interface{}{"key": "value"},
			X:        100.5,
			Y:        200.5,
			Disabled: false,
		}

		assert.Equal(t, "test-node-1", node.ID)
		assert.Equal(t, "function", node.Type)
		assert.Equal(t, "Test Node", node.Name)
		assert.Equal(t, float64(100.5), node.X)
		assert.Equal(t, float64(200.5), node.Y)
		assert.False(t, node.Disabled)
		assert.Equal(t, "value", node.Config["key"])
	})

	t.Run("should create node with minimal fields", func(t *testing.T) {
		node := &Node{
			ID:   "minimal-node",
			Type: "inject",
		}

		assert.Equal(t, "minimal-node", node.ID)
		assert.Equal(t, "inject", node.Type)
		assert.Empty(t, node.Name)
		assert.Equal(t, float64(0), node.X)
		assert.Equal(t, float64(0), node.Y)
	})
}

func TestNodeConnectionCreation(t *testing.T) {
	t.Run("should create connection with all fields", func(t *testing.T) {
		conn := NodeConnection{
			ID:          "conn-1",
			SourceNode:  "node-1",
			SourcePort:  "output",
			TargetNode:  "node-2",
			TargetPort:  "input",
		}

		assert.Equal(t, "conn-1", conn.ID)
		assert.Equal(t, "node-1", conn.SourceNode)
		assert.Equal(t, "output", conn.SourcePort)
		assert.Equal(t, "node-2", conn.TargetNode)
		assert.Equal(t, "input", conn.TargetPort)
	})

	t.Run("should create connection with minimal fields", func(t *testing.T) {
		conn := NodeConnection{
			SourceNode: "node-1",
			TargetNode: "node-2",
		}

		assert.Empty(t, conn.ID)
		assert.Equal(t, "node-1", conn.SourceNode)
		assert.Equal(t, "node-2", conn.TargetNode)
		assert.Empty(t, conn.SourcePort)
		assert.Empty(t, conn.TargetPort)
	})
}

func TestFlowCreation(t *testing.T) {
	t.Run("should create flow with default values", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")

		assert.Equal(t, "flow-1", flow.ID)
		assert.Equal(t, "Test Flow", flow.Name)
		assert.Empty(t, flow.Description)
		assert.NotNil(t, flow.Nodes)
		assert.Len(t, flow.Nodes, 0)
		assert.NotNil(t, flow.Connections)
		assert.Len(t, flow.Connections, 0)
		assert.Equal(t, FlowStatusInactive, flow.Status)
		assert.NotZero(t, flow.CreatedAt)
		assert.NotZero(t, flow.UpdatedAt)
		assert.Equal(t, "1.0", flow.Version)

		// Check default config
		assert.Equal(t, 30*time.Second, flow.Config.Timeout)
		assert.Equal(t, 100, flow.Config.MaxConcurrency)
		assert.Equal(t, 3, flow.Config.RetryPolicy.MaxRetries)
		assert.Equal(t, 1*time.Second, flow.Config.RetryPolicy.Backoff)
		assert.Equal(t, 30*time.Second, flow.Config.RetryPolicy.MaxBackoff)
	})
}

func TestFlowAddNode(t *testing.T) {
	t.Run("should add node to flow", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node := &Node{
			ID:   "node-1",
			Type: "function",
			X:    100,
			Y:    200,
		}

		err := flow.AddNode(node)
		require.NoError(t, err)
		assert.Len(t, flow.Nodes, 1)
		assert.Contains(t, flow.Nodes, "node-1")
		assert.Equal(t, node, flow.Nodes["node-1"])
	})

	t.Run("should fail to add node with empty ID", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node := &Node{
			ID:   "",
			Type: "function",
		}

		err := flow.AddNode(node)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "node ID cannot be empty")
		assert.Len(t, flow.Nodes, 0)
	})

	t.Run("should fail to add duplicate node", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-1", Type: "inject"}

		err := flow.AddNode(node1)
		require.NoError(t, err)

		err = flow.AddNode(node2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Len(t, flow.Nodes, 1)
	})
}

func TestFlowRemoveNode(t *testing.T) {
	t.Run("should remove node from flow", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-2", Type: "inject"}

		flow.AddNode(node1)
		flow.AddNode(node2)

		// Add a connection between the nodes
		conn := NodeConnection{
			ID:          "conn-1",
			SourceNode:  "node-1",
			TargetNode:  "node-2",
			SourcePort:  "output",
			TargetPort:  "input",
		}
		flow.AddConnection(conn)
		assert.Len(t, flow.Connections, 1)

		err := flow.RemoveNode("node-1")
		require.NoError(t, err)
		assert.Len(t, flow.Nodes, 1)
		assert.NotContains(t, flow.Nodes, "node-1")
		assert.Contains(t, flow.Nodes, "node-2")
		// Connection should also be removed
		assert.Len(t, flow.Connections, 0)
	})

	t.Run("should fail to remove non-existent node", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")

		err := flow.RemoveNode("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFlowAddConnection(t *testing.T) {
	t.Run("should add connection between existing nodes", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-2", Type: "debug"}

		flow.AddNode(node1)
		flow.AddNode(node2)

		conn := NodeConnection{
			ID:          "conn-1",
			SourceNode:  "node-1",
			SourcePort:  "output",
			TargetNode:  "node-2",
			TargetPort:  "input",
		}

		err := flow.AddConnection(conn)
		require.NoError(t, err)
		assert.Len(t, flow.Connections, 1)
		assert.Equal(t, conn, flow.Connections[0])
	})

	t.Run("should fail to add connection with non-existent source node", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node := &Node{ID: "node-1", Type: "function"}
		flow.AddNode(node)

		conn := NodeConnection{
			SourceNode: "non-existent",
			TargetNode: "node-1",
		}

		err := flow.AddConnection(conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source node")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("should fail to add connection with non-existent target node", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node := &Node{ID: "node-1", Type: "function"}
		flow.AddNode(node)

		conn := NodeConnection{
			SourceNode: "node-1",
			TargetNode: "non-existent",
		}

		err := flow.AddConnection(conn)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "target node")
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("should fail to add duplicate connection", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-2", Type: "debug"}
		flow.AddNode(node1)
		flow.AddNode(node2)

		conn1 := NodeConnection{
			SourceNode: "node-1",
			TargetNode: "node-2",
			SourcePort: "output",
			TargetPort: "input",
		}
		conn2 := NodeConnection{
			SourceNode: "node-1",
			TargetNode: "node-2",
			SourcePort: "output",
			TargetPort: "input",
		}

		flow.AddConnection(conn1)
		err := flow.AddConnection(conn2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
		assert.Len(t, flow.Connections, 1)
	})

	t.Run("should generate ID for connection without ID", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-2", Type: "debug"}
		flow.AddNode(node1)
		flow.AddNode(node2)

		conn := NodeConnection{
			SourceNode: "node-1",
			TargetNode: "node-2",
		}

		err := flow.AddConnection(conn)
		require.NoError(t, err)
		assert.Len(t, flow.Connections, 1)
		assert.NotEmpty(t, flow.Connections[0].ID)
		assert.Contains(t, flow.Connections[0].ID, "conn-")
	})
}

func TestFlowRemoveConnection(t *testing.T) {
	t.Run("should remove connection by ID", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-2", Type: "debug"}
		flow.AddNode(node1)
		flow.AddNode(node2)

		conn := NodeConnection{
			ID:          "conn-1",
			SourceNode:  "node-1",
			TargetNode:  "node-2",
		}
		flow.AddConnection(conn)
		assert.Len(t, flow.Connections, 1)

		err := flow.RemoveConnection("conn-1")
		require.NoError(t, err)
		assert.Len(t, flow.Connections, 0)
	})

	t.Run("should fail to remove non-existent connection", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")

		err := flow.RemoveConnection("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestFlowGetConnectionsForNode(t *testing.T) {
	flow := NewFlow("flow-1", "Test Flow")
	node1 := &Node{ID: "node-1", Type: "function"}
	node2 := &Node{ID: "node-2", Type: "debug"}
	node3 := &Node{ID: "node-3", Type: "inject"}
	flow.AddNode(node1)
	flow.AddNode(node2)
	flow.AddNode(node3)

	// Add connections: node-1 -> node-2, node-3 -> node-2
	conn1 := NodeConnection{
		ID:         "conn-1",
		SourceNode: "node-1",
		TargetNode: "node-2",
	}
	conn2 := NodeConnection{
		ID:         "conn-2",
		SourceNode: "node-3",
		TargetNode: "node-2",
	}
	flow.AddConnection(conn1)
	flow.AddConnection(conn2)

	t.Run("should get all connections for a node", func(t *testing.T) {
		connections := flow.GetConnectionsForNode("node-2")
		assert.Len(t, connections, 2)
	})

	t.Run("should get connections where node is source", func(t *testing.T) {
		connections := flow.GetSourceConnections("node-1")
		assert.Len(t, connections, 1)
		assert.Equal(t, "conn-1", connections[0].ID)
	})

	t.Run("should get connections where node is target", func(t *testing.T) {
		connections := flow.GetTargetConnections("node-2")
		assert.Len(t, connections, 2)
	})

	t.Run("should return empty for node with no connections", func(t *testing.T) {
		flow2 := NewFlow("flow-2", "Test Flow 2")
		node := &Node{ID: "isolated", Type: "function"}
		flow2.AddNode(node)

		connections := flow2.GetConnectionsForNode("isolated")
		assert.Len(t, connections, 0)
	})
}

func TestFlowValidate(t *testing.T) {
	t.Run("should validate flow with nodes and connections", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node1 := &Node{ID: "node-1", Type: "function"}
		node2 := &Node{ID: "node-2", Type: "debug"}
		flow.AddNode(node1)
		flow.AddNode(node2)

		conn := NodeConnection{
			SourceNode: "node-1",
			TargetNode: "node-2",
		}
		flow.AddConnection(conn)

		err := flow.Validate()
		assert.NoError(t, err)
	})

	t.Run("should fail validation for empty flow", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")

		err := flow.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one node")
	})

	t.Run("should fail validation for connection with non-existent source", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node := &Node{ID: "node-1", Type: "function"}
		flow.AddNode(node)

		// Create connection manually (bypassing AddConnection validation)
		flow.Connections = append(flow.Connections, NodeConnection{
			SourceNode: "non-existent",
			TargetNode: "node-1",
		})

		err := flow.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-existent source node")
	})

	t.Run("should fail validation for connection with non-existent target", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		node := &Node{ID: "node-1", Type: "function"}
		flow.AddNode(node)

		flow.Connections = append(flow.Connections, NodeConnection{
			SourceNode: "node-1",
			TargetNode: "non-existent",
		})

		err := flow.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-existent target node")
	})
}

func TestFlowClone(t *testing.T) {
	t.Run("should create deep copy of flow", func(t *testing.T) {
		flow := NewFlow("flow-1", "Test Flow")
		flow.Description = "Original description"

		node1 := &Node{
			ID:       "node-1",
			Type:     "function",
			Name:     "Node 1",
			X:        100,
			Y:        200,
			Config:   map[string]interface{}{"key": "value"},
			Disabled: true,
		}
		node2 := &Node{
			ID:   "node-2",
			Type: "debug",
		}
		flow.AddNode(node1)
		flow.AddNode(node2)

		conn := NodeConnection{
			ID:          "conn-1",
			SourceNode:  "node-1",
			SourcePort:  "output",
			TargetNode:  "node-2",
			TargetPort:  "input",
		}
		flow.AddConnection(conn)

		cloned := flow.Clone()

		// Verify basic fields
		assert.Equal(t, flow.ID, cloned.ID)
		assert.Equal(t, flow.Name, cloned.Name)
		assert.Equal(t, flow.Description, cloned.Description)
		assert.Equal(t, flow.Status, cloned.Status)

		// Verify nodes are cloned
		assert.Len(t, cloned.Nodes, 2)
		assert.Contains(t, cloned.Nodes, "node-1")
		assert.Contains(t, cloned.Nodes, "node-2")

		// Verify node details
		clonedNode1 := cloned.Nodes["node-1"]
		assert.Equal(t, node1.ID, clonedNode1.ID)
		assert.Equal(t, node1.Type, clonedNode1.Type)
		assert.Equal(t, node1.Name, clonedNode1.Name)
		assert.Equal(t, node1.X, clonedNode1.X)
		assert.Equal(t, node1.Y, clonedNode1.Y)
		assert.Equal(t, node1.Disabled, clonedNode1.Disabled)
		assert.Equal(t, node1.Config, clonedNode1.Config)

		// Verify connections are cloned
		assert.Len(t, cloned.Connections, 1)
		assert.Equal(t, conn.ID, cloned.Connections[0].ID)
		assert.Equal(t, conn.SourceNode, cloned.Connections[0].SourceNode)
		assert.Equal(t, conn.TargetNode, cloned.Connections[0].TargetNode)

		// Verify modification of clone doesn't affect original
		cloned.Nodes["node-1"].X = 999
		assert.Equal(t, float64(100), node1.X)
		assert.Equal(t, float64(999), cloned.Nodes["node-1"].X)
	})
}

func TestFlowStatus(t *testing.T) {
	assert.Equal(t, FlowStatus("inactive"), FlowStatusInactive)
	assert.Equal(t, FlowStatus("active"), FlowStatusActive)
	assert.Equal(t, FlowStatus("error"), FlowStatusError)
	assert.Equal(t, FlowStatus("deploying"), FlowStatusDeploying)
	assert.Equal(t, FlowStatus("undeploying"), FlowStatusUndeploying)
}
