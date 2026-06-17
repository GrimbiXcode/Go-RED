// Package websocket provides WebSocket communication for Go-RED.
// It handles real-time updates between the server and connected clients.
package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType defines the type of WebSocket message
// These match the WebSocketMessageType in the frontend
type MessageType string

const (
	// Flow-related message types
	MessageTypeFlowList    MessageType = "flow:list"
	MessageTypeFlowGet     MessageType = "flow:get"
	MessageTypeFlowCreate  MessageType = "flow:create"
	MessageTypeFlowUpdate  MessageType = "flow:update"
	MessageTypeFlowDelete  MessageType = "flow:delete"
	MessageTypeFlowDeploy  MessageType = "flow:deploy"
	MessageTypeFlowUndeploy MessageType = "flow:undeploy"
	MessageTypeFlowStatus  MessageType = "flow:status"
	MessageTypeFlowExport  MessageType = "flow:export"
	MessageTypeFlowImport  MessageType = "flow:import"

	// Node-related message types
	MessageTypeNodeAdd     MessageType = "node:add"
	MessageTypeNodeRemove  MessageType = "node:remove"
	MessageTypeNodeUpdate  MessageType = "node:update"
	MessageTypeNodeConfig  MessageType = "node:config"
	MessageTypeNodeStatus  MessageType = "node:status"

	// Connection-related message types
	MessageTypeConnectionAdd    MessageType = "connection:add"
	MessageTypeConnectionRemove MessageType = "connection:remove"
	MessageTypeConnectionUpdate MessageType = "connection:update"

	// Message-related message types
	MessageTypeMessageSend  MessageType = "message:send"
	MessageTypeMessageLog   MessageType = "message:log"
	MessageTypeMessageDebug MessageType = "message:debug"

	// System message types
	MessageTypeError   MessageType = "error"
	MessageTypeWarning MessageType = "warning"
	MessageTypeInfo    MessageType = "info"
	MessageTypePing    MessageType = "ping"
	MessageTypePong    MessageType = "pong"
	MessageTypeStateSync MessageType = "state:sync"
	MessageTypeAll     MessageType = "*"
)

// WebSocketMessage represents a message sent or received over WebSocket
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp string          `json:"timestamp"`
	RequestID string          `json:"requestId,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan WebSocketMessage
	messageHandler func(*Client, WebSocketMessage)
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan WebSocketMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan WebSocketMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected. Total clients: %d", len(h.clients))

			// Send initial state sync to new client
			initialState := WebSocketMessage{
				Type:      MessageTypeStateSync,
				Data:      json.RawMessage(`{"message": "connected"}`),
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			client.send <- initialState

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected. Total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, close connection
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(messageType MessageType, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling WebSocket message data: %v", err)
		return
	}

	message := WebSocketMessage{
		Type:      messageType,
		Data:      json.RawMessage(jsonData),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	select {
	case h.broadcast <- message:
	default:
		log.Println("Broadcast channel full, dropping message")
	}
}

// BroadcastToClient sends a message to a specific client
func (h *Hub) BroadcastToClient(client *Client, messageType MessageType, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling WebSocket message data: %v", err)
		return
	}

	message := WebSocketMessage{
		Type:      messageType,
		Data:      json.RawMessage(jsonData),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	select {
	case client.send <- message:
	default:
		close(client.send)
		delete(h.clients, client)
	}
}

// ClientCount returns the number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClients returns all connected clients
func (h *Hub) GetClients() []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	return clients
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			log.Printf("Error unmarshaling WebSocket message: %v", err)
			continue
		}

		// Handle the message using the custom message handler if provided
		if c.messageHandler != nil {
			c.messageHandler(c, wsMessage)
		} else {
			// Default handling - just log the message
			log.Printf("Received WebSocket message of type: %s (no handler configured)", wsMessage.Type)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Marshal the message directly - json.RawMessage is handled correctly by json.Marshal
			msgJSON, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling WebSocket message: %v", err)
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			w.Write(msgJSON)

			// Add queued messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				nextMessage := <-c.send
				nextMsgJSON, err := json.Marshal(nextMessage)
				if err != nil {
					log.Printf("Error marshaling WebSocket message: %v", err)
					continue
				}
				w.Write(nextMsgJSON)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWebSocket handles WebSocket requests and upgrades the connection
// It accepts an optional messageHandler function for custom message processing
func (h *Hub) ServeWebSocket(w http.ResponseWriter, r *http.Request, messageHandler func(*Client, WebSocketMessage)) {
	// Check Origin header for CORS (optional, can be configured)
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins for development
			// In production, you should validate the origin
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:           h,
		conn:          conn,
		send:          make(chan WebSocketMessage, 256),
		messageHandler: messageHandler,
	}

	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}
