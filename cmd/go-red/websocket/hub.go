// Package websocket provides WebSocket communication for Go-RED.
// It handles real-time updates between the server and connected clients.
package websocket

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType defines the type of WebSocket message. The WebSocket carries
// server events and read-only queries; every flow mutation goes through the
// REST API (see WebSocketHandler).
type MessageType string

const (
	// Flow queries (client -> server, answered on the same type) and
	// flow events (server -> client, to every client).
	MessageTypeFlowList   MessageType = "flow:list"   // query + broadcast after any change
	MessageTypeFlowGet    MessageType = "flow:get"    // query
	MessageTypeFlowDelete MessageType = "flow:delete" // event
	MessageTypeFlowStatus MessageType = "flow:status" // event: dto.FlowStatusEvent

	// Per-flow subscription (client -> server). A client only receives the
	// runtime events below for flows it subscribed to; subscribing answers
	// with flow:snapshot.
	MessageTypeSubscribe    MessageType = "subscribe"     // dto.SubscribeRequest
	MessageTypeUnsubscribe  MessageType = "unsubscribe"   // dto.SubscribeRequest
	MessageTypeFlowSnapshot MessageType = "flow:snapshot" // dto.FlowSnapshot

	// Runtime events (server -> subscribed clients).
	MessageTypeNodeStatus   MessageType = "node:status"   // dto.NodeStatusEvent
	MessageTypeDebugMessage MessageType = "debug:message" // dto.DebugMessage
	MessageTypeFlowMetrics  MessageType = "flow:metrics"  // dto.FlowMetricsEvent

	// Runtime actions.
	MessageTypeMessageSend MessageType = "message:send" // inject at a node

	// System message types
	MessageTypeError     MessageType = "error"
	MessageTypeInfo      MessageType = "info"
	MessageTypePing      MessageType = "ping"
	MessageTypePong      MessageType = "pong"
	MessageTypeStateSync MessageType = "state:sync"
	MessageTypeAll       MessageType = "*"
)

// AllMessageTypes lists every MessageType constant. It exists purely so
// cmd/gentypes can enumerate this enum via reflection (Go has no way to
// list const declarations at runtime) — keep it in sync when adding or
// removing a MessageType constant above.
var AllMessageTypes = []MessageType{
	MessageTypeFlowList,
	MessageTypeFlowGet,
	MessageTypeFlowDelete,
	MessageTypeFlowStatus,
	MessageTypeSubscribe,
	MessageTypeUnsubscribe,
	MessageTypeFlowSnapshot,
	MessageTypeNodeStatus,
	MessageTypeDebugMessage,
	MessageTypeFlowMetrics,
	MessageTypeMessageSend,
	MessageTypeError,
	MessageTypeInfo,
	MessageTypePing,
	MessageTypePong,
	MessageTypeStateSync,
	MessageTypeAll,
}

// WebSocketMessage represents a message sent or received over WebSocket.
// Exactly one WebSocketMessage is sent per WebSocket frame.
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp string          `json:"timestamp"`
	RequestID string          `json:"requestId,omitempty"`
}

const (
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// pongWait is the time allowed to read the next pong from the peer.
	pongWait = 60 * time.Second
	// pingPeriod is how often pings are sent; must be less than pongWait.
	pingPeriod = 30 * time.Second
	// maxMessageSize is the maximum inbound message size in bytes.
	maxMessageSize = 512 * 1024
	// sendBufferSize is the number of outbound messages buffered per client.
	sendBufferSize = 256
)

// Client represents a WebSocket client connection.
//
// The send channel is never closed; the hub tells a client to go away by
// closing done, and writePump reacts to that. This way nothing can ever
// send on a closed channel, whichever goroutine is enqueuing.
type Client struct {
	hub            *Hub
	conn           *websocket.Conn
	send           chan WebSocketMessage
	done           chan struct{}
	closeOnce      sync.Once
	messageHandler func(*Client, WebSocketMessage)

	// subscriptions are the flow IDs this client wants runtime events for.
	subMu         sync.Mutex
	subscriptions map[string]struct{}
}

// Subscribe adds flowID to the client's runtime event subscriptions.
func (c *Client) Subscribe(flowID string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subscriptions == nil {
		c.subscriptions = make(map[string]struct{})
	}
	c.subscriptions[flowID] = struct{}{}
}

// Unsubscribe removes flowID from the client's subscriptions.
func (c *Client) Unsubscribe(flowID string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	delete(c.subscriptions, flowID)
}

// IsSubscribed reports whether the client receives runtime events of flowID.
func (c *Client) IsSubscribed(flowID string) bool {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	_, ok := c.subscriptions[flowID]
	return ok
}

// outbound is a message on its way to clients, optionally scoped to the
// subscribers of one flow.
type outbound struct {
	message WebSocketMessage
	flowID  string
}

// close asks writePump to send a close frame and stop. Safe to call more
// than once and from any goroutine.
func (c *Client) close() {
	c.closeOnce.Do(func() {
		if c.done != nil {
			close(c.done)
		}
	})
}

// enqueue queues a message for the client without blocking. It returns
// false if the client's send buffer is full.
func (c *Client) enqueue(message WebSocketMessage) bool {
	select {
	case c.send <- message:
		return true
	default:
		return false
	}
}

// Hub maintains the set of active clients and broadcasts messages to them.
// The clients map is only ever mutated by Run (via register/unregister) or
// by removeClient, always under mu.
type Hub struct {
	// CheckOrigin decides which browser origins may open a WebSocket. nil
	// means same-origin only (see sameOrigin); main sets it from the
	// configured allowed origins.
	CheckOrigin func(r *http.Request) bool

	clients    map[*Client]bool
	broadcast  chan outbound
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan outbound, 1024),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func newMessage(messageType MessageType, data json.RawMessage) WebSocketMessage {
	return WebSocketMessage{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			slog.Info("websocket client connected", "clients", count)

			client.enqueue(newMessage(MessageTypeStateSync, json.RawMessage(`{"message": "connected"}`)))

		case client := <-h.unregister:
			h.removeClient(client)

		case out := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if out.flowID != "" && !client.IsSubscribed(out.flowID) {
					continue
				}
				if !client.enqueue(out.message) {
					slog.Warn("websocket client send buffer full, dropping broadcast", "type", out.message.Type)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// removeClient drops a client from the hub and tells its writePump to
// finish. Safe to call for a client that was already removed.
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	_, known := h.clients[client]
	if known {
		delete(h.clients, client)
	}
	count := len(h.clients)
	h.mu.Unlock()

	client.close()
	if known {
		slog.Info("websocket client disconnected", "clients", count)
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(messageType MessageType, data interface{}) {
	h.enqueueBroadcast("", messageType, data)
}

// BroadcastToFlow sends a message to the clients subscribed to flowID.
func (h *Hub) BroadcastToFlow(flowID string, messageType MessageType, data interface{}) {
	h.enqueueBroadcast(flowID, messageType, data)
}

func (h *Hub) enqueueBroadcast(flowID string, messageType MessageType, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal websocket broadcast", "type", messageType, "err", err)
		return
	}

	select {
	case h.broadcast <- outbound{message: newMessage(messageType, jsonData), flowID: flowID}:
	default:
		slog.Warn("websocket broadcast channel full, dropping message", "type", messageType)
	}
}

// BroadcastToClient sends a message to a specific client. A client whose
// send buffer is full loses the message (and is logged); it is not
// disconnected here - a client that never drains its buffer trips the
// write deadline in writePump and is removed through the normal path.
func (h *Hub) BroadcastToClient(client *Client, messageType MessageType, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal websocket message", "type", messageType, "err", err)
		return
	}

	if !client.enqueue(newMessage(messageType, jsonData)) {
		slog.Warn("websocket client send buffer full, dropping message", "type", messageType)
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

// readPump pumps messages from the WebSocket connection to the handler.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				slog.Debug("websocket read error", "err", err)
			}
			break
		}

		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			slog.Warn("ignoring malformed websocket message", "err", err, "bytes", len(message))
			continue
		}

		c.dispatch(wsMessage)
	}
}

// dispatch runs the message handler for one inbound message. A panic in a
// handler is logged and answered with an error message instead of taking
// the whole server down.
func (c *Client) dispatch(message WebSocketMessage) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in websocket handler", "type", message.Type, "panic", r, "stack", string(debug.Stack()))
			c.hub.BroadcastToClient(c, MessageTypeError, map[string]string{
				"error":   "internal error",
				"message": "the server failed to handle this message",
			})
		}
	}()

	if c.messageHandler == nil {
		slog.Debug("websocket message without handler", "type", message.Type)
		return
	}
	c.messageHandler(c, message)
}

// writePump pumps messages from the hub to the WebSocket connection, one
// WebSocketMessage per frame, and keeps the connection alive with pings.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message := <-c.send:
			data, err := json.Marshal(message)
			if err != nil {
				slog.Error("failed to marshal websocket message", "type", message.Type, "err", err)
				continue
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-c.done:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			return

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sameOrigin is the default origin policy: browser handshakes must come
// from the page the server itself served (Origin host == request Host);
// requests without an Origin header (non-browser clients) pass.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

// ServeWebSocket handles WebSocket requests and upgrades the connection
// It accepts an optional messageHandler function for custom message processing
func (h *Hub) ServeWebSocket(w http.ResponseWriter, r *http.Request, messageHandler func(*Client, WebSocketMessage)) {
	checkOrigin := h.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = sameOrigin
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin,
		// The editor offers "gored" (plus a token-carrying protocol when
		// authentication is on); selecting it makes browsers accept the
		// handshake. Clients that offer nothing get nothing, as before.
		Subprotocols: []string{"gored"},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("websocket upgrade failed", "err", err)
		return
	}

	client := &Client{
		hub:            h,
		conn:           conn,
		send:           make(chan WebSocketMessage, sendBufferSize),
		done:           make(chan struct{}),
		messageHandler: messageHandler,
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}
