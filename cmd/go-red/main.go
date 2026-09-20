// Package main is the entry point for the Go—RED application.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/GrimbiXcode/Go-RED/cmd/go-red/websocket"
	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/GrimbiXcode/Go-RED/internal/nodered"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/state"

	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/batch"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/catch"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/change"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/comment"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/complete"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/csvnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/delay"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/execnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/file"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/filein"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/htmlnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httpproxy"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httprequest"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httpresponse"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/join"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/jsonnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/junction"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/linkin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/linkout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttbroker"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/rangenode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/rbe"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/sortnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/split"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/status"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/switchnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tcpin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tcpout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tcprequest"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/template"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tlsconfig"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/trigger"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/udpin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/udpout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/watch"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketclient"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketlistener"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/xmlnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/yamlnode"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

// maxBodyBytes caps the size of any REST request body.
const maxBodyBytes = 10 << 20

func main() {
	config, showVersion, err := loadConfig(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "go-red:", err)
		os.Exit(2)
	}
	if showVersion {
		fmt.Println("go-red", version)
		os.Exit(0)
	}
	setupLogging(config.LogLevel)
	slog.Info("starting Go-RED", "version", version, "port", config.Port, "dataDir", config.DataDir, "webDir", config.WebUIDir,
		"auth", config.AuthToken != "", "allowedOrigins", config.AllowedOrigins, "rateLimit", config.RateLimit)

	nodeRegistry := registry.GetGlobalRegistry()
	slog.Info("node registry initialized", "nodeTypes", len(nodeRegistry.GetAllNodes()))

	stateManager, err := state.NewFileStateManagerWithOptions(config.DataDir, state.Options{BackupKeep: config.BackupKeep, BackupMinInterval: config.BackupInterval})
	if err != nil {
		slog.Error("failed to create state manager", "err", err)
		os.Exit(1)
	}

	flowEngine := engine.NewFlowEngine(engine.EngineConfig{
		MessageBufferSize:  config.MaxMessages,
		MaxInflightPerFlow: config.MaxInflight,
		MessageLogSize:     config.MessageLog,
		DefaultTimeout:     30 * time.Second,
		MaxRetries:         3,
		RetryBackoff:       1 * time.Second,
	}, nodeRegistry)
	flowEngine.SetStateManager(stateManager)

	if err := flowEngine.Start(); err != nil {
		slog.Error("failed to start flow engine", "err", err)
		os.Exit(1)
	}
	if err := flowEngine.LoadAllFlows(); err != nil {
		slog.Warn("failed to load existing flows", "err", err)
	}

	wsHub := websocket.NewHub()
	wsHub.CheckOrigin = func(r *http.Request) bool { return originAllowed(r, config.AllowedOrigins) }
	wsHandler := websocket.NewWebSocketHandler(wsHub, flowEngine, nodeRegistry)
	go wsHub.Run()

	server := &http.Server{
		Addr: ":" + strconv.Itoa(config.Port),
		Handler: newRouter(flowEngine, nodeRegistry, wsHandler, routerOptions{
			WebDir:             config.WebUIDir,
			AuthToken:          config.AuthToken,
			AllowedOrigins:     config.AllowedOrigins,
			RateLimitPerMinute: config.RateLimit,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("server listening", "addr", server.Addr, "websocket", "/ws")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Warn("http server shutdown", "err", err)
	}
	flowEngine.Stop()
	slog.Info("shutdown complete")
}

// setupLogging installs a leveled slog handler as the process-wide default.
// The standard log package is routed through it too, so node packages that
// still use log.Printf end up in the same stream.
func setupLogging(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}

// Notifier is how the REST handlers tell connected WebSocket clients about
// flow changes. The REST API is the only write path; the WebSocket only
// carries the resulting events.
type Notifier interface {
	FlowChanged(flowID string)
	FlowDeleted(flowID string)
}

type noopNotifier struct{}

func (noopNotifier) FlowChanged(string) {}
func (noopNotifier) FlowDeleted(string) {}

// server bundles the dependencies the REST handlers need.
type server struct {
	engine    *engine.FlowEngine
	registry  *registry.NodeRegistry
	notify    Notifier
	webDir    string
	startedAt time.Time
}

// routerOptions is the part of the configuration the HTTP layer needs.
type routerOptions struct {
	WebDir             string
	AuthToken          string
	AllowedOrigins     []string
	RateLimitPerMinute int
}

// newRouter wires every REST route, the WebSocket endpoint (when ws is not
// nil), the operational endpoints and the static WebUI with SPA fallback
// into one handler, guarded by the origin policy and the optional token.
func newRouter(e *engine.FlowEngine, reg *registry.NodeRegistry, ws *websocket.WebSocketHandler, opts routerOptions) http.Handler {
	var notify Notifier = noopNotifier{}
	if ws != nil {
		notify = ws
	}
	s := &server{engine: e, registry: reg, notify: notify, webDir: opts.WebDir, startedAt: time.Now()}
	limiter := newRateLimiter(opts.RateLimitPerMinute)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/flows", s.handleGetFlows)
	mux.HandleFunc("POST /api/flows", s.handleCreateFlow)
	mux.HandleFunc("POST /api/flows/import", limiter.limit(s.handleImportFlow))
	mux.HandleFunc("GET /api/flows/{id}", s.handleGetFlow)
	mux.HandleFunc("PUT /api/flows/{id}", s.handleUpdateFlow)
	mux.HandleFunc("DELETE /api/flows/{id}", s.handleDeleteFlow)
	mux.HandleFunc("POST /api/flows/{id}/deploy", limiter.limit(s.handleDeployFlow))
	mux.HandleFunc("POST /api/flows/{id}/undeploy", s.handleUndeployFlow)
	mux.HandleFunc("GET /api/flows/{id}/export", s.handleExportFlow)
	mux.HandleFunc("GET /api/nodes", s.handleGetNodes)
	mux.HandleFunc("GET /api/nodes/{type}", s.handleGetNode)
	mux.HandleFunc("GET /api/messages", s.handleGetMessages)
	if ws != nil {
		mux.HandleFunc("GET /ws", ws.ServeWebSocket)
	}
	mux.Handle("/", spaHandler(opts.WebDir))
	return corsMiddleware(opts.AllowedOrigins, requireAuth(opts.AuthToken, mux))
}

// spaHandler serves the built WebUI. Existing files are served as-is;
// any other extension-less path falls back to index.html so client-side
// routes survive a reload. Missing files with an extension are a plain 404.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		full := filepath.Join(dir, filepath.FromSlash(clean))

		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}

		f, err := os.Open(index)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// ServeContent (not ServeFile) so the response does not depend on
		// the request path at all: no "/index.html" redirect and no
		// rejection of paths containing "..".
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", info.ModTime(), f)
	})
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("failed to write response", "err", err)
	}
}

// writeError sends a JSON error body with the given status.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, dto.ErrorResponse{Error: message})
}

// statusForError maps engine errors to HTTP status codes.
func statusForError(err error) int {
	switch {
	case errors.Is(err, engine.ErrFlowNotFound):
		return http.StatusNotFound
	case errors.Is(err, engine.ErrFlowExists):
		return http.StatusConflict
	case errors.Is(err, engine.ErrFlowNotDeployed):
		return http.StatusConflict
	case errors.Is(err, engine.ErrInvalidFlowID):
		return http.StatusBadRequest
	case errors.Is(err, engine.ErrInvalidFlow), errors.Is(err, engine.ErrNodeInit):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// writeEngineError turns an engine error into a response. Errors the engine
// classifies (see engine's sentinel errors) are user-facing and returned as
// they are; anything else is logged in full and answered generically so
// internal details never reach a client.
func writeEngineError(w http.ResponseWriter, err error) {
	status := statusForError(err)
	if status == http.StatusInternalServerError {
		slog.Error("request failed", "err", err)
		writeError(w, status, "internal error")
		return
	}
	writeError(w, status, err.Error())
}

// decodeJSON reads a size-capped JSON request body into v.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	return json.NewDecoder(r.Body).Decode(v)
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	flows := s.engine.GetAllFlows()
	deployed := 0
	for _, flow := range flows {
		if s.engine.IsDeployed(flow.ID) {
			deployed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"version":       version,
		"flows":         len(flows),
		"deployed":      deployed,
		"uptimeSeconds": int(time.Since(s.startedAt).Seconds()),
	})
}

func (s *server) handleGetFlows(w http.ResponseWriter, r *http.Request) {
	flows := s.engine.GetAllFlows()
	response := make([]dto.FlowSummary, len(flows))
	for i, flow := range flows {
		response[i] = dto.ToWireSummary(flow)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleCreateFlow(w http.ResponseWriter, r *http.Request) {
	var request dto.FlowCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	request.Name = strings.TrimSpace(sanitizeString(request.Name))
	request.Description = sanitizeString(request.Description)
	if request.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	flow, err := s.engine.CreateFlow(request.ID, request.Name, request.Description)
	if err != nil {
		writeEngineError(w, err)
		return
	}
	s.notify.FlowChanged(flow.ID)
	writeJSON(w, http.StatusCreated, dto.ToWire(flow))
}

func (s *server) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	flow, err := s.engine.GetFlow(r.PathValue("id"))
	if err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.ToWire(flow))
}

func (s *server) handleUpdateFlow(w http.ResponseWriter, r *http.Request) {
	var request dto.FlowUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
		node.Description = sanitizeString(node.Description)
		request.Nodes[id] = node
	}

	flow, err := s.engine.UpdateFlow(r.PathValue("id"), func(f *engine.Flow) error {
		request.ApplyTo(f)
		return nil
	})
	if err != nil {
		writeEngineError(w, err)
		return
	}
	s.notify.FlowChanged(flow.ID)
	writeJSON(w, http.StatusOK, dto.ToWire(flow))
}

func (s *server) handleDeleteFlow(w http.ResponseWriter, r *http.Request) {
	flowID := r.PathValue("id")
	if err := s.engine.DeleteFlow(flowID); err != nil {
		writeEngineError(w, err)
		return
	}
	s.notify.FlowDeleted(flowID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted", "flowId": flowID})
}

func (s *server) handleDeployFlow(w http.ResponseWriter, r *http.Request) {
	flowID := r.PathValue("id")
	// Deploy and undeploy results reach clients as engine runtime events
	// (flow:status); no REST-side notification needed.
	if err := s.engine.DeployFlow(flowID); err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.deployResponse(flowID, dto.FlowStatusRunning))
}

func (s *server) handleUndeployFlow(w http.ResponseWriter, r *http.Request) {
	flowID := r.PathValue("id")
	if err := s.engine.Undeploy(flowID); err != nil {
		writeEngineError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.deployResponse(flowID, dto.FlowStatusDraft))
}

// deployResponse builds the response of deploy/undeploy from the flow's
// current timestamps.
func (s *server) deployResponse(flowID string, status dto.FlowStatus) dto.DeployResponse {
	resp := dto.DeployResponse{FlowID: flowID, Status: status}
	if flow, err := s.engine.GetFlow(flowID); err == nil {
		summary := dto.ToWireSummary(flow)
		resp.UpdatedAt = summary.UpdatedAt
		resp.DeployedAt = summary.DeployedAt
	}
	return resp
}

func (s *server) handleGetNodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.registry.GetAllNodes())
}

func (s *server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	metadata, err := s.registry.GetMetadata(r.PathValue("type"))
	if err != nil {
		writeError(w, http.StatusNotFound, "node type not found")
		return
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (s *server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	flowID := r.URL.Query().Get("flowId")

	var messages []engine.Message
	if flowID != "" {
		messages = s.engine.GetMessageLogForFlow(flowID)
	} else {
		messages = s.engine.GetMessageLog()
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && len(messages) > limit {
			messages = messages[len(messages)-limit:]
		}
	}

	writeJSON(w, http.StatusOK, dto.MessagesToWire(messages))
}

func (s *server) handleExportFlow(w http.ResponseWriter, r *http.Request) {
	flowID := r.PathValue("id")
	flow, err := s.engine.GetFlow(flowID)
	if err != nil {
		writeEngineError(w, err)
		return
	}

	if r.URL.Query().Get("format") == "node-red" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=flow-%s.node-red.json", flowID))
		writeJSON(w, http.StatusOK, nodered.Export(flow))
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=flow-%s.json", flowID))
	writeJSON(w, http.StatusOK, dto.ToWire(flow))
}

func (s *server) handleImportFlow(w http.ResponseWriter, r *http.Request) {
	// The body is either a canonical wire Flow (the shape produced by
	// GET /api/flows/{id}/export) or a Node-RED export: a JSON array of
	// nodes with "wires", which may hold several tabs.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if nodered.IsExport(body) {
		s.importNodeRED(w, body)
		return
	}

	var importData dto.Flow
	if err := json.Unmarshal(body, &importData); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	importData.Name = strings.TrimSpace(sanitizeString(importData.Name))
	importData.Description = sanitizeString(importData.Description)
	if importData.Name == "" {
		writeError(w, http.StatusBadRequest, "flow name is required")
		return
	}

	// Imported flows always get a fresh ID so they never collide with an
	// existing flow; the original ID is echoed back for reference.
	originalID := importData.ID
	flow := engine.NewFlow(uuid.New().String(), importData.Name)
	dto.PopulateFromWire(flow, importData)
	s.sanitizeNodes(flow)

	if err := s.engine.AddFlow(flow); err != nil {
		writeEngineError(w, err)
		return
	}
	s.notify.FlowChanged(flow.ID)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":     "imported",
		"format":     "go-red",
		"flowId":     flow.ID,
		"originalId": originalID,
		"name":       flow.Name,
		"flows":      []map[string]interface{}{{"flowId": flow.ID, "originalId": originalID, "name": flow.Name}},
		"warnings":   []string{},
		"message":    "Flow imported successfully",
	})
}

// importNodeRED imports every tab of a Node-RED export as its own flow.
func (s *server) importNodeRED(w http.ResponseWriter, body []byte) {
	result, err := nodered.Import(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(result.Flows) == 0 {
		writeError(w, http.StatusBadRequest, "the export contains no flows")
		return
	}

	imported := make([]map[string]interface{}, 0, len(result.Flows))
	for _, flow := range result.Flows {
		originalID := flow.ID
		flow.ID = uuid.New().String()
		flow.Name = strings.TrimSpace(sanitizeString(flow.Name))
		if flow.Name == "" {
			flow.Name = "Imported flow"
		}
		flow.Description = sanitizeString(flow.Description)
		s.sanitizeNodes(flow)
		if err := s.engine.AddFlow(flow); err != nil {
			writeEngineError(w, err)
			return
		}
		s.notify.FlowChanged(flow.ID)
		imported = append(imported, map[string]interface{}{"flowId": flow.ID, "originalId": originalID, "name": flow.Name})
	}
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":     "imported",
		"format":     "node-red",
		"flowId":     imported[0]["flowId"],
		"originalId": imported[0]["originalId"],
		"name":       imported[0]["name"],
		"flows":      imported,
		"warnings":   warnings,
		"message":    fmt.Sprintf("%d flow(s) imported from Node-RED", len(imported)),
	})
}

// sanitizeNodes strips control characters from user-visible node strings.
func (s *server) sanitizeNodes(flow *engine.Flow) {
	for nodeID, node := range flow.Nodes {
		node.Type = sanitizeString(node.Type)
		node.Name = sanitizeString(node.Name)
		node.Description = sanitizeString(node.Description)
		flow.Nodes[nodeID] = node
	}
}
