package websocket

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// WebSocketHandler integrates the WebSocket hub with the flow engine and
// node registry.
//
// The WebSocket is an event and query channel, not a write path: flows are
// created, edited, deployed and deleted over the REST API (cmd/go-red), and
// the REST handlers call FlowChanged/FlowDeleted so every connected client
// learns about the result. Clients may query (flow:list, flow:get,
// message:log, state:sync) and trigger runtime actions (message:send).
type WebSocketHandler struct {
	hub          *Hub
	flowEngine   *engine.FlowEngine
	nodeRegistry *registry.NodeRegistry
}

// NewWebSocketHandler creates a new WebSocketHandler
func NewWebSocketHandler(hub *Hub, flowEngine *engine.FlowEngine, nodeRegistry *registry.NodeRegistry) *WebSocketHandler {
	return &WebSocketHandler{
		hub:          hub,
		flowEngine:   flowEngine,
		nodeRegistry: nodeRegistry,
	}
}

// Inbound payload shapes.

type flowIDPayload struct {
	FlowID string `json:"flowId"`
}

type messageLogPayload struct {
	FlowID string `json:"flowId"`
	Limit  int    `json:"limit"`
}

type messageSendPayload struct {
	FlowID  string                 `json:"flowId"`
	NodeID  string                 `json:"nodeId"`
	Payload map[string]interface{} `json:"payload"`
}

// publicError returns the text of err that is safe to show a client. Every
// error the engine returns is user-facing by construction (unknown flow,
// invalid definition, node config problems), except persistence failures,
// which may carry file system details.
func publicError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, engine.ErrPersist) {
		return "failed to save flow"
	}
	return err.Error()
}

// sendError answers client with an error message. code is a short, stable
// identifier for the failure; err becomes the human-readable message.
func (h *WebSocketHandler) sendError(client *Client, code string, err error, fields map[string]interface{}) {
	payload := map[string]interface{}{
		"error":   code,
		"message": publicError(err),
	}
	for k, v := range fields {
		payload[k] = v
	}
	h.BroadcastToClient(client, MessageTypeError, payload)
}

// HandleMessage processes incoming WebSocket messages.
func (h *WebSocketHandler) HandleMessage(client *Client, message WebSocketMessage) {
	slog.Debug("websocket message received", "type", message.Type)

	data := []byte(message.Data)
	decode := func(v interface{}) bool {
		if len(data) == 0 {
			return true
		}
		if err := json.Unmarshal(data, v); err != nil {
			slog.Warn("malformed websocket payload", "type", message.Type, "err", err)
			h.sendError(client, "invalid payload", errors.New("payload could not be parsed"), map[string]interface{}{"type": message.Type})
			return false
		}
		return true
	}

	switch message.Type {
	case MessageTypeFlowList:
		h.handleFlowList(client)
	case MessageTypeFlowGet:
		var p flowIDPayload
		if decode(&p) {
			h.handleFlowGet(client, p.FlowID)
		}
	case MessageTypePing:
		h.BroadcastToClient(client, MessageTypePong, map[string]string{"message": "pong"})
	case MessageTypeStateSync:
		h.handleStateSync(client)
	case MessageTypeMessageLog:
		var p messageLogPayload
		if decode(&p) {
			h.handleMessageLog(client, p.FlowID, p.Limit)
		}
	case MessageTypeMessageSend:
		var p messageSendPayload
		if decode(&p) {
			h.handleMessageSend(client, p.FlowID, p.NodeID, p.Payload)
		}
	default:
		slog.Debug("unknown websocket message type", "type", message.Type)
		h.sendError(client, "unknown message type", errors.New("unknown message type; flows are edited over the REST API"), map[string]interface{}{"type": message.Type})
	}
}

// FlowChanged tells every client that a flow was created, edited, deployed
// or undeployed. Called by the REST handlers after a successful mutation.
func (h *WebSocketHandler) FlowChanged(flowID string) {
	h.broadcastFlowStatus(flowID)
	h.broadcastFlowList()
}

// FlowDeleted tells every client that a flow no longer exists.
func (h *WebSocketHandler) FlowDeleted(flowID string) {
	h.hub.Broadcast(MessageTypeFlowDelete, map[string]interface{}{"flowId": flowID})
	h.broadcastFlowList()
}

// allFlowSummaries returns the wire-format summary of every flow.
func (h *WebSocketHandler) allFlowSummaries() []dto.FlowSummary {
	flows := h.flowEngine.GetAllFlows()
	summaries := make([]dto.FlowSummary, len(flows))
	for i, flow := range flows {
		summaries[i] = dto.ToWireSummary(flow)
	}
	return summaries
}

func (h *WebSocketHandler) broadcastFlowList() {
	h.hub.Broadcast(MessageTypeFlowList, map[string]interface{}{"flows": h.allFlowSummaries()})
}

// broadcastFlowStatus pushes a flow's current lifecycle status to every
// client, in the same wire enum GET /api/flows uses.
func (h *WebSocketHandler) broadcastFlowStatus(flowID string) {
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		return
	}
	summary := dto.ToWireSummary(flow)
	h.hub.Broadcast(MessageTypeFlowStatus, map[string]interface{}{
		"flowId":     flowID,
		"status":     summary.Status,
		"updatedAt":  summary.UpdatedAt,
		"deployedAt": summary.DeployedAt,
	})
}

func (h *WebSocketHandler) handleFlowList(client *Client) {
	h.BroadcastToClient(client, MessageTypeFlowList, map[string]interface{}{
		"flows": h.allFlowSummaries(),
	})
}

func (h *WebSocketHandler) handleFlowGet(client *Client, flowID string) {
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		h.sendError(client, "flow not found", err, map[string]interface{}{"flowId": flowID})
		return
	}
	h.BroadcastToClient(client, MessageTypeFlowGet, dto.ToWire(flow))
}

func (h *WebSocketHandler) handleStateSync(client *Client) {
	engineFlows := h.flowEngine.GetAllFlows()
	flows := make([]dto.Flow, len(engineFlows))
	for i, f := range engineFlows {
		flows[i] = dto.ToWire(f)
	}

	h.BroadcastToClient(client, MessageTypeStateSync, map[string]interface{}{
		"flows":     flows,
		"nodeTypes": h.nodeRegistry.GetAllNodes(),
	})
}

func (h *WebSocketHandler) handleMessageSend(client *Client, flowID, nodeID string, payload map[string]interface{}) {
	if err := h.flowEngine.InjectMessage(flowID, nodeID, payload); err != nil {
		h.sendError(client, "failed to inject message", err, map[string]interface{}{"flowId": flowID, "nodeId": nodeID})
		return
	}

	h.BroadcastToClient(client, MessageTypeMessageSend, map[string]interface{}{
		"status": "sent",
		"flowId": flowID,
		"nodeId": nodeID,
	})
}

// handleMessageLog answers a message:log request with the logged messages
// (optionally filtered by flowId, optionally capped by limit).
func (h *WebSocketHandler) handleMessageLog(client *Client, flowID string, limit int) {
	var messages []engine.Message
	if flowID != "" {
		messages = h.flowEngine.GetMessageLogForFlow(flowID)
	} else {
		messages = h.flowEngine.GetMessageLog()
	}

	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	h.BroadcastToClient(client, MessageTypeMessageLog, map[string]interface{}{
		"flowId":   flowID,
		"messages": dto.MessagesToWire(messages),
	})
}

// BroadcastToClient sends a message to a specific client
func (h *WebSocketHandler) BroadcastToClient(client *Client, messageType MessageType, data interface{}) {
	h.hub.BroadcastToClient(client, messageType, data)
}

// Broadcast sends a message to all clients
func (h *WebSocketHandler) Broadcast(messageType MessageType, data interface{}) {
	h.hub.Broadcast(messageType, data)
}

// GetHub returns the underlying hub
func (h *WebSocketHandler) GetHub() *Hub {
	return h.hub
}

// ServeWebSocket upgrades the request and attaches HandleMessage to the client.
func (h *WebSocketHandler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeWebSocket(w, r, h.HandleMessage)
}
