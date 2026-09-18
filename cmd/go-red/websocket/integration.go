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

// WebSocketHandler integrates the WebSocket hub with the flow engine and node registry
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

// Inbound payload shapes. Kept as named types so handlers have a signature
// a reader can follow (and tests can construct).

type flowIDPayload struct {
	FlowID string `json:"flowId"`
}

type flowCreatePayload struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type flowUpdatePayload struct {
	FlowID string                `json:"flowId"`
	Flow   dto.FlowUpdateRequest `json:"flow"`
}

type flowDeployPayload struct {
	FlowID string `json:"flowId"`
	Force  bool   `json:"force,omitempty"`
}

type nodePayload struct {
	Node   dto.Node `json:"node"`
	FlowID string   `json:"flowId"`
}

type nodeRemovePayload struct {
	NodeID string `json:"nodeId"`
	FlowID string `json:"flowId"`
}

type connectionPayload struct {
	Connection dto.Connection `json:"connection"`
	FlowID     string         `json:"flowId"`
}

type connectionRemovePayload struct {
	ConnectionID string `json:"connectionId"`
	FlowID       string `json:"flowId"`
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

// HandleMessage processes incoming WebSocket messages with full engine integration
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
	case MessageTypeFlowCreate:
		var p flowCreatePayload
		if decode(&p) {
			h.handleFlowCreate(client, p.Name, p.Description)
		}
	case MessageTypeFlowUpdate:
		var p flowUpdatePayload
		if decode(&p) {
			h.handleFlowUpdate(client, p.FlowID, p.Flow)
		}
	case MessageTypeFlowDelete:
		var p flowIDPayload
		if decode(&p) {
			h.handleFlowDelete(client, p.FlowID)
		}
	case MessageTypeFlowDeploy:
		var p flowDeployPayload
		if decode(&p) {
			h.handleFlowDeploy(client, p.FlowID)
		}
	case MessageTypeFlowUndeploy:
		var p flowIDPayload
		if decode(&p) {
			h.handleFlowUndeploy(client, p.FlowID)
		}

	case MessageTypeNodeAdd:
		var p nodePayload
		if decode(&p) {
			h.handleNodeAdd(client, p)
		}
	case MessageTypeNodeRemove:
		var p nodeRemovePayload
		if decode(&p) {
			h.handleNodeRemove(client, p.NodeID, p.FlowID)
		}
	case MessageTypeNodeUpdate:
		var p nodePayload
		if decode(&p) {
			h.handleNodeUpdate(client, p)
		}

	case MessageTypeConnectionAdd:
		var p connectionPayload
		if decode(&p) {
			h.handleConnectionAdd(client, p)
		}
	case MessageTypeConnectionRemove:
		var p connectionRemovePayload
		if decode(&p) {
			h.handleConnectionRemove(client, p.ConnectionID, p.FlowID)
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
		h.sendError(client, "unknown message type", errors.New("unknown message type"), map[string]interface{}{"type": message.Type})
	}
}

// allFlowSummaries returns the wire-format summary of every flow, for the
// flow:list broadcast (both the direct response to a flow:list request and
// the broadcasts sent after any flow-mutating operation).
func (h *WebSocketHandler) allFlowSummaries() []dto.FlowSummary {
	flows := h.flowEngine.GetAllFlows()
	summaries := make([]dto.FlowSummary, len(flows))
	for i, flow := range flows {
		summaries[i] = dto.ToWireSummary(flow)
	}
	return summaries
}

// broadcastFlowList pushes the current flow list to every client.
func (h *WebSocketHandler) broadcastFlowList() {
	h.hub.Broadcast(MessageTypeFlowList, map[string]interface{}{"flows": h.allFlowSummaries()})
}

// broadcastFlowStatus pushes a flow's current lifecycle status to every
// client, in the same wire enum GET /api/flows uses.
func (h *WebSocketHandler) broadcastFlowStatus(flowID string) {
	status, err := h.flowEngine.GetFlowStatus(flowID)
	if err != nil {
		return
	}
	h.hub.Broadcast(MessageTypeFlowStatus, map[string]interface{}{
		"flowId": flowID,
		"status": dto.FlowStatusFromEngine(status),
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

func (h *WebSocketHandler) handleFlowCreate(client *Client, name, description string) {
	flow, err := h.flowEngine.CreateFlow("", name, description)
	if err != nil {
		h.sendError(client, "failed to create flow", err, map[string]interface{}{"name": name})
		return
	}

	h.BroadcastToClient(client, MessageTypeFlowCreate, dto.ToWire(flow))
	h.broadcastFlowList()
}

func (h *WebSocketHandler) handleFlowUpdate(client *Client, flowID string, req dto.FlowUpdateRequest) {
	flow, err := h.flowEngine.UpdateFlow(flowID, func(f *engine.Flow) error {
		req.ApplyTo(f)
		return nil
	})
	if err != nil {
		h.sendError(client, "failed to update flow", err, map[string]interface{}{"flowId": flowID})
		return
	}

	h.BroadcastToClient(client, MessageTypeFlowUpdate, dto.ToWire(flow))
	h.broadcastFlowList()
}

func (h *WebSocketHandler) handleFlowDelete(client *Client, flowID string) {
	if err := h.flowEngine.DeleteFlow(flowID); err != nil {
		h.sendError(client, "failed to delete flow", err, map[string]interface{}{"flowId": flowID})
		return
	}

	h.hub.Broadcast(MessageTypeFlowDelete, map[string]interface{}{
		"flowId": flowID,
	})
	h.broadcastFlowList()
}

func (h *WebSocketHandler) handleFlowDeploy(client *Client, flowID string) {
	if err := h.flowEngine.DeployFlow(flowID); err != nil {
		h.sendError(client, "failed to deploy flow", err, map[string]interface{}{"flowId": flowID})
		h.broadcastFlowStatus(flowID)
		return
	}

	h.hub.Broadcast(MessageTypeFlowDeploy, map[string]interface{}{
		"flowId": flowID,
		"status": dto.FlowStatusRunning,
	})
	h.broadcastFlowStatus(flowID)
	h.broadcastFlowList()
}

func (h *WebSocketHandler) handleFlowUndeploy(client *Client, flowID string) {
	if err := h.flowEngine.Undeploy(flowID); err != nil {
		h.sendError(client, "failed to undeploy flow", err, map[string]interface{}{"flowId": flowID})
		return
	}

	h.hub.Broadcast(MessageTypeFlowUndeploy, map[string]interface{}{
		"flowId": flowID,
		"status": dto.FlowStatusDraft,
	})
	h.broadcastFlowStatus(flowID)
	h.broadcastFlowList()
}

func (h *WebSocketHandler) handleNodeAdd(client *Client, p nodePayload) {
	node := dto.NodeFromWire(p.Node.ID, p.Node)

	_, err := h.flowEngine.UpdateFlow(p.FlowID, func(f *engine.Flow) error {
		return f.AddNode(node)
	})
	if err != nil {
		h.sendError(client, "failed to add node", err, map[string]interface{}{"flowId": p.FlowID, "nodeId": p.Node.ID})
		return
	}

	h.hub.Broadcast(MessageTypeNodeAdd, map[string]interface{}{
		"flowId": p.FlowID,
		"node":   dto.NodeToWire(node),
	})
}

func (h *WebSocketHandler) handleNodeRemove(client *Client, nodeID, flowID string) {
	_, err := h.flowEngine.UpdateFlow(flowID, func(f *engine.Flow) error {
		return f.RemoveNode(nodeID)
	})
	if err != nil {
		h.sendError(client, "failed to remove node", err, map[string]interface{}{"flowId": flowID, "nodeId": nodeID})
		return
	}

	h.hub.Broadcast(MessageTypeNodeRemove, map[string]interface{}{
		"flowId": flowID,
		"nodeId": nodeID,
	})
}

func (h *WebSocketHandler) handleNodeUpdate(client *Client, p nodePayload) {
	var updated *engine.Node
	_, err := h.flowEngine.UpdateFlow(p.FlowID, func(f *engine.Flow) error {
		node, exists := f.Nodes[p.Node.ID]
		if !exists {
			return errors.New("node " + p.Node.ID + " not found in flow")
		}
		node.Type = p.Node.Type
		node.Name = p.Node.Name
		node.Config = p.Node.Config
		node.X = p.Node.Position.X
		node.Y = p.Node.Position.Y
		node.Disabled = p.Node.Disabled
		copied := *node
		updated = &copied
		return nil
	})
	if err != nil {
		h.sendError(client, "failed to update node", err, map[string]interface{}{"flowId": p.FlowID, "nodeId": p.Node.ID})
		return
	}

	h.hub.Broadcast(MessageTypeNodeUpdate, map[string]interface{}{
		"flowId": p.FlowID,
		"nodeId": p.Node.ID,
		"node":   dto.NodeToWire(updated),
	})
}

func (h *WebSocketHandler) handleConnectionAdd(client *Client, p connectionPayload) {
	conn := dto.ConnectionFromWire(p.Connection)

	_, err := h.flowEngine.UpdateFlow(p.FlowID, func(f *engine.Flow) error {
		return f.AddConnection(conn)
	})
	if err != nil {
		h.sendError(client, "failed to add connection", err, map[string]interface{}{"flowId": p.FlowID})
		return
	}

	h.hub.Broadcast(MessageTypeConnectionAdd, map[string]interface{}{
		"flowId":     p.FlowID,
		"connection": dto.ConnectionToWire(conn),
	})
}

func (h *WebSocketHandler) handleConnectionRemove(client *Client, connectionID, flowID string) {
	_, err := h.flowEngine.UpdateFlow(flowID, func(f *engine.Flow) error {
		return f.RemoveConnection(connectionID)
	})
	if err != nil {
		h.sendError(client, "failed to remove connection", err, map[string]interface{}{"flowId": flowID, "connectionId": connectionID})
		return
	}

	h.hub.Broadcast(MessageTypeConnectionRemove, map[string]interface{}{
		"flowId":       flowID,
		"connectionId": connectionID,
	})
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
