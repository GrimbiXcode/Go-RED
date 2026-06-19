package engine

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// FlowStatus represents the current status of a flow.
type FlowStatus string

const (
	// FlowStatusInactive means the flow is not currently running.
	FlowStatusInactive FlowStatus = "inactive"
	// FlowStatusActive means the flow is currently running.
	FlowStatusActive FlowStatus = "active"
	// FlowStatusError means the flow encountered an error.
	FlowStatusError FlowStatus = "error"
	// FlowStatusDeploying means the flow is being deployed.
	FlowStatusDeploying FlowStatus = "deploying"
	// FlowStatusUndeploying means the flow is being undeployed.
	FlowStatusUndeploying FlowStatus = "undeploying"
)

// Node represents a node in a flow.
type Node struct {
	// ID is a unique identifier for this node within the flow.
	ID string `json:"id"`
	
	// Type is the type of node (e.g., "http-request", "function").
	Type string `json:"type"`
	
	// Name is an optional display name for the node.
	Name string `json:"name,omitempty"`
	
	// Config contains the configuration for this node.
	Config map[string]interface{} `json:"config"`
	
	// X and Y are the coordinates of the node in the UI.
	X float64 `json:"x"`
	Y float64 `json:"y"`
	
	// Disabled indicates whether this node is disabled.
	Disabled bool `json:"disabled"`
}

// NodeConnection represents a connection between two nodes.
type NodeConnection struct {
	// ID is a unique identifier for this connection.
	ID string `json:"id"`
	
	// SourceNode is the ID of the source node.
	SourceNode string `json:"sourceNode"`
	
	// SourcePort is the ID of the source port (e.g., "output1").
	SourcePort string `json:"sourcePort"`
	
	// TargetNode is the ID of the target node.
	TargetNode string `json:"targetNode"`
	
	// TargetPort is the ID of the target port (e.g., "input").
	TargetPort string `json:"targetPort"`
}

// FlowConfig contains configuration options for a flow.
type FlowConfig struct {
	// Timeout is the maximum time a message can spend in the flow before timing out.
	Timeout time.Duration `json:"timeout"`
	
	// MaxConcurrency is the maximum number of messages that can be processed concurrently.
	MaxConcurrency int `json:"maxConcurrency"`
	
	// RetryPolicy defines how to handle retries for failed messages.
	RetryPolicy RetryPolicy `json:"retryPolicy"`
	
	// Environment variables for the flow.
	Environment map[string]string `json:"environment"`
}

// RetryPolicy defines retry behavior for failed messages.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"maxRetries"`
	
	// Backoff is the initial backoff duration between retries.
	Backoff time.Duration `json:"backoff"`
	
	// MaxBackoff is the maximum backoff duration between retries.
	MaxBackoff time.Duration `json:"maxBackoff"`
	
	// RetryOn defines which errors should trigger a retry.
	// Empty list means retry on all errors.
	RetryOn []string `json:"retryOn"`
}

// Flow represents a complete flow definition.
type Flow struct {
	// ID is a unique identifier for this flow.
	ID string `json:"id"`
	
	// Name is a human-readable name for the flow.
	Name string `json:"name"`
	
	// Description provides additional information about the flow.
	Description string `json:"description"`
	
	// Nodes contains all nodes in the flow, keyed by their ID.
	Nodes map[string]*Node `json:"nodes"`
	
	// Connections contains all connections between nodes.
	Connections []NodeConnection `json:"connections"`
	
	// Config contains flow-level configuration.
	Config FlowConfig `json:"config"`
	
	// Status is the current status of the flow.
	Status FlowStatus `json:"status"`
	
	// CreatedAt is when the flow was created.
	CreatedAt time.Time `json:"createdAt"`
	
	// UpdatedAt is when the flow was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
	
	// Version is the version of the flow schema.
	Version string `json:"version"`
}

// NewFlow creates a new Flow with default values.
func NewFlow(id, name string) *Flow {
	return &Flow{
		ID:          id,
		Name:        name,
		Description: "",
		Nodes:       make(map[string]*Node),
		Connections: []NodeConnection{},
		Config: FlowConfig{
			Timeout:       30 * time.Second,
			MaxConcurrency: 100,
			RetryPolicy: RetryPolicy{
				MaxRetries:  3,
				Backoff:     1 * time.Second,
				MaxBackoff:  30 * time.Second,
				RetryOn:     []string{},
			},
			Environment: make(map[string]string),
		},
		Status:    FlowStatusInactive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Version:   "1.0",
	}
}

// AddNode adds a node to the flow.
func (f *Flow) AddNode(node *Node) error {
	if node.ID == "" {
		return errors.New("node ID cannot be empty")
	}
	if _, exists := f.Nodes[node.ID]; exists {
		return errors.New("node with ID " + node.ID + " already exists")
	}
	
	f.Nodes[node.ID] = node
	f.UpdatedAt = time.Now().UTC()
	return nil
}

// RemoveNode removes a node from the flow.
func (f *Flow) RemoveNode(nodeID string) error {
	if _, exists := f.Nodes[nodeID]; !exists {
		return errors.New("node with ID " + nodeID + " not found")
	}
	
	// Remove all connections involving this node
	var newConnections []NodeConnection
	for _, conn := range f.Connections {
		if conn.SourceNode != nodeID && conn.TargetNode != nodeID {
			newConnections = append(newConnections, conn)
		}
	}
	f.Connections = newConnections
	
	delete(f.Nodes, nodeID)
	f.UpdatedAt = time.Now().UTC()
	return nil
}

// AddConnection adds a connection between two nodes.
func (f *Flow) AddConnection(conn NodeConnection) error {
	// Validate source node exists
	if _, exists := f.Nodes[conn.SourceNode]; !exists {
		return errors.New("source node " + conn.SourceNode + " not found")
	}
	
	// Validate target node exists
	if _, exists := f.Nodes[conn.TargetNode]; !exists {
		return errors.New("target node " + conn.TargetNode + " not found")
	}
	
	// Check for duplicate connection
	for _, existing := range f.Connections {
		if existing.SourceNode == conn.SourceNode &&
			existing.SourcePort == conn.SourcePort &&
			existing.TargetNode == conn.TargetNode &&
			existing.TargetPort == conn.TargetPort {
			return errors.New("connection already exists")
		}
	}
	
	// Generate ID if not provided
	if conn.ID == "" {
		conn.ID = "conn-" + uuid.New().String()
	}
	
	f.Connections = append(f.Connections, conn)
	f.UpdatedAt = time.Now().UTC()
	return nil
}

// RemoveConnection removes a connection from the flow.
func (f *Flow) RemoveConnection(connectionID string) error {
	for i, conn := range f.Connections {
		if conn.ID == connectionID {
			f.Connections = append(f.Connections[:i], f.Connections[i+1:]...)
			f.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errors.New("connection with ID " + connectionID + " not found")
}

// GetConnectionsForNode returns all connections where the given node is either source or target.
func (f *Flow) GetConnectionsForNode(nodeID string) []NodeConnection {
	var connections []NodeConnection
	for _, conn := range f.Connections {
		if conn.SourceNode == nodeID || conn.TargetNode == nodeID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// GetSourceConnections returns all connections where the given node is the source.
func (f *Flow) GetSourceConnections(nodeID string) []NodeConnection {
	var connections []NodeConnection
	for _, conn := range f.Connections {
		if conn.SourceNode == nodeID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// GetTargetConnections returns all connections where the given node is the target.
func (f *Flow) GetTargetConnections(nodeID string) []NodeConnection {
	var connections []NodeConnection
	for _, conn := range f.Connections {
		if conn.TargetNode == nodeID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// Validate checks if the flow is valid.
func (f *Flow) Validate() error {
	// Check flow has at least one node
	if len(f.Nodes) == 0 {
		return errors.New("flow must have at least one node")
	}
	
	// Check all connections reference existing nodes
	for _, conn := range f.Connections {
		if _, exists := f.Nodes[conn.SourceNode]; !exists {
			return errors.New("connection references non-existent source node: " + conn.SourceNode)
		}
		if _, exists := f.Nodes[conn.TargetNode]; !exists {
			return errors.New("connection references non-existent target node: " + conn.TargetNode)
		}
	}
	
	return nil
}

// Clone creates a deep copy of the flow.
func (f *Flow) Clone() *Flow {
	clone := &Flow{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		Nodes:       make(map[string]*Node),
		Connections: make([]NodeConnection, len(f.Connections)),
		Config:      f.Config,
		Status:      f.Status,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
		Version:     f.Version,
	}
	
	// Clone nodes
	for k, v := range f.Nodes {
		clone.Nodes[k] = &Node{
			ID:       v.ID,
			Type:     v.Type,
			Config:   cloneMap(v.Config),
			X:        v.X,
			Y:        v.Y,
			Disabled:  v.Disabled,
		}
	}
	
	// Clone connections
	for i, conn := range f.Connections {
		clone.Connections[i] = NodeConnection{
			ID:          conn.ID,
			SourceNode:  conn.SourceNode,
			SourcePort:  conn.SourcePort,
			TargetNode:  conn.TargetNode,
			TargetPort:  conn.TargetPort,
		}
	}
	
	// Clone config
	clone.Config.Environment = cloneStringMap(f.Config.Environment)
	
	return clone
}

// cloneMap creates a deep copy of a map[string]interface{}.
func cloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	
	clone := make(map[string]interface{}, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// cloneStringMap creates a deep copy of a map[string]string.
func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	
	clone := make(map[string]string, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}
