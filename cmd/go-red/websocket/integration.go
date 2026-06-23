package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
)

// WebSocketHandler integrates the WebSocket hub with the flow engine and node registry
type WebSocketHandler struct {
	hub         *Hub
	flowEngine   *engine.FlowEngine
	nodeRegistry *registry.NodeRegistry
}

// NewWebSocketHandler creates a new WebSocketHandler
func NewWebSocketHandler(hub *Hub, flowEngine *engine.FlowEngine, nodeRegistry *registry.NodeRegistry) *WebSocketHandler {
	return &WebSocketHandler{
		hub:         hub,
		flowEngine:   flowEngine,
		nodeRegistry: nodeRegistry,
	}
}

// HandleMessage processes incoming WebSocket messages with full engine integration
func (h *WebSocketHandler) HandleMessage(client *Client, message WebSocketMessage) {
	messageType := string(message.Type)
	log.Printf("Received WebSocket message of type: %s", messageType)

	// message.Data is a string containing JSON, ready for unmarshaling
	// Convert to []byte for json.Unmarshal
	data := []byte(message.Data)

	switch messageType {
	// Flow-related messages
	case "flow:list":
		h.handleFlowList(client)
	case "flow:get":
		var flowData struct{ FlowID string `json:"flowId"` }
		if err := json.Unmarshal(data, &flowData); err == nil {
			h.handleFlowGet(client, flowData.FlowID)
		}
	case "flow:create":
		var flowData struct {
			Name        string `json:"name"`
			Description string `json:"description,omitempty"`
		}
		if err := json.Unmarshal(data, &flowData); err == nil {
			h.handleFlowCreate(client, flowData.Name, flowData.Description)
		}
	case "flow:update":
		var flowData struct {
			FlowID string                 `json:"flowId"`
			Flow   map[string]interface{} `json:"flow"`
		}
		if err := json.Unmarshal(data, &flowData); err == nil {
			h.handleFlowUpdate(client, flowData.FlowID, flowData.Flow)
		}
	case "flow:delete":
		var flowData struct{ FlowID string `json:"flowId"` }
		if err := json.Unmarshal(data, &flowData); err == nil {
			h.handleFlowDelete(client, flowData.FlowID)
		}
	case "flow:deploy":
		var flowData struct {
			FlowID string `json:"flowId"`
			Force  bool   `json:"force,omitempty"`
		}
		if err := json.Unmarshal(data, &flowData); err == nil {
			h.handleFlowDeploy(client, flowData.FlowID, flowData.Force)
		}
	case "flow:undeploy":
		var flowData struct{ FlowID string `json:"flowId"` }
		if err := json.Unmarshal(data, &flowData); err == nil {
			h.handleFlowUndeploy(client, flowData.FlowID)
		}

	// Node-related messages
	case "node:add":
		var nodeData struct {
			Node struct {
				ID       string                 `json:"id"`
				Type     string                 `json:"type"`
				Position struct {
					X float64 `json:"x"`
					Y float64 `json:"y"`
				} `json:"position"`
				Config map[string]interface{} `json:"config,omitempty"`
			} `json:"node"`
			FlowID string `json:"flowId"`
		}
		if err := json.Unmarshal(data, &nodeData); err == nil {
			log.Printf("[BACKEND] node:add received - flowId: %s, nodeId: %s, type: %s", nodeData.FlowID, nodeData.Node.ID, nodeData.Node.Type)
			h.handleNodeAdd(client, nodeData)
		} else {
			log.Printf("[BACKEND] node:add parse error: %v, data: %s", err, string(data))
		}
	case "node:remove":
		var nodeData struct {
			NodeID string `json:"nodeId"`
			FlowID string `json:"flowId"`
		}
		if err := json.Unmarshal(data, &nodeData); err == nil {
			h.handleNodeRemove(client, nodeData.NodeID, nodeData.FlowID)
		}
	case "node:update":
		var nodeData struct {
			Node struct {
				ID       string                 `json:"id"`
				Type     string                 `json:"type"`
				Position struct {
					X float64 `json:"x"`
					Y float64 `json:"y"`
				} `json:"position"`
				Config map[string]interface{} `json:"config,omitempty"`
			} `json:"node"`
			FlowID string `json:"flowId"`
		}
		if err := json.Unmarshal(data, &nodeData); err == nil {
			log.Printf("[BACKEND] node:update received - flowId: %s, nodeId: %s", nodeData.FlowID, nodeData.Node.ID)
			h.handleNodeUpdate(client, nodeData)
		} else {
			log.Printf("[BACKEND] node:update parse error: %v, data: %s", err, string(data))
		}
	case "node:config":
		var nodeData struct {
			NodeID string                 `json:"nodeId"`
			Config map[string]interface{} `json:"config"`
		}
		if err := json.Unmarshal(data, &nodeData); err == nil {
			h.handleNodeConfig(client, nodeData.NodeID, nodeData.Config)
		}
	case "node:status":
		var nodeData struct {
			NodeID string                 `json:"nodeId"`
			FlowID string                 `json:"flowId"`
			Status map[string]interface{} `json:"status"`
		}
		if err := json.Unmarshal(data, &nodeData); err == nil {
			h.BroadcastToClient(client, MessageTypeNodeStatus, nodeData)
		}

	// Connection-related messages
	case "connection:add":
		var connData struct {
			Connection struct {
				ID          string `json:"id,omitempty"`
				SourceNode  string `json:"sourceNode"`
				SourcePort  string `json:"sourcePort,omitempty"`
				TargetNode  string `json:"targetNode"`
				TargetPort  string `json:"targetPort,omitempty"`
			} `json:"connection"`
			FlowID string `json:"flowId"`
		}
		if err := json.Unmarshal(data, &connData); err == nil {
			log.Printf("[BACKEND] connection:add received - flowId: %s, sourceNode: %s, targetNode: %s", connData.FlowID, connData.Connection.SourceNode, connData.Connection.TargetNode)
			h.handleConnectionAdd(client, connData)
		} else {
			log.Printf("[BACKEND] connection:add parse error: %v, data: %s", err, string(data))
		}
	case "connection:remove":
		var connData struct {
			ConnectionID string `json:"connectionId"`
			FlowID      string `json:"flowId"`
		}
		if err := json.Unmarshal(data, &connData); err == nil {
			h.handleConnectionRemove(client, connData.ConnectionID, connData.FlowID)
		}

	// System messages
	case "ping":
		h.BroadcastToClient(client, MessageTypePong, map[string]string{"message": "pong"})
	case "state:sync":
		h.handleStateSync(client)
	case "message:log":
		// Message log from backend - this can be safely ignored or used for debugging
		log.Printf("[WEBSOCKET] Received message:log - data: %s", message.Data)
		// For now, just acknowledge receipt with an info message
		h.BroadcastToClient(client, MessageTypeInfo, map[string]interface{}{
			"type":    "message:log",
			"status":  "received",
			"message": "Message log acknowledged",
		})
	case "message:send":
		var msgData struct {
			FlowID  string                 `json:"flowId"`
			NodeID  string                 `json:"nodeId"`
			Payload map[string]interface{} `json:"payload"`
		}
		if err := json.Unmarshal(data, &msgData); err == nil {
			log.Printf("[WEBSOCKET] Handling message:send - flowId: %s, nodeId: %s, payload: %v", msgData.FlowID, msgData.NodeID, msgData.Payload)
			h.handleMessageSend(client, msgData.FlowID, msgData.NodeID, msgData.Payload)
		} else {
			log.Printf("[WEBSOCKET] message:send parse error: %v, data: %s", err, string(data))
			h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
				"error":   "failed to parse message send request",
				"message": err.Error(),
			})
		}

	// Unknown message type
	default:
		log.Printf("[WEBSOCKET] Unknown WebSocket message type: %s, data: %s", messageType, message.Data)
		errorResponse := map[string]interface{}{
			"error":   "unknown message type",
			"type":    messageType,
			"message": "Received unknown message type",
		}
		h.BroadcastToClient(client, MessageTypeError, errorResponse)
	}
}

// Flow handlers with full engine integration

func (h *WebSocketHandler) handleFlowList(client *Client) {
	log.Println("Handling flow:list request")
	flows := h.flowEngine.GetAllFlows()
	
	// Convert flows to summary format for the client
	type flowSummary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Status      string `json:"status"`
		NodeCount   int    `json:"nodeCount"`
		CreatedAt   string `json:"createdAt"`
		UpdatedAt   string `json:"updatedAt"`
	}
	
	summaries := make([]flowSummary, len(flows))
	for i, flow := range flows {
		summaries[i] = flowSummary{
			ID:          flow.ID,
			Name:        flow.Name,
			Description: flow.Description,
			Status:      string(flow.Status),
			NodeCount:   len(flow.Nodes),
			CreatedAt:   flow.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   flow.UpdatedAt.Format(time.RFC3339),
		}
	}
	
	h.BroadcastToClient(client, MessageTypeFlowList, map[string]interface{}{
		"flows": summaries,
	})
}

func (h *WebSocketHandler) handleFlowGet(client *Client, flowID string) {
	log.Printf("Handling flow:get request for flow: %s", flowID)
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  flowID,
			"message": err.Error(),
		})
		return
	}
	log.Printf("[BACKEND] handleFlowGet - Converting flow to frontend format")
	flowResponse := convertFlowToFrontend(flow)
	h.BroadcastToClient(client, MessageTypeFlowGet, flowResponse)
}

func (h *WebSocketHandler) handleFlowCreate(client *Client, name, description string) {
	log.Printf("[BACKEND] handleFlowCreate - Creating flow: name=%s, description=%s", name, description)
	flow, err := h.flowEngine.CreateFlow("", name)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to create flow",
			"name":    name,
			"message": err.Error(),
		})
		return
	}
	
	if description != "" {
		flow.Description = description
	}
	
	// Save the flow to state manager
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Convert flow to frontend format
	flowResponse := convertFlowToFrontend(flow)
	
	// Broadcast to client
	h.BroadcastToClient(client, MessageTypeFlowCreate, flowResponse)
}

func (h *WebSocketHandler) handleFlowUpdate(client *Client, flowID string, flowData map[string]interface{}) {
	log.Printf("[BACKEND] handleFlowUpdate - Updating flow: flowId=%s, data=%v", flowID, flowData)
	
	// Get the existing flow
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  flowID,
			"message": err.Error(),
		})
		return
	}
	
	// Apply updates from flowData to the flow
	if name, ok := flowData["name"].(string); ok && name != "" {
		flow.Name = name
	}
	if desc, ok := flowData["description"].(string); ok {
		flow.Description = desc
	}
	// Config update - TODO: Implement proper FlowConfig merging
	if flowConfig, ok := flowData["config"].(map[string]interface{}); ok {
		// For now, just log that config was received
		log.Printf("[BACKEND] handleFlowUpdate - Config update received (not implemented yet)")
		_ = flowConfig
	}
	
	// Update nodes if provided
	if nodes, ok := flowData["nodes"].(map[string]interface{}); ok {
		log.Printf("[BACKEND] handleFlowUpdate - Updating nodes: %v", nodes)
		// Clear existing nodes and add new ones
		flow.Nodes = make(map[string]*engine.Node)
		for id, nodeData := range nodes {
			if nodeMap, ok := nodeData.(map[string]interface{}); ok {
				node := &engine.Node{
					ID:       id,
					Type:     nodeMap["type"].(string),
					Name:     getString(nodeMap, "name"),
					X:        getFloat64(nodeMap, "position", "x"),
					Y:        getFloat64(nodeMap, "position", "y"),
					Config:   getConfig(nodeMap, "config"),
					Disabled: false,
				}
				flow.Nodes[id] = node
			}
		}
		log.Printf("[BACKEND] handleFlowUpdate - Updated %d nodes", len(flow.Nodes))
	}
	
	// Update connections if provided
	if connections, ok := flowData["connections"].([]interface{}); ok {
		log.Printf("[BACKEND] handleFlowUpdate - Updating connections: %v", connections)
		flow.Connections = make([]engine.NodeConnection, len(connections))
		for i, connData := range connections {
			if connMap, ok := connData.(map[string]interface{}); ok {
				conn := engine.NodeConnection{
					ID:          connMap["id"].(string),
					SourceNode:  connMap["sourceNode"].(string),
					SourcePort:  getString(connMap, "sourcePort"),
					TargetNode:  connMap["targetNode"].(string),
					TargetPort:  getString(connMap, "targetPort"),
				}
				flow.Connections[i] = conn
			}
		}
		log.Printf("[BACKEND] handleFlowUpdate - Updated %d connections", len(flow.Connections))
	}
	
	// Update the flow's updated timestamp
	flow.UpdatedAt = time.Now().UTC()
	
	// Save the flow to state manager
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Broadcast updates
	log.Printf("[BACKEND] handleFlowUpdate - Converting flow to frontend format")
	flowResponse := convertFlowToFrontend(flow)
	h.BroadcastToClient(client, MessageTypeFlowUpdate, flowResponse)
	h.hub.Broadcast(MessageTypeFlowList, h.flowEngine.GetAllFlowsSummary())
}

func (h *WebSocketHandler) handleFlowDelete(client *Client, flowID string) {
	log.Printf("[BACKEND] handleFlowDelete - Deleting flow: flowId=%s", flowID)
	
	// First undeploy the flow if it's active
	h.flowEngine.Undeploy(flowID)
	
	// Delete from state manager
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().DeleteFlow(flowID)
	}
	
	// Broadcast deletion
	h.hub.Broadcast(MessageTypeFlowDelete, map[string]interface{}{
		"flowId": flowID,
	})
	h.hub.Broadcast(MessageTypeFlowList, h.flowEngine.GetAllFlowsSummary())
}

func (h *WebSocketHandler) handleFlowDeploy(client *Client, flowID string, force bool) {
	log.Printf("Handling flow:deploy request for flow: %s (force: %v)", flowID, force)
	
	// Get the flow
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		// Flow might not be loaded yet, try to load it from state manager
		if h.flowEngine.GetStateManager() != nil {
			flow, err = h.flowEngine.GetStateManager().LoadFlow(flowID)
			if err != nil {
				h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
					"error":   "flow not found",
					"flowId":  flowID,
					"message": err.Error(),
				})
				return
			}
		} else {
			h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
				"error":   "flow not found",
				"flowId":  flowID,
				"message": err.Error(),
			})
			return
		}
	}
	
	// Deploy the flow
	if err := h.flowEngine.Deploy(flow); err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to deploy flow",
			"flowId":  flowID,
			"message": err.Error(),
		})
		return
	}
	
	// Broadcast success
	h.hub.Broadcast(MessageTypeFlowDeploy, map[string]interface{}{
		"flowId": flowID,
		"status": "deployed",
	})
	h.hub.Broadcast(MessageTypeFlowStatus, map[string]interface{}{
		"flowId": flowID,
		"status": "deployed",
	})
	h.hub.Broadcast(MessageTypeFlowList, h.flowEngine.GetAllFlowsSummary())
}

func (h *WebSocketHandler) handleFlowUndeploy(client *Client, flowID string) {
	log.Printf("Handling flow:undeploy request for flow: %s", flowID)
	
	if err := h.flowEngine.Undeploy(flowID); err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to undeploy flow",
			"flowId":  flowID,
			"message": err.Error(),
		})
		return
	}
	
	// Broadcast success
	h.hub.Broadcast(MessageTypeFlowUndeploy, map[string]interface{}{
		"flowId": flowID,
		"status": "undeployed",
	})
	h.hub.Broadcast(MessageTypeFlowStatus, map[string]interface{}{
		"flowId": flowID,
		"status": "inactive",
	})
	h.hub.Broadcast(MessageTypeFlowList, h.flowEngine.GetAllFlowsSummary())
}

func (h *WebSocketHandler) handleNodeAdd(client *Client, nodeData struct {
	Node struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Position struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"position"`
		Config map[string]interface{} `json:"config,omitempty"`
	} `json:"node"`
	FlowID string `json:"flowId"`
}) {
	log.Printf("[BACKEND] handleNodeAdd - flowId: %s, nodeId: %s, type: %s, position: (%.1f, %.1f)", 
		nodeData.FlowID, nodeData.Node.ID, nodeData.Node.Type, nodeData.Node.Position.X, nodeData.Node.Position.Y)
	
	// Get the flow
	flow, err := h.flowEngine.GetFlow(nodeData.FlowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  nodeData.FlowID,
			"message": err.Error(),
		})
		return
	}
	
	// Create new node - use the node ID from frontend
	node := &engine.Node{
		ID:       nodeData.Node.ID,
		Type:     nodeData.Node.Type,
		Config:   nodeData.Node.Config,
		X:        nodeData.Node.Position.X,
		Y:        nodeData.Node.Position.Y,
		Disabled:  false,
	}
	log.Printf("[BACKEND] handleNodeAdd - Created node with ID: %s, type: %s", node.ID, node.Type)
	
	// Add node to flow
	if err := flow.AddNode(node); err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to add node",
			"message": err.Error(),
		})
		return
	}
	
	// Save the flow
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Broadcast node addition
	h.hub.Broadcast(MessageTypeNodeAdd, map[string]interface{}{
		"flowId": nodeData.FlowID,
		"node": map[string]interface{}{
			"id":       node.ID,
			"type":     node.Type,
			"name":     node.Name,
			"config":   node.Config,
			"position": map[string]float64{"x": node.X, "y": node.Y},
			"status":   map[string]interface{}{"state": "idle"},
			"disabled":  node.Disabled,
		},
	})
}

func (h *WebSocketHandler) handleNodeRemove(client *Client, nodeID, flowID string) {
	log.Printf("Handling node:remove request for node: %s in flow: %s", nodeID, flowID)
	
	// Get the flow
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  flowID,
			"message": err.Error(),
		})
		return
	}
	
	// Remove node from flow
	if err := flow.RemoveNode(nodeID); err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to remove node",
			"message": err.Error(),
		})
		return
	}
	
	// Save the flow
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Broadcast node removal
	h.hub.Broadcast(MessageTypeNodeRemove, map[string]interface{}{
		"flowId": flowID,
		"nodeId": nodeID,
	})
}

func (h *WebSocketHandler) handleNodeUpdate(client *Client, nodeData struct {
	Node struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Position struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"position"`
		Config map[string]interface{} `json:"config,omitempty"`
	} `json:"node"`
	FlowID string `json:"flowId"`
}) {
	log.Printf("[BACKEND] handleNodeUpdate - flowId: %s, nodeId: %s", nodeData.FlowID, nodeData.Node.ID)
	
	// Get the flow
	flow, err := h.flowEngine.GetFlow(nodeData.FlowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  nodeData.FlowID,
			"message": err.Error(),
		})
		return
	}
	
	// Get the node
	node, exists := flow.Nodes[nodeData.Node.ID]
	if !exists {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "node not found",
			"nodeId":  nodeData.Node.ID,
			"message": "node not found in flow",
		})
		return
	}
	
	// Apply updates from the node data
	node.Type = nodeData.Node.Type
	node.Config = nodeData.Node.Config
	node.X = nodeData.Node.Position.X
	node.Y = nodeData.Node.Position.Y
	
	// Update flow timestamp
	flow.UpdatedAt = time.Now().UTC()
	
	// Save the flow
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Broadcast node update
	h.hub.Broadcast(MessageTypeNodeUpdate, map[string]interface{}{
		"flowId": nodeData.FlowID,
		"nodeId": nodeData.Node.ID,
		"node": map[string]interface{}{
			"id":       node.ID,
			"type":     node.Type,
			"name":     node.Name,
			"config":   node.Config,
			"position": map[string]float64{"x": node.X, "y": node.Y},
			"status":   map[string]interface{}{"state": "idle"},
			"disabled":  node.Disabled,
		},
	})
}

func (h *WebSocketHandler) handleNodeConfig(client *Client, nodeID string, config map[string]interface{}) {
	log.Printf("Handling node:config request for node: %s with config: %+v", nodeID, config)
	
	// Broadcast configuration update
	h.hub.Broadcast(MessageTypeNodeConfig, map[string]interface{}{
		"nodeId": nodeID,
		"config": config,
	})
}

func (h *WebSocketHandler) handleConnectionAdd(client *Client, connData struct {
	Connection struct {
		ID          string `json:"id,omitempty"`
		SourceNode  string `json:"sourceNode"`
		SourcePort  string `json:"sourcePort,omitempty"`
		TargetNode  string `json:"targetNode"`
		TargetPort  string `json:"targetPort,omitempty"`
	} `json:"connection"`
	FlowID string `json:"flowId"`
}) {
	log.Printf("[BACKEND] handleConnectionAdd - flowId: %s, sourceNode: %s, targetNode: %s", 
		connData.FlowID, connData.Connection.SourceNode, connData.Connection.TargetNode)
	
	// Get the flow
	flow, err := h.flowEngine.GetFlow(connData.FlowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  connData.FlowID,
			"message": err.Error(),
		})
		return
	}
	
	// Create connection
	conn := engine.NodeConnection{
		ID:          connData.Connection.ID,
		SourceNode:  connData.Connection.SourceNode,
		SourcePort:  connData.Connection.SourcePort,
		TargetNode:  connData.Connection.TargetNode,
		TargetPort:  connData.Connection.TargetPort,
	}
	log.Printf("[BACKEND] handleConnectionAdd - Creating connection: %s -> %s", connData.Connection.SourceNode, connData.Connection.TargetNode)
	
	// Add connection to flow
	if err := flow.AddConnection(conn); err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to add connection",
			"message": err.Error(),
		})
		return
	}
	
	// Save the flow
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Broadcast connection addition
	h.hub.Broadcast(MessageTypeConnectionAdd, map[string]interface{}{
		"flowId":      connData.FlowID,
		"connection":  conn,
	})
}

func (h *WebSocketHandler) handleConnectionRemove(client *Client, connectionID, flowID string) {
	log.Printf("Handling connection:remove request for connection: %s in flow: %s", connectionID, flowID)
	
	// Get the flow
	flow, err := h.flowEngine.GetFlow(flowID)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "flow not found",
			"flowId":  flowID,
			"message": err.Error(),
		})
		return
	}
	
	// Remove connection from flow
	if err := flow.RemoveConnection(connectionID); err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to remove connection",
			"message": err.Error(),
		})
		return
	}
	
	// Save the flow
	if h.flowEngine.GetStateManager() != nil {
		h.flowEngine.GetStateManager().SaveFlow(flow)
	}
	
	// Broadcast connection removal
	h.hub.Broadcast(MessageTypeConnectionRemove, map[string]interface{}{
		"flowId":        flowID,
		"connectionId": connectionID,
	})
}

func (h *WebSocketHandler) handleStateSync(client *Client) {
	log.Println("Handling state:sync request")
	
	// Send full state to the client
	flows := h.flowEngine.GetAllFlows()
	nodes := h.nodeRegistry.GetAllNodes()
	
	state := map[string]interface{}{
		"flows":     flows,
		"nodeTypes": nodes,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	
	h.BroadcastToClient(client, MessageTypeStateSync, state)
}

func (h *WebSocketHandler) handleMessageSend(client *Client, flowID, nodeID string, payload map[string]interface{}) {
	log.Printf("Handling message:send request - flowId: %s, nodeId: %s", flowID, nodeID)
	
	// Inject the message into the specified flow at the specified node
	err := h.flowEngine.InjectMessage(flowID, nodeID, payload)
	if err != nil {
		h.BroadcastToClient(client, MessageTypeError, map[string]interface{}{
			"error":   "failed to inject message",
			"flowId":  flowID,
			"nodeId":  nodeID,
			"message": err.Error(),
		})
		return
	}
	
	// Broadcast success
	h.BroadcastToClient(client, MessageTypeMessageSend, map[string]interface{}{
		"status":  "sent",
		"flowId":  flowID,
		"nodeId":  nodeID,
		"message": "Message injected successfully",
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

// generateID generates a unique ID for nodes
func generateID() string {
	return "" + time.Now().UTC().Format("20060102150405000000000")
}

// GetHub returns the underlying hub
func (h *WebSocketHandler) GetHub() *Hub {
	return h.hub
}

// convertFlowToFrontend converts a Go flow to frontend-compatible format
func convertFlowToFrontend(flow *engine.Flow) map[string]interface{} {
	log.Printf("[BACKEND] convertFlowToFrontend - Converting flow %s with %d nodes", flow.ID, len(flow.Nodes))
	
	// Convert nodes
	nodesMap := make(map[string]interface{})
	for id, node := range flow.Nodes {
		nodeMap := map[string]interface{}{
			"id":       node.ID,
			"type":     node.Type,
			"name":     node.Name,
			"position": map[string]float64{"x": node.X, "y": node.Y},
			"config":   node.Config,
			"status":   convertNodeStatus(node),
			"disabled":  node.Disabled,
		}
		nodesMap[id] = nodeMap
		log.Printf("[BACKEND] convertFlowToFrontend - Converted node %s: position=(%.1f, %.1f)", id, node.X, node.Y)
	}
	
	// Convert connections
	connectionsList := make([]interface{}, len(flow.Connections))
	for i, conn := range flow.Connections {
		connMap := map[string]interface{}{
			"id":           conn.ID,
			"sourceNode":   conn.SourceNode,
			"sourcePort":   conn.SourcePort,
			"targetNode":   conn.TargetNode,
			"targetPort":   conn.TargetPort,
		}
		connectionsList[i] = connMap
	}
	
	// Convert flow config
	flowConfigMap := map[string]interface{}{
		"timeout":        flow.Config.Timeout.String(),
		"maxConcurrency": flow.Config.MaxConcurrency,
		"retryPolicy": map[string]interface{}{
			"maxRetries":  flow.Config.RetryPolicy.MaxRetries,
			"backoff":     flow.Config.RetryPolicy.Backoff.String(),
			"maxBackoff":  flow.Config.RetryPolicy.MaxBackoff.String(),
			"retryOn":     flow.Config.RetryPolicy.RetryOn,
		},
		"environment": flow.Config.Environment,
	}
	
	// Convert flow status
	frontendStatus := convertFlowStatus(flow.Status)
	
	return map[string]interface{}{
		"id":          flow.ID,
		"name":        flow.Name,
		"description": flow.Description,
		"nodes":       nodesMap,
		"connections": connectionsList,
		"status":      frontendStatus,
		"config":      flowConfigMap,
		"createdAt":   flow.CreatedAt.Format(time.RFC3339),
		"updatedAt":   flow.UpdatedAt.Format(time.RFC3339),
		"version":     flow.Version,
	}
}

// convertFlowStatus converts Go flow status to frontend status
func convertFlowStatus(status engine.FlowStatus) string {
	switch status {
	case engine.FlowStatusInactive:
		return "stopped"
	case engine.FlowStatusActive:
		return "running"
	case engine.FlowStatusError:
		return "error"
	case engine.FlowStatusDeploying:
		return "deploying"
	case engine.FlowStatusUndeploying:
		return "undeploying"
	default:
		return string(status)
	}
}

// convertNodeStatus converts Go node to frontend node status
func convertNodeStatus(node *engine.Node) map[string]interface{} {
	// For now, return a basic status based on disabled state
	// This can be enhanced later with actual node status tracking
	if node.Disabled {
		return map[string]interface{}{
			"state": "idle",
			"message": "Node is disabled",
		}
	}
	return map[string]interface{}{
		"state": "idle",
	}
}

// ServeWebSocket serves WebSocket connections
// Helper functions for type-safe map access
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getFloat64(m map[string]interface{}, positionKey, coordKey string) float64 {
	if pos, ok := m[positionKey].(map[string]interface{}); ok {
		if val, ok := pos[coordKey].(float64); ok {
			return val
		}
	}
	return 0
}

func getConfig(m map[string]interface{}, configKey string) map[string]interface{} {
	if val, ok := m[configKey].(map[string]interface{}); ok {
		return val
	}
	return make(map[string]interface{})
}

func (h *WebSocketHandler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeWebSocket(w, r, h.HandleMessage)
}
