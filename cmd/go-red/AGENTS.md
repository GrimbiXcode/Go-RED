# Go-RED Main Application Guidelines

This file contains **application-specific** guidelines for the main Go-RED application in `cmd/go-red/`.

---

## Package Overview

The `cmd/go-red/` directory contains the **main application entry point** and related infrastructure:

```
cmd/go-red/
├── main.go           # Main entry point, HTTP server, configuration
└── websocket/        # WebSocket communication layer
    ├── hub.go         # WebSocket hub - manages all connections
    └── integration.go # WebSocket message handlers
```

This is the **executable** part of Go-RED - everything that runs when you execute `go run cmd/go-red/main.go` or the built binary.

---

## Architecture

### Main Components

```
┌─────────────────────────────────────────────────────────────┐
│                     HTTP Server (main.go)                      │
├─────────────────────────────────────────────────────────────┤
│  /api/*           REST API (Flows, Nodes, Messages)             │
│  /ws              WebSocket connection                         │
│  /                Static file server (Web UI)                  │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ↓                     ↓                     ↓
┌─────────────┐      ┌──────────────┐    ┌──────────────────┐
│ Flow Engine  │◄─────│ WebSocket Hub │    │ Static File Server│
│ (internal/)  │      │ (websocket/)  │    │ (http.FileServer) │
└─────────────┘      └──────────────┘    └──────────────────┘
        ▲
        │
┌─────────────────────┐
│   Node Registry      │
│  (internal/registry) │
└─────────────────────┘
        ▲
        │
┌─────────────────────┐
│   State Manager      │
│  (internal/state)    │
└─────────────────────┘
```

### Request Flow

```
1. HTTP Request → main.go
2. Route Matching → mux.HandleFunc()
3. Handler Execution → handle*Flows(), handle*Nodes(), etc.
4. Engine Interaction → FlowEngine methods
5. Response → JSON or static files
```

### WebSocket Flow

```
1. Connection → wsHandler.ServeWebSocket()
2. Registration → hub.register()
3. Message Receive → hub.readPump()
4. Message Process → hub.handleMessage()
5. Broadcast → hub.broadcast()
6. Send to Clients → hub.writePump()
```

---

## Main Application (main.go)

### Command-Line Configuration

The application uses **Go flags** for configuration:

```go
type Config struct {
    Port        int    // HTTP server port (default: 8080)
    DataDir     string // Flow data directory (default: "data")
    PluginDir   string // Plugin directory (default: "plugins")
    WebUIDir    string // Web UI directory (default: "web/dist")
    MaxWorkers  int    // Worker pool size (default: 100)
    MaxMessages int    // Message buffer size (default: 1000)
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
```

**Usage:**
```bash
# Start with default settings
go run cmd/go-red/main.go

# Start on custom port
go run cmd/go-red/main.go -port 3000

# Start with custom data directory
go run cmd/go-red/main.go -data-dir ./my-data

# All flags
go run cmd/go-red/main.go -port 3000 -data-dir ./data -web-dir ./web/dist
```

### Application Lifecycle

```go
func main() {
    // 1. Parse configuration
    config := parseFlags()
    
    // 2. Initialize logging
    log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
    log.Println("Starting Go—RED...")
    
    // 3. Initialize node registry (auto-registers built-in nodes)
    nodeRegistry := registry.GetGlobalRegistry()
    
    // 4. Initialize state manager
    stateManager, err := state.NewFileStateManager(config.DataDir)
    if err != nil { log.Fatal(...) }
    
    // 5. Initialize flow engine
    flowEngine := engine.NewFlowEngine(engine.EngineConfig{...}, nodeRegistry)
    flowEngine.SetStateManager(stateManager)
    
    // 6. Load existing flows
    if err := flowEngine.LoadAllFlows(); err != nil { log.Printf("Warning: %v", err) }
    
    // 7. Start flow engine
    if err := flowEngine.Start(); err != nil { log.Fatal(...) }
    
    // 8. Initialize WebSocket hub
    wsHub := websocket.NewHub()
    wsHandler := websocket.NewWebSocketHandler(wsHub, flowEngine, nodeRegistry)
    go wsHub.Run()
    
    // 9. Setup HTTP server
    mux := http.NewServeMux()
    // ... register routes ...
    
    // 10. Start HTTP server
    server := &http.Server{Addr: ":" + strconv.Itoa(config.Port), Handler: mux}
    go server.ListenAndServe()
    
    // 11. Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // 12. Graceful shutdown
    log.Println("Shutting down...")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    flowEngine.Stop()
    server.Shutdown(ctx)
    
    // 13. Save all active flows
    activeFlows := flowEngine.GetAllFlows()
    for _, flow := range activeFlows {
        stateManager.SaveFlow(flow)
    }
    log.Println("Shutdown complete")
}
```

---

## REST API Endpoints

### Flows

| Method | Endpoint | Description | Handler |
|--------|----------|-------------|---------|
| GET | `/api/flows` | List all flows | `handleGetFlows` |
| POST | `/api/flows` | Create new flow | `handleCreateFlow` |
| GET | `/api/flows/{id}` | Get specific flow | `handleGetFlow` |
| PUT | `/api/flows/{id}` | Update flow | `handleUpdateFlow` |
| DELETE | `/api/flows/{id}` | Delete flow | `handleDeleteFlow` |
| POST | `/api/flows/{id}/deploy` | Deploy flow | `handleDeployFlow` |
| POST | `/api/flows/{id}/undeploy` | Undeploy flow | `handleUndeployFlow` |
| GET | `/api/flows/{id}/export` | Export flow as JSON | `handleExportFlow` |
| POST | `/api/flows/import` | Import flow from JSON | `handleImportFlow` |

### Nodes

| Method | Endpoint | Description | Handler |
|--------|----------|-------------|---------|
| GET | `/api/nodes` | List all node types | `handleGetNodes` |
| GET | `/api/nodes/{type}` | Get node metadata | `handleGetNode` |

### Messages

| Method | Endpoint | Description | Handler |
|--------|----------|-------------|---------|
| GET | `/api/messages` | Get message log | `handleGetMessages` |

### WebSocket

| Method | Endpoint | Description | Handler |
|--------|----------|-------------|---------|
| GET | `/ws` | WebSocket connection | `wsHandler.ServeWebSocket` |

### Static Files

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Web UI (index.html) | `http.FileServer` |
| GET | `/assets/*` | Static assets | `http.FileServer` |

---

## API Handler Guidelines

### Handler Structure

All API handlers follow a similar pattern:

```go
func handleGetFlows(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    // 1. Get data
    flows := e.GetAllFlows()
    
    // 2. Transform data for API response
    type flowResponse struct {
        ID          string    `json:"id"`
        Name        string    `json:"name"`
        Description string    `json:"description"`
        Status      engine.FlowStatus `json:"status"`
        CreatedAt   time.Time `json:"createdAt"`
        UpdatedAt   time.Time `json:"updatedAt"`
    }
    response := make([]flowResponse, len(flows))
    for i, flow := range flows {
        response[i] = flowResponse{
            ID: flow.ID, Name: flow.Name, Description: flow.Description,
            Status: flow.Status, CreatedAt: flow.CreatedAt, UpdatedAt: flow.UpdatedAt,
        }
    }
    
    // 3. Set headers
    w.Header().Set("Content-Type", "application/json")
    
    // 4. Encode response
    json.NewEncoder(w).Encode(response)
}
```

### Error Handling in Handlers

```go
func handleGetFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    flowID := r.PathValue("id")
    
    flow, err := e.GetFlow(flowID)
    if err != nil {
        // Return appropriate HTTP status code
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    
    // Success
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(flow)
}
```

**Error Response Format:**
```json
{
  "error": "Flow not found",
  "flowId": "non-existent-id"
}
```

### Request Validation

```go
func handleCreateFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    // 1. Validate method
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // 2. Parse request body
    var request struct {
        ID          string `json:"id"`
        Name        string `json:"name"`
        Description string `json:"description"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // 3. Validate required fields
    if request.Name == "" {
        http.Error(w, "Name is required", http.StatusBadRequest)
        return
    }
    
    // 4. Process request
    flow, err := e.CreateFlow(request.ID, request.Name)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // 5. Set created flow's description
    if request.Description != "" {
        flow.Description = request.Description
    }
    
    // 6. Return success response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(flow)
}
```

### Frontend-Backend Conversion

The backend uses Go types, while the frontend expects specific JSON formats. Use conversion functions:

```go
// convertFlowToFrontendAPI converts Go flow to frontend-compatible format
func convertFlowToFrontendAPI(flow *engine.Flow) map[string]interface{} {
    // Convert nodes
    nodesMap := make(map[string]interface{})
    for id, node := range flow.Nodes {
        nodeMap := map[string]interface{}{
            "id":       node.ID,
            "type":     node.Type,
            "name":     node.Name,
            "position": map[string]float64{"x": node.X, "y": node.Y},
            "config":   node.Config,
            "status":   map[string]interface{}{...},
            "disabled": node.Disabled,
        }
        nodesMap[id] = nodeMap
    }
    
    // Convert connections
    connectionsList := make([]interface{}, len(flow.Connections))
    for i, conn := range flow.Connections {
        connMap := map[string]interface{}{
            "id":          conn.ID,
            "sourceNode":  conn.SourceNode,
            "sourcePort":  conn.SourcePort,
            "targetNode":  conn.TargetNode,
            "targetPort":  conn.TargetPort,
        }
        connectionsList[i] = connMap
    }
    
    return map[string]interface{}{
        "id":          flow.ID,
        "name":        flow.Name,
        "description": flow.Description,
        "nodes":       nodesMap,
        "connections": connectionsList,
        "status":      convertFlowStatusAPI(flow.Status),
        "config":      convertFlowConfigAPI(flow.Config),
        "createdAt":   flow.CreatedAt.Format(time.RFC3339),
        "updatedAt":   flow.UpdatedAt.Format(time.RFC3339),
        "version":     flow.Version,
    }
}

// convertFlowStatusAPI converts Go flow status to frontend status
func convertFlowStatusAPI(status engine.FlowStatus) string {
    switch status {
    case engine.FlowStatusInactive:
        return "draft"
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
```

---

## WebSocket Implementation

### WebSocket Hub (hub.go)

The hub manages all WebSocket connections and broadcasts messages:

```go
type Hub struct {
    // Registered connections
    clients map[*Client]bool
    
    // Inbound messages from clients
    broadcast chan []byte
    
    // Register requests from clients
    register chan *Client
    
    // Unregister requests from clients
    unregister chan *Client
    
    // Flow engine for processing
    engine *engine.FlowEngine
    
    // Node registry
    registry *registry.NodeRegistry
}

type Client struct {
    hub  *Hub
    conn *websocket.Conn
    send chan []byte
}
```

### Message Flow

```
Client Connection
    ↓
Client Registration (hub.register)
    ↓
Client Read Loop (client.readPump)
    ↓
Message Received
    ↓
hub.handleMessage()
    ↓
Process Message (interact with FlowEngine)
    ↓
Broadcast Response (hub.broadcast)
    ↓
Client Write Loop (client.writePump)
    ↓
All Clients Receive Message
```

### WebSocket Message Types

All messages are JSON with a `type` field:

```json
{
  "type": "message-type",
  "payload": { ... }
}
```

**Message Types:**

| Type | Direction | Description | Payload |
|------|-----------|-------------|---------|
| `flow:list` | Frontend → Backend | Request flow list | - |
| `flow:list` | Backend → Frontend | Flow list response | `{flows: [...]}` |
| `flow:get` | Frontend → Backend | Request specific flow | `{flowId: string}` |
| `flow:get` | Backend → Frontend | Flow response | `{flow: {...}}` |
| `flow:create` | Frontend → Backend | Create new flow | `{name: string, description?: string}` |
| `flow:create` | Backend → Frontend | Created flow | `{flow: {...}}` |
| `flow:update` | Frontend → Backend | Update flow | `{flowId: string, flow: {...}}` |
| `flow:delete` | Frontend → Backend | Delete flow | `{flowId: string}` |
| `flow:deploy` | Frontend → Backend | Deploy flow | `{flowId: string}` |
| `flow:undeploy` | Frontend → Backend | Undeploy flow | `{flowId: string}` |
| `flow:status` | Backend → Frontend | Flow status change | `{flowId: string, status: string}` |
| `node:list` | Frontend → Backend | Request node types | - |
| `node:list` | Backend → Frontend | Node types list | `{nodes: [...]}` |
| `message:log` | Backend → Frontend | New message in log | `{flowId: string, message: {...}}` |
| `error` | Backend → Frontend | Error notification | `{error: string, details?: {...}}` |

---

## Configuration Management

### Environment Variables

Consider supporting environment variables for production:

```go
// In main.go or config loading
func loadConfigFromEnv() Config {
    config := Config{}
    
    if port := os.Getenv("GO_RED_PORT"); port != "" {
        if p, err := strconv.Atoi(port); err == nil {
            config.Port = p
        }
    }
    
    if dataDir := os.Getenv("GO_RED_DATA_DIR"); dataDir != "" {
        config.DataDir = dataDir
    }
    
    return config
}
```

**Supported Environment Variables:**
- `GO_RED_PORT` - HTTP server port
- `GO_RED_DATA_DIR` - Flow data directory
- `GO_RED_PLUGIN_DIR` - Plugin directory
- `GO_RED_WEB_DIR` - Web UI directory
- `GO_RED_MAX_WORKERS` - Maximum worker goroutines
- `GO_RED_MAX_MESSAGES` - Message buffer size

### Configuration File (Future)

Consider adding YAML/JSON configuration file support:

```yaml
# config.yaml
server:
  port: 8080
  host: 0.0.0.0

directories:
  data: ./data
  plugins: ./plugins
  web: ./web/dist

engine:
  max_workers: 100
  max_messages: 1000
  default_timeout: 30s

logging:
  level: info
  format: text
```

---

## Testing Main Application

### Integration Tests

Test the complete application stack:

```go
func TestMain_Integration(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    
    // Start server in goroutine
    cmd := exec.Command("go", "run", "cmd/go-red/main.go", 
        "-port", "0", // Random port
        "-data-dir", tmpDir,
        "-web-dir", "web/dist",
    )
    
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    
    require.NoError(t, cmd.Start())
    defer cmd.Process.Kill()
    
    // Wait for server to start
    time.Sleep(2 * time.Second)
    
    // Find the port
    // (This would require parsing the output or using a fixed test port)
    port := findFreePort()
    baseURL := fmt.Sprintf("http://localhost:%d", port)
    
    // Test API endpoints
    client := &http.Client{}
    
    // Test GET /api/flows
    resp, err := client.Get(baseURL + "/api/flows")
    require.NoError(t, err)
    defer resp.Body.Close()
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    // Test GET /api/nodes
    resp, err = client.Get(baseURL + "/api/nodes")
    require.NoError(t, err)
    defer resp.Body.Close()
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    // More tests...
}
```

### Handler Unit Tests

Test individual handlers in isolation:

```go
func TestHandleGetFlows(t *testing.T) {
    // Setup
    registry := registry.GetGlobalRegistry()
    engine := NewFlowEngine(DefaultEngineConfig(), registry)
    
    // Create test flow
    engine.CreateFlow("test-1", "Test 1")
    engine.CreateFlow("test-2", "Test 2")
    
    // Create request
    req := httptest.NewRequest("GET", "/api/flows", nil)
    w := httptest.NewRecorder()
    
    // Call handler
    handleGetFlows(w, req, engine)
    
    // Verify response
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
    
    var response []flowResponse
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Len(t, response, 2)
}

func TestHandleCreateFlow(t *testing.T) {
    // Setup
    registry := registry.GetGlobalRegistry()
    engine := NewFlowEngine(DefaultEngineConfig(), registry)
    
    // Create request body
    body := map[string]interface{}{
        "name": "New Flow",
        "description": "A new flow",
    }
    bodyJSON, _ := json.Marshal(body)
    
    req := httptest.NewRequest("POST", "/api/flows", bytes.NewReader(bodyJSON))
    w := httptest.NewRecorder()
    
    // Call handler
    handleCreateFlow(w, req, engine)
    
    // Verify response
    assert.Equal(t, http.StatusCreated, w.Code)
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    assert.Equal(t, "New Flow", response["name"])
    assert.NotEmpty(t, response["id"])
}
```

### WebSocket Tests

Test WebSocket communication:

```go
func TestWebSocket_Connection(t *testing.T) {
    // Setup
    registry := registry.GetGlobalRegistry()
    engine := NewFlowEngine(DefaultEngineConfig(), registry)
    engine.Start()
    defer engine.Stop()
    
    hub := websocket.NewHub()
    handler := websocket.NewWebSocketHandler(hub, engine, registry)
    go hub.Run()
    
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        handler.ServeWebSocket(w, r)
    }))
    defer server.Close()
    
    // Connect WebSocket client
    u := url.URL{Scheme: "ws", Host: server.Listener.Addr().String(), Path: "/"}
    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    require.NoError(t, err)
    defer conn.Close()
    
    // Send message
    message := map[string]interface{}{
        "type": "node:list",
    }
    jsonData, _ := json.Marshal(message)
    err = conn.WriteMessage(websocket.TextMessage, jsonData)
    require.NoError(t, err)
    
    // Read response
    _, responseData, err := conn.ReadMessage()
    require.NoError(t, err)
    
    var response map[string]interface{}
    json.Unmarshal(responseData, &response)
    assert.Equal(t, "node:list", response["type"])
    assert.Contains(t, response, "payload")
}
```

---

## Performance Considerations

### HTTP Server Optimization

1. **Connection Pooling**: Reuse HTTP connections
2. **Compression**: Enable gzip compression for responses
3. **Keep-Alive**: Enable HTTP keep-alive
4. **Timeouts**: Set appropriate timeouts

```go
// Optimized server configuration
server := &http.Server{
    Addr:         ":8080",
    Handler:      mux,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  30 * time.Second,
    MaxHeaderBytes: 1 << 20, // 1 MB
}
```

### WebSocket Optimization

1. **Message Batching**: Batch multiple messages into one
2. **Compression**: Enable WebSocket compression
3. **Buffer Sizes**: Configure appropriate buffer sizes
4. **Ping/Pong**: Configure keep-alive

```go
// Optimized WebSocket upgrader
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        return true // Or implement proper origin checking
    },
    EnableCompression: true,
}

// In client struct
const (
    writeWait      = 10 * time.Second
    pongWait       = 60 * time.Second
    pingPeriod     = (pongWait * 9) / 10
    maxMessageSize = 512
)
```

---

## Security Considerations

### WebSocket Security

1. **Origin Checking**: Verify WebSocket connection origins
2. **Authentication**: Authenticate WebSocket connections
3. **Rate Limiting**: Prevent abuse
4. **Message Validation**: Validate all incoming messages

```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        // Allow connections from specific origins
        origin := r.Header.Get("Origin")
        allowedOrigins := []string{"http://localhost:8080", "http://localhost:3000"}
        for _, allowed := range allowedOrigins {
            if origin == allowed {
                return true
            }
        }
        return false
    },
    // Or allow all for development
    // CheckOrigin: func(r *http.Request) bool { return true },
}
```

### CORS Configuration

Add CORS headers for development:

```go
func enableCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

// Usage
mux.Handle("/api/", enableCORS(http.StripPrefix("/api/", apiHandler)))
```

### Input Sanitization

Always sanitize user input:

```go
func sanitizeString(s string) string {
    // Remove null bytes
    s = strings.ReplaceAll(s, "\x00", "")
    // Limit length
    if len(s) > 10000 {
        s = s[:10000]
    }
    return s
}

func handleCreateFlow(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    var request struct {
        Name string `json:"name"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Sanitize input
    request.Name = sanitizeString(request.Name)
    
    // Validate
    if request.Name == "" {
        http.Error(w, "Name is required", http.StatusBadRequest)
        return
    }
    
    // ... rest of handler
}
```

---

## Debugging

### Common Issues

#### Server Won't Start
**Symptoms**: Application exits immediately or hangs

**Checks:**
1. Is the port already in use?
2. Are there permission issues with data directory?
3. Are required dependencies installed?
4. Are there panic messages in stderr?

**Solution:**
```bash
# Check port usage
lsof -i :8080

# Run with verbose logging
go run cmd/go-red/main.go -port 9000

# Check dependencies
go mod tidy
go mod download
```

#### API Not Responding
**Symptoms**: API endpoints return errors or timeouts

**Checks:**
1. Is the server running?
2. Are there errors in the server logs?
3. Is the database/state manager connected?
4. Are there panics in the goroutines?

**Solution:**
```go
// Add debug logging to handlers
func handleGetFlows(w http.ResponseWriter, r *http.Request, e *engine.FlowEngine) {
    log.Println("GET /api/flows called")
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic in handleGetFlows: %v", r)
        }
    }()
    // ... rest of handler
}
```

#### WebSocket Connection Fails
**Symptoms**: WebSocket connection cannot be established

**Checks:**
1. Is the WebSocket endpoint correct?
2. Are there CORS issues?
3. Is the origin allowed?
4. Are there errors in browser console?

**Solution:**
```javascript
// Check browser console for WebSocket errors
const socket = new WebSocket("ws://localhost:8080/ws");
socket.onerror = (e) => console.error("WebSocket error:", e);
socket.onclose = (e) => console.log("WebSocket closed:", e.code, e.reason);
```

---

## Future Enhancements

### Planned Features
- **Configuration file support** (YAML/JSON)
- **Environment variable support** for all config options
- **HTTPS support** with automatic certificate management
- **Authentication** (JWT, OAuth, basic auth)
- **Rate limiting** for API endpoints
- **API documentation** (Swagger/OpenAPI)
- **Metrics endpoint** (Prometheus)
- **Health check endpoint**
- **Graceful reload** (SIGHUP to reload config)

### Architecture Improvements
- **Modular middleware** system for HTTP handlers
- **Plugin API** for extending the server
- **Multi-tenancy** support
- **Cluster mode** for horizontal scaling
- **API versioning** for backward compatibility

---

## Checklist for Main Application Changes

Before committing changes to `cmd/go-red/`:

- [ ] All existing tests pass
- [ ] HTTP endpoints work correctly
- [ ] WebSocket communication works
- [ ] Error handling is consistent
- [ ] No panics in normal operation
- [ ] Graceful shutdown works
- [ ] Configuration changes are documented
- [ ] Security considerations are addressed
- [ ] Performance hasn't degraded
- [ ] Frontend integration still works

---

*Last updated: 2026-06-21*
*Overrides: None (extends root AGENTS.md)*
