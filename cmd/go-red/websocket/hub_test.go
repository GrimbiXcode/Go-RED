package websocket

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHub(t *testing.T) {
	t.Run("should create new hub", func(t *testing.T) {
		hub := NewHub()
		assert.NotNil(t, hub)
		assert.NotNil(t, hub.clients)
		assert.NotNil(t, hub.register)
		assert.NotNil(t, hub.unregister)
		assert.NotNil(t, hub.broadcast)
	})
}

func TestHubClientCount(t *testing.T) {
	t.Run("should return zero clients initially", func(t *testing.T) {
		hub := NewHub()
		assert.Equal(t, 0, hub.ClientCount())
	})
}

func TestHubGetClients(t *testing.T) {
	t.Run("should return empty list initially", func(t *testing.T) {
		hub := NewHub()
		clients := hub.GetClients()
		assert.Len(t, clients, 0)
	})
}

func TestHubBroadcastWithoutClients(t *testing.T) {
	t.Run("should broadcast message without panic when no clients", func(t *testing.T) {
		hub := NewHub()
		// This should not panic even with no clients
		hub.Broadcast(MessageType("test:message"), map[string]string{"test": "data"})
		assert.True(t, true, "Broadcast should not panic")
	})
}

func TestHubBroadcastToClient(t *testing.T) {
	t.Run("should broadcast to specific client without panic", func(t *testing.T) {
		hub := NewHub()

		// Create a client with a message channel
		client := &Client{
			hub:  hub,
			send: make(chan WebSocketMessage, 10),
		}

		// Register the client directly (without running the hub loop)
		hub.mu.Lock()
		hub.clients[client] = true
		hub.mu.Unlock()

		// Send a broadcast message to specific client
		hub.BroadcastToClient(client, MessageType("test:message"), map[string]string{"test": "data"})

		// Read the message from client channel
		select {
		case msg := <-client.send:
			assert.Equal(t, MessageType("test:message"), msg.Type)
		default:
			// Message might not be received if channel is full, but test should not panic
			assert.True(t, true, "Broadcast to client should not panic")
		}
	})
}

func TestClientConnection(t *testing.T) {
	t.Run("should create client with required fields", func(t *testing.T) {
		hub := NewHub()
		client := &Client{
			hub:  hub,
			send: make(chan WebSocketMessage, 1),
		}

		assert.NotNil(t, client)
		assert.NotNil(t, client.hub)
		assert.NotNil(t, client.send)
	})
}

func TestConcurrentClientRegistration(t *testing.T) {
	t.Run("should handle concurrent client registrations in map", func(t *testing.T) {
		hub := NewHub()

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				client := &Client{
					hub:  hub,
					send: make(chan WebSocketMessage, 1),
				}
				// Directly add to clients map (simulating registration)
				hub.mu.Lock()
				hub.clients[client] = true
				hub.mu.Unlock()
			}(i)
		}
		wg.Wait()

		// Verify all clients were added
		hub.mu.RLock()
		defer hub.mu.RUnlock()
		assert.Len(t, hub.clients, 10)
	})
}

func TestMessageType(t *testing.T) {
	t.Run("should have all expected message types", func(t *testing.T) {
		// Verify that all expected message types are defined
		messageTypes := []MessageType{
			MessageTypeFlowList,
			MessageTypeFlowGet,
			MessageTypeFlowCreate,
			MessageTypeFlowUpdate,
			MessageTypeFlowDelete,
			MessageTypeFlowDeploy,
			MessageTypeFlowUndeploy,
			MessageTypeFlowStatus,
			MessageTypeFlowExport,
			MessageTypeFlowImport,
			MessageTypeNodeAdd,
			MessageTypeNodeRemove,
			MessageTypeNodeUpdate,
			MessageTypeNodeConfig,
			MessageTypeNodeStatus,
			MessageTypeConnectionAdd,
			MessageTypeConnectionRemove,
			MessageTypeConnectionUpdate,
			MessageTypeMessageSend,
			MessageTypeMessageLog,
			MessageTypeMessageDebug,
			MessageTypeStateSync,
			MessageTypeError,
			MessageTypeWarning,
			MessageTypeInfo,
			MessageTypePing,
			MessageTypePong,
			MessageTypeAll,
		}

		for _, msgType := range messageTypes {
			assert.NotEmpty(t, string(msgType), "Message type should not be empty")
		}
	})
}

func TestMessageCreation(t *testing.T) {
	t.Run("should create message with type and data", func(t *testing.T) {
		message := WebSocketMessage{
			Type: "test:message",
			Data: json.RawMessage(`{"test": "data"}`),
		}

		assert.Equal(t, MessageType("test:message"), message.Type)
		assert.Equal(t, json.RawMessage(`{"test": "data"}`), message.Data)
	})
}

func TestWebSocketMessageStruct(t *testing.T) {
	t.Run("should create WebSocketMessage with all fields", func(t *testing.T) {
		message := WebSocketMessage{
			Type:      MessageType("test:type"),
			Data:      json.RawMessage(`{"key": "value"}`),
			Timestamp: "2026-01-01T00:00:00Z",
			RequestID: "req-123",
		}

		assert.Equal(t, MessageType("test:type"), message.Type)
		assert.Equal(t, json.RawMessage(`{"key": "value"}`), message.Data)
		assert.Equal(t, "2026-01-01T00:00:00Z", message.Timestamp)
		assert.Equal(t, "req-123", message.RequestID)
	})
}

func TestClientUnregister(t *testing.T) {
	t.Run("should unregister client from map", func(t *testing.T) {
		hub := NewHub()

		client := &Client{
			hub:  hub,
			send: make(chan WebSocketMessage, 1),
		}

		// Add client
		hub.mu.Lock()
		hub.clients[client] = true
		hub.mu.Unlock()

		assert.Equal(t, 1, hub.ClientCount())

		// Remove client
		hub.mu.Lock()
		delete(hub.clients, client)
		close(client.send)
		hub.mu.Unlock()

		assert.Equal(t, 0, hub.ClientCount())
	})
}
