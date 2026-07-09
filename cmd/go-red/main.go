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
    "strings"
    "syscall"
    "time"

    "github.com/google/uuid"

    "github.com/GrimbiXcode/Go-RED/cmd/go-red/websocket"
    "github.com/GrimbiXcode/Go-RED/internal/dto"
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

// sanitizeString strips null bytes and caps the length of user-supplied
// strings before they are stored or echoed back to a client.
func sanitizeString(s string) string {
    s = strings.ReplaceAll(s, "\x00", "")
    if len(s) > 10000 {
        s = s[:10000]
    }
    return s
}

// writeError logs the full error server-side and returns only a generic,
// safe message to the client so internal details (file paths, wrapped
// errors, etc.) are never exposed over the API.
func writeError(w http.ResponseWriter, status int, publicMessage string, err error) {
    if err != nil {
        log.Printf("[ERROR] %s: %v", publicMessage, err)
    }
    http.Error(w, publicMessage, status)
}

func handleGetFlows(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    flows := e.GetAllFlows()
    response := make([]dto.FlowSummary, len(flows))
    for i, flow := range flows {
        response[i] = dto.ToWireSummary(flow)
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func handleCreateFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    var request dto.FlowCreateRequest
    if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body", err)
        return
    }
    request.Name = sanitizeString(request.Name)
    request.Description = sanitizeString(request.Description)
    flow, err := e.CreateFlow(request.ID, request.Name)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create flow", err)
        return
    }
    if request.Description != "" {
        flow.Description = request.Description
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(dto.ToWire(flow))
}

func handleGetFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    flowID := r.PathValue("id")
    flow, err := e.GetFlow(flowID)
    if err != nil {
        writeError(w, http.StatusNotFound, "flow not found", err)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(dto.ToWire(flow))
}

func handleUpdateFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    flowID := r.PathValue("id")

    var request dto.FlowUpdateRequest
    if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body", err)
        return
    }
    if request.Name != nil {
        sanitized := sanitizeString(*request.Name)
        request.Name = &sanitized
    }
    if request.Description != nil {
        sanitized := sanitizeString(*request.Description)
        request.Description = &sanitized
    }
    for id, node := range request.Nodes {
        node.Type = sanitizeString(node.Type)
        node.Name = sanitizeString(node.Name)
        request.Nodes[id] = node
    }

    // Get existing flow
    flow, err := e.GetFlow(flowID)
    if err != nil {
        writeError(w, http.StatusNotFound, "flow not found", err)
        return
    }

    request.ApplyTo(flow)

    // Save the flow
    if e.GetStateManager() != nil {
        e.GetStateManager().SaveFlow(flow)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(dto.ToWire(flow))
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
    log.Printf("[REST API] Deploy request for flow: %s", flowID)
    
    // Check if flow is already deployed
    if existingFlow, err := e.GetFlow(flowID); err == nil && existingFlow != nil {
        log.Printf("[REST API] Flow %s is already deployed, returning success", flowID)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":  "deployed",
            "flowId":  flowID,
            "message": "Flow was already deployed",
        })
        return
    }
    
    // Flow not in memory, try to load from state manager
    var flow *engine.Flow
    if e.GetStateManager() != nil {
        var err error
        flow, err = e.GetStateManager().LoadFlow(flowID)
        if err != nil {
            log.Printf("[REST API] Failed to load flow %s from state manager: %v", flowID, err)
            http.Error(w, "flow not found", http.StatusNotFound)
            return
        }
        log.Printf("[REST API] Loaded flow %s from state manager, deploying...", flowID)
    } else {
        log.Printf("[REST API] No state manager configured")
        http.Error(w, "state manager not configured", http.StatusInternalServerError)
        return
    }
    
    // Deploy the flow
    log.Printf("[REST API] Deploying flow %s with %d nodes and %d connections", flowID, len(flow.Nodes), len(flow.Connections))
    if err := e.Deploy(flow); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to deploy flow "+flowID, err)
        return
    }
    
    log.Printf("[REST API] Flow %s deployed successfully via REST API", flowID)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{"status": "deployed", "flowId": flowID})
}

func handleUndeployFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    flowID := r.PathValue("id")
    if err := e.Undeploy(flowID); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to undeploy flow "+flowID, err)
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
        writeError(w, http.StatusNotFound, "node type not found", err)
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
    json.NewEncoder(w).Encode(dto.MessagesToWire(messages))
}

func handleExportFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    flowID := r.PathValue("id")

    // Get the flow
    flow, err := e.GetFlow(flowID)
    if err != nil {
        writeError(w, http.StatusNotFound, "flow not found", err)
        return
    }

    // Set headers for file download
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=flow-%s.json", flowID))

    json.NewEncoder(w).Encode(dto.ToWire(flow))
}

func handleImportFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    // Only accept POST requests
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Parse the request body as a canonical wire Flow (the same shape
    // produced by GET /api/flows/{id}/export).
    var importData dto.Flow
    if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body", err)
        return
    }

    importData.Name = sanitizeString(importData.Name)
    importData.Description = sanitizeString(importData.Description)

    // Validate required fields
    if importData.Name == "" {
        http.Error(w, "Flow name is required", http.StatusBadRequest)
        return
    }

    // Create a new flow with a new ID (to avoid conflicts)
    // But keep the original ID for reference in the response
    originalID := importData.ID

    // Create the flow with a new UUID
    flow := engine.NewFlow(uuid.New().String(), importData.Name)
    dto.PopulateFromWire(flow, importData)

    for nodeID, node := range flow.Nodes {
        node.Type = sanitizeString(node.Type)
        node.Name = sanitizeString(node.Name)
        flow.Nodes[nodeID] = node
    }

    // Save the flow to state manager
    if e.GetStateManager() != nil {
        if err := e.GetStateManager().SaveFlow(flow); err != nil {
            writeError(w, http.StatusInternalServerError, "failed to save flow", err)
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
