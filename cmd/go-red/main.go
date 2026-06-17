// Package main is the entry point for the Go—RED application.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
	defer flowEngine.Stop()

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
	
	var request struct {
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
		Nodes       map[string]interface{} `json:"nodes,omitempty"`
		Connections []interface{} `json:"connections,omitempty"`
		Config      map[string]interface{} `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Get existing flow
	flow, err := e.GetFlow(flowID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	// Update flow
	if request.Name != "" {
		flow.Name = request.Name
	}
	if request.Description != "" {
		flow.Description = request.Description
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
