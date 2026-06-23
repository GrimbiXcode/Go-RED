// Package engine provides the core flow execution engine for Go—RED.
package engine

import (
    "context"
    "github.com/google/uuid"
    "time"
)

// Message represents a message flowing between nodes in a flow.
// Messages contain the data payload, metadata, and execution context.
type Message struct {
    // ID is a unique identifier for this message.
    ID string `json:"id"`
    
    // Payload contains the main data being processed.
    // This can be any JSON-serializable value.
    Payload map[string]interface{} `json:"payload"`
    
    // Metadata contains additional information about the message.
    // Common metadata includes timestamp, source node, flow ID, etc.
    Metadata map[string]string `json:"metadata"`
    
    // Context provides cancellation and timeout for message processing.
    // This is not serialized to JSON as it contains non-serializable data.
    Context context.Context `json:"-"`
    
    // FlowID identifies which flow this message belongs to.
    FlowID string `json:"flowId"`
    
    // Path contains the list of node IDs this message has passed through.
    // This is useful for debugging and tracing message flow.
    Path []string `json:"path"`
    
    // Timestamp indicates when the message was created.
    Timestamp time.Time `json:"timestamp"`
}

// NewMessage creates a new Message with the given payload and flow ID.
func NewMessage(payload map[string]interface{}, flowID string) Message {
    return Message{
        ID:        uuid.New().String(),
        Payload:   payload,
        Metadata:  make(map[string]string),
        Context:   context.Background(),
        FlowID:    flowID,
        Path:      []string{},
        Timestamp: time.Now().UTC(),
    }
}

// NewMessageWithContext creates a new Message with a specific context.
func NewMessageWithContext(ctx context.Context, payload map[string]interface{}, flowID string) Message {
    return Message{
        ID:        uuid.New().String(),
        Payload:   payload,
        Metadata:  make(map[string]string),
        Context:   ctx,
        FlowID:    flowID,
        Path:      []string{},
        Timestamp: time.Now().UTC(),
    }
}

// AddToPath adds a node ID to the message path.
func (m *Message) AddToPath(nodeID string) {
    m.Path = append(m.Path, nodeID)
}

// SetMetadata sets a metadata key-value pair.
func (m *Message) SetMetadata(key, value string) {
    if m.Metadata == nil {
        m.Metadata = make(map[string]string)
    }
    m.Metadata[key] = value
}

// GetMetadata returns the value for a metadata key.
func (m *Message) GetMetadata(key string) (string, bool) {
    val, ok := m.Metadata[key]
    return val, ok
}

// Clone creates a deep copy of the message.
func (m *Message) Clone() Message {
    return Message{
        ID:        uuid.New().String(),
        Payload:   cloneMap(m.Payload),
        Metadata:  cloneStringMap(m.Metadata),
        Context:   m.Context,
        FlowID:    m.FlowID,
        Path:      append([]string(nil), m.Path...),
        Timestamp: m.Timestamp,
    }
}


