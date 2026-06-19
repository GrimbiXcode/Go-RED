// Package main is the entry point for the Go—RED application.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/GrimbiXcode/Go-RED/cmd/go-red/websocket"
	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/state"

	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
)

type Config struct {
	Port int
	DataDir string
	PluginDir string
	WebUIDir string
	MaxWorkers int
	MaxMessages int
}

func main() {
	config := parseFlags()
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting Go—RED...")
	
	nodeRegistry := registry.GetGlobalRegistry()
	log.Printf("Node registry initialized with %d node types", len(nodeRegistry.GetAllNodes()))
	
	stateManager, err := state.NewFileStateManager(config.DataDir)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	
	flowEngine := engine.NewFlowEngine(engine.EngineConfig{
		WorkerPoolSize: 100,
		MessageBufferSize: 1000,
		DefaultTimeout: 30 * time.Second,
		MaxRetries: 3,
		RetryBackoff: 1 * time.Second,
	}, nodeRegistry)
	
	flowEngine.SetStateManager(stateManager)
	
	if err := flowEngine.LoadAllFlows(); err != nil {
		log.Printf("Warning: Failed to load existing flows: %v", err)
	}
	
	if err := flowEngine.Start(); err != nil {
		log.Fatalf("Failed to start flow engine: %v", err)
	}

	// Initialize WebSocket hub and handler
	wsHub := websocket.NewHub()
	wsHandler := websocket.NewWebSocketHandler(wsHub, flowEngine, nodeRegistry)
	go wsHub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/flows", func(w http.ResponseWriter, r *http.Request) {
		handleGetFlows(w, r, flowEngine)
	})
	mux.HandleFunc("POST /api/flows", func(w http.ResponseWriter, r *http.Request) {
		handleCreateFlow(w, r, flowEngine)
	})
	mux.HandleFunc("GET /api/flows/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleGetFlow(w, r, flowEngine)
	})
	mux.HandleFunc("PUT /api/flows/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleUpdateFlow(w, r, flowEngine)
	})
	mux.HandleFunc("DELETE /api/flows/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleDeleteFlow(w, r, flowEngine)
	})
	mux.HandleFunc("POST /api/flows/{id}/deploy", func(w http.ResponseWriter, r *http.Request) {
		handleDeployFlow(w, r, flowEngine)
	})
	mux.HandleFunc("POST /api/flows/{id}/undeploy", func(w http.ResponseWriter, r *http.Request) {
		handleUndeployFlow(w, r, flowEngine)
	})
	mux.HandleFunc("GET /api/nodes", func(w http.ResponseWriter, r *http.Request) {
		handleGetNodes(w, r, nodeRegistry)
	})
	mux.HandleFunc("GET /api/nodes/{type}", func(w http.ResponseWriter, r *http.Request) {
		handleGetNode(w, r, nodeRegistry)
	})
	mux.HandleFunc("GET /api/messages", func(w http.ResponseWriter, r *http.Request) {
		handleGetMessages(w, r, flowEngine)
	})
	mux.HandleFunc("GET /api/flows/{id}/export", func(w http.ResponseWriter, r *http.Request) {
		handleExportFlow(w, r, flowEngine)
	})
	mux.HandleFunc("POST /api/flows/import", func(w http.ResponseWriter, r *http.Request) {
		handleImportFlow(w, r, flowEngine)
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler.ServeWebSocket(w, r)
	})
	mux.Handle("/", http.FileServer(http.Dir(config.WebUIDir)))

	server := &http.Server{Addr: ":" + strconv.Itoa(config.Port), Handler: mux}
	
	go func() {
		log.Printf("Server listening on port %d", config.Port)
		log.Printf("WebSocket available at ws://localhost:%d/ws", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	flowEngine.Stop()
	server.Shutdown(ctx)
	
	activeFlows := flowEngine.GetAllFlows()
	for _, flow := range activeFlows {
		stateManager.SaveFlow(flow)
	}
	log.Println("Shutdown complete")
}

func parseFlags() Config {
	var config Config
	flag.IntVar(&config.Port, "port", 8080, "Port to listen on")
	flag.StringVar(&config.DataDir, "data-dir", "data", "Directory for flow data")
	flag.StringVar(&config.PluginDir, "plugin-dir", "plugins", "Directory for plugins")
	flag.StringVar(&config.WebUIDir, "web-dir", "web/dist", "Directory for WebUI")
	flag.IntVar(&config.MaxWorkers, "max-workers", 100, "Maximum number of worker goroutines")
	flag.IntVar(&config.MaxMessages, "max-messages", 1000, "Maximum message buffer size")
	flag.Parse()
	return config
}

func handleGetFlows(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flows := e.GetAllFlows()
	type flowResponse struct {
		ID string `json:"id"`
		Name string `json:"name"`
		Description string `json:"description"`
		Status engine.FlowStatus `json:"status"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	response := make([]flowResponse, len(flows))
	for i, flow := range flows {
		response[i] = flowResponse{ID: flow.ID, Name: flow.Name, Description: flow.Description, Status: flow.Status, CreatedAt: flow.CreatedAt, UpdatedAt: flow.UpdatedAt}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleCreateFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	var request struct {
		ID string `json:"id"`
		Name string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	flow, err := e.CreateFlow(request.ID, request.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if request.Description != "" {
		flow.Description = request.Description
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(flow)
}

func handleGetFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flowID := r.PathValue("id")
	flow, err := e.GetFlow(flowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flow)
}

func handleUpdateFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flowID := r.PathValue("id")
	
	// First, read the raw request body to handle custom JSON structure
	var rawBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Get existing flow
	flow, err := e.GetFlow(flowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Update flow fields
	if name, ok := rawBody["name"].(string); ok && name != "" {
		flow.Name = name
	}
	if description, ok := rawBody["description"].(string); ok && description != "" {
		flow.Description = description
	}
	
	// Update nodes - handle frontend position object
	if nodes, ok := rawBody["nodes"].(map[string]interface{}); ok && nodes != nil {
		for nodeID, nodeData := range nodes {
			if nodeMap, ok := nodeData.(map[string]interface{}); ok {
				// Check if node exists
				if existingNode, exists := flow.Nodes[nodeID]; exists {
					// Update existing node
					if nodeType, ok := nodeMap["type"].(string); ok {
						existingNode.Type = nodeType
					}
					if config, ok := nodeMap["config"].(map[string]interface{}); ok {
						existingNode.Config = config
					}
					if position, ok := nodeMap["position"].(map[string]interface{}); ok {
						if x, ok := position["x"].(float64); ok {
							existingNode.X = x
						}
						if y, ok := position["y"].(float64); ok {
							existingNode.Y = y
						}
					}
					if disabled, ok := nodeMap["disabled"].(bool); ok {
						existingNode.Disabled = disabled
					}
				} else {
					// Create new node
					newNode := &engine.Node{
						ID: nodeID,
					}
					if nodeType, ok := nodeMap["type"].(string); ok {
						newNode.Type = nodeType
					}
					if config, ok := nodeMap["config"].(map[string]interface{}); ok {
						newNode.Config = config
					}
					if position, ok := nodeMap["position"].(map[string]interface{}); ok {
						if x, ok := position["x"].(float64); ok {
							newNode.X = x
						}
						if y, ok := position["y"].(float64); ok {
							newNode.Y = y
						}
					}
					if disabled, ok := nodeMap["disabled"].(bool); ok {
						newNode.Disabled = disabled
					}
					flow.Nodes[nodeID] = newNode
				}
			}
		}
	}
	
	// Update connections
	if connections, ok := rawBody["connections"].([]interface{}); ok && connections != nil {
		var newConnections []engine.NodeConnection
		for _, connData := range connections {
			if connMap, ok := connData.(map[string]interface{}); ok {
				newConn := engine.NodeConnection{}
				if id, ok := connMap["id"].(string); ok {
					newConn.ID = id
				}
				if sourceNode, ok := connMap["sourceNode"].(string); ok {
					newConn.SourceNode = sourceNode
				}
				if sourcePort, ok := connMap["sourcePort"].(string); ok {
					newConn.SourcePort = sourcePort
				}
				if targetNode, ok := connMap["targetNode"].(string); ok {
					newConn.TargetNode = targetNode
				}
				if targetPort, ok := connMap["targetPort"].(string); ok {
					newConn.TargetPort = targetPort
				}
				newConnections = append(newConnections, newConn)
			}
		}
		flow.Connections = newConnections
	}
	
	// Update config
	if config, ok := rawBody["config"].(map[string]interface{}); ok && config != nil {
		if timeout, ok := config["timeout"].(float64); ok {
			flow.Config.Timeout = time.Duration(timeout) * time.Second
		}
		if maxMessages, ok := config["maxMessages"].(float64); ok {
			// Frontend sends maxMessages, backend uses MaxConcurrency
			flow.Config.MaxConcurrency = int(maxMessages)
		}
		if maxConcurrency, ok := config["maxConcurrency"].(float64); ok {
			flow.Config.MaxConcurrency = int(maxConcurrency)
		}
		if env, ok := config["environment"].(map[string]interface{}); ok {
			for k, v := range env {
				if strVal, ok := v.(string); ok {
					flow.Config.Environment[k] = strVal
				}
			}
		}
		// Handle autoDeploy if present (frontend-specific)
		if autoDeploy, ok := config["autoDeploy"].(bool); ok {
			// Could set some flag or auto-deploy, but for now just ignore
			_ = autoDeploy
		}
	}
	
	flow.UpdatedAt = time.Now().UTC()
	
	// Save the flow
	if e.GetStateManager() != nil {
		e.GetStateManager().SaveFlow(flow)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(flow)
}

func handleDeleteFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flowID := r.PathValue("id")
	
	// First undeploy the flow if it's active
	e.Undeploy(flowID)
	
	// Delete from state manager
	if e.GetStateManager() != nil {
		e.GetStateManager().DeleteFlow(flowID)
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "deleted", "flowId": flowID})
}

func handleDeployFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flowID := r.PathValue("id")
	flow, err := e.GetFlow(flowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := e.Deploy(flow); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "deployed", "flowId": flowID})
}

func handleUndeployFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flowID := r.PathValue("id")
	if err := e.Undeploy(flowID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "undeployed", "flowId": flowID})
}

func handleGetNodes(w http.ResponseWriter, r *http.Request, reg *registry.NodeRegistry) {
	nodes := reg.GetAllNodes()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func handleGetNode(w http.ResponseWriter, r *http.Request, reg *registry.NodeRegistry) {
	nodeType := r.PathValue("type")
	metadata, err := reg.GetMetadata(nodeType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metadata)
}

func handleGetMessages(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	// Get query parameters for filtering
	flowID := r.URL.Query().Get("flowId")
	limitStr := r.URL.Query().Get("limit")
	
	var messages []engine.Message
	
	if flowID != "" {
		// Get messages for specific flow
		messages = e.GetMessageLogForFlow(flowID)
	} else {
		// Get all messages
		messages = e.GetMessageLog()
	}
	
	// Apply limit if specified
	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil && limit > 0 {
			startIndex := len(messages) - limit
			if startIndex < 0 {
				startIndex = 0
			}
			messages = messages[startIndex:]
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func handleExportFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	flowID := r.PathValue("id")
	
	// Get the flow
	flow, err := e.GetFlow(flowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Set headers for file download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=flow-%s.json", flowID))
	
	// Create export structure
	exportData := map[string]interface{}{
		"id":          flow.ID,
		"name":        flow.Name,
		"description": flow.Description,
		"nodes":       flow.Nodes,
		"connections": flow.Connections,
		"config":      flow.Config,
		"createdAt":   flow.CreatedAt.Format(time.RFC3339),
		"updatedAt":   flow.UpdatedAt.Format(time.RFC3339),
		"status":      flow.Status,
	}
	
	json.NewEncoder(w).Encode(exportData)
}

func handleImportFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
	// Only accept POST requests
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Parse the request body
	var importData struct {
		ID          string                 `json:"id"`
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Nodes       map[string]*engine.Node `json:"nodes"`
		Connections []engine.NodeConnection `json:"connections"`
		Config      map[string]interface{}  `json:"config"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Validate required fields
	if importData.Name == "" {
		http.Error(w, "Flow name is required", http.StatusBadRequest)
		return
	}
	
	// Create a new flow with a new ID (to avoid conflicts)
	// But keep the original ID for reference in the response
	originalID := importData.ID
	importData.ID = ""
	
	// Create the flow with a new UUID
	flow := engine.NewFlow(uuid.New().String(), importData.Name)
	flow.Description = importData.Description
	
	// Import nodes
	for nodeID, node := range importData.Nodes {
		// Create a copy to avoid pointer issues
		newNode := *node
		newNode.ID = nodeID
		flow.Nodes[nodeID] = &newNode
	}
	
	// Import connections
	for _, conn := range importData.Connections {
		flow.Connections = append(flow.Connections, conn)
	}
	
	// Import config - convert from map[string]interface{} to FlowConfig
	// For now, keep the default FlowConfig from NewFlow as the import format
	// uses a generic map which may not match the FlowConfig structure exactly
	// This can be enhanced later with proper type conversion
	if importData.Config != nil {
		// Try to convert config values if they match the expected types
		if timeout, ok := importData.Config["timeout"].(float64); ok {
			flow.Config.Timeout = time.Duration(timeout) * time.Second
		}
		if maxConcurrency, ok := importData.Config["maxConcurrency"].(float64); ok {
			flow.Config.MaxConcurrency = int(maxConcurrency)
		}
		if env, ok := importData.Config["environment"].(map[string]interface{}); ok {
			for k, v := range env {
				if strVal, ok := v.(string); ok {
					flow.Config.Environment[k] = strVal
				}
			}
		}
	}
	
	// Save the flow to state manager
	if e.GetStateManager() != nil {
		if err := e.GetStateManager().SaveFlow(flow); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save flow: %v", err), http.StatusInternalServerError)
			return
		}
	}
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "imported",
		"flowId":    flow.ID,
		"originalId": originalID,
		"name":      flow.Name,
		"message":   "Flow imported successfully",
	})
}
