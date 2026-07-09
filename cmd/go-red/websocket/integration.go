package websocket

import (
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/GrimbiXcode/Go-RED/internal/dto"
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
            FlowID string                `json:"flowId"`
            Flow   dto.FlowUpdateRequest `json:"flow"`
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
            Node   dto.Node `json:"node"`
            FlowID string   `json:"flowId"`
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
            Node   dto.Node `json:"node"`
            FlowID string   `json:"flowId"`
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
            Connection dto.Connection `json:"connection"`
            FlowID     string         `json:"flowId"`
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
        var logRequest struct {
            FlowID string `json:"flowId"`
            Limit  int    `json:"limit"`
        }
        if err := json.Unmarshal(data, &logRequest); err == nil {
            h.handleMessageLog(client, logRequest.FlowID, logRequest.Limit)
        } else {
            log.Printf("[WEBSOCKET] message:log parse error: %v, data: %s", err, string(data))
        }
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

func (h *WebSocketHandler) handleFlowList(client *Client) {
    log.Println("Handling flow:list request")
    h.BroadcastToClient(client, MessageTypeFlowList, map[string]interface{}{
        "flows": h.allFlowSummaries(),
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
    h.BroadcastToClient(client, MessageTypeFlowGet, dto.ToWire(flow))
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

    // Broadcast to client
    h.BroadcastToClient(client, MessageTypeFlowCreate, dto.ToWire(flow))
}

func (h *WebSocketHandler) handleFlowUpdate(client *Client, flowID string, req dto.FlowUpdateRequest) {
    log.Printf("[BACKEND] handleFlowUpdate - Updating flow: flowId=%s", flowID)

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

    req.ApplyTo(flow)
    log.Printf("[BACKEND] handleFlowUpdate - Flow now has %d nodes, %d connections", len(flow.Nodes), len(flow.Connections))

    // Save the flow to state manager
    if h.flowEngine.GetStateManager() != nil {
        h.flowEngine.GetStateManager().SaveFlow(flow)
    }

    // Broadcast updates
    h.BroadcastToClient(client, MessageTypeFlowUpdate, dto.ToWire(flow))
    h.hub.Broadcast(MessageTypeFlowList, map[string]interface{}{
        "flows": h.allFlowSummaries(),
    })
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
    h.hub.Broadcast(MessageTypeFlowList, map[string]interface{}{"flows": h.allFlowSummaries()})
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
    h.hub.Broadcast(MessageTypeFlowList, map[string]interface{}{"flows": h.allFlowSummaries()})
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
    h.hub.Broadcast(MessageTypeFlowList, map[string]interface{}{"flows": h.allFlowSummaries()})
}

func (h *WebSocketHandler) handleNodeAdd(client *Client, nodeData struct {
    Node   dto.Node `json:"node"`
    FlowID string   `json:"flowId"`
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
    node := dto.NodeFromWire(nodeData.Node.ID, nodeData.Node)
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
        "node":   dto.NodeToWire(node),
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
    Node   dto.Node `json:"node"`
    FlowID string   `json:"flowId"`
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
    node.Name = nodeData.Node.Name
    node.Config = nodeData.Node.Config
    node.X = nodeData.Node.Position.X
    node.Y = nodeData.Node.Position.Y
    node.Disabled = nodeData.Node.Disabled

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
        "node":   dto.NodeToWire(node),
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
    Connection dto.Connection `json:"connection"`
    FlowID     string         `json:"flowId"`
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
    conn := dto.ConnectionFromWire(connData.Connection)
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
        "flowId":     connData.FlowID,
        "connection": dto.ConnectionToWire(conn),
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

    // Send full state to the client, using the same canonical wire Flow
    // shape as every other endpoint (previously this sent raw engine.Flow
    // objects, a distinct shape from flow:get/flow:list).
    engineFlows := h.flowEngine.GetAllFlows()
    flows := make([]dto.Flow, len(engineFlows))
    for i, f := range engineFlows {
        flows[i] = dto.ToWire(f)
    }
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

// handleMessageLog answers a message:log request with the actual logged
// messages (optionally filtered by flowId, optionally capped by limit).
// Previously this only acknowledged receipt without returning any data,
// even though the frontend (useMessageLog.ts) already expected a real
// {messages: [...]} response on this same message type.
func (h *WebSocketHandler) handleMessageLog(client *Client, flowID string, limit int) {
    log.Printf("Handling message:log request - flowId: %s, limit: %d", flowID, limit)

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

// generateID generates a unique ID for nodes
func generateID() string {
    return "" + time.Now().UTC().Format("20060102150405000000000")
}

// GetHub returns the underlying hub
func (h *WebSocketHandler) GetHub() *Hub {
    return h.hub
}

func (h *WebSocketHandler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
    h.hub.ServeWebSocket(w, r, h.HandleMessage)
}
