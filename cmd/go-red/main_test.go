package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/GrimbiXcode/Go-RED/internal/engine"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/webui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryStateManager is an in-memory engine.StateManager for handler tests.
type memoryStateManager struct {
	flows map[string]*engine.Flow
}

func newMemoryStateManager() *memoryStateManager {
	return &memoryStateManager{flows: make(map[string]*engine.Flow)}
}

func (m *memoryStateManager) SaveFlow(flow *engine.Flow) error {
	m.flows[flow.ID] = flow.Clone()
	return nil
}

func (m *memoryStateManager) LoadFlow(flowID string) (*engine.Flow, error) {
	flow, ok := m.flows[flowID]
	if !ok {
		return nil, engine.ErrFlowNotFound
	}
	return flow.Clone(), nil
}

func (m *memoryStateManager) LoadAllFlows() ([]*engine.Flow, error) {
	flows := make([]*engine.Flow, 0, len(m.flows))
	for _, flow := range m.flows {
		flows = append(flows, flow.Clone())
	}
	return flows, nil
}

func (m *memoryStateManager) DeleteFlow(flowID string) error {
	if _, ok := m.flows[flowID]; !ok {
		return engine.ErrFlowNotFound
	}
	delete(m.flows, flowID)
	return nil
}

var _ engine.StateManager = (*memoryStateManager)(nil)

// newTestServer returns a started engine and the real router (without the
// WebSocket endpoint) so tests exercise exactly the routes main wires up.
func newTestServer(t *testing.T) (*engine.FlowEngine, http.Handler) {
	t.Helper()
	return newTestServerWith(t, routerOptions{})
}

// newTestServerWith is newTestServer with router options (auth token,
// origins, rate limit); an empty WebDir becomes a temp dir.
func newTestServerWith(t *testing.T, opts routerOptions) (*engine.FlowEngine, http.Handler) {
	t.Helper()
	reg := registry.GetGlobalRegistry()
	e := engine.NewFlowEngine(engine.EngineConfig{
		MessageBufferSize: 100,
		DefaultTimeout:    5 * time.Second,
	}, reg)
	e.SetStateManager(newMemoryStateManager())
	require.NoError(t, e.Start())
	t.Cleanup(func() { e.Stop() })
	if opts.WebDir == "" {
		opts.WebDir = t.TempDir()
	}
	return e, newRouter(e, reg, nil, opts)
}

func do(t *testing.T, h http.Handler, method, target string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), v), "body: %s", w.Body.String())
}

func TestCreateFlow(t *testing.T) {
	_, h := newTestServer(t)

	w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "demo", Name: "Demo", Description: "hello"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created dto.Flow
	decodeBody(t, w, &created)
	assert.Equal(t, "demo", created.ID)
	assert.Equal(t, "Demo", created.Name)
	assert.Equal(t, "hello", created.Description)
	assert.Equal(t, dto.FlowStatusDraft, created.Status)

	w = do(t, h, http.MethodGet, "/api/flows", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var list []dto.FlowSummary
	decodeBody(t, w, &list)
	require.Len(t, list, 1)
	assert.Equal(t, "demo", list[0].ID)

	t.Run("duplicate ID is a conflict", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "demo", Name: "Again"})
		assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	})

	t.Run("missing name is rejected", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{Name: "   "})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid ID is rejected before it reaches storage", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "../../etc/passwd", Name: "Evil"})
		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		var resp dto.ErrorResponse
		decodeBody(t, w, &resp)
		assert.Contains(t, resp.Error, "invalid flow ID")
	})

	t.Run("generated IDs are valid", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{Name: "No ID"})
		require.Equal(t, http.StatusCreated, w.Code)
		var created dto.Flow
		decodeBody(t, w, &created)
		assert.NoError(t, engine.ValidateFlowID(created.ID))
	})
}

func TestDeployLifecycle(t *testing.T) {
	e, h := newTestServer(t)

	w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "lc", Name: "Lifecycle"})
	require.Equal(t, http.StatusCreated, w.Code)

	// A flow created through the API can be edited ...
	w = do(t, h, http.MethodPut, "/api/flows/lc", dto.FlowUpdateRequest{
		Nodes: map[string]dto.Node{
			"n1": {ID: "n1", Type: "debug", Position: dto.Position{X: 10, Y: 20}, Config: map[string]interface{}{}},
		},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var updated dto.Flow
	decodeBody(t, w, &updated)
	assert.Equal(t, 10.0, updated.Nodes["n1"].Position.X)

	// ... and then actually deployed (this used to be a silent no-op).
	w = do(t, h, http.MethodPost, "/api/flows/lc/deploy", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var deployResp dto.DeployResponse
	decodeBody(t, w, &deployResp)
	assert.Equal(t, dto.FlowStatusRunning, deployResp.Status)
	assert.True(t, e.IsDeployed("lc"))

	w = do(t, h, http.MethodGet, "/api/flows/lc", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var fetched dto.Flow
	decodeBody(t, w, &fetched)
	assert.Equal(t, dto.FlowStatusRunning, fetched.Status)

	// Editing while running is allowed and does not disturb the runtime.
	w = do(t, h, http.MethodPut, "/api/flows/lc", dto.FlowUpdateRequest{
		Nodes: map[string]dto.Node{
			"n2": {ID: "n2", Type: "debug", Config: map[string]interface{}{}},
		},
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, e.IsDeployed("lc"))

	// Deploying again is a redeploy, not an error.
	w = do(t, h, http.MethodPost, "/api/flows/lc/deploy", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, e.IsDeployed("lc"))

	w = do(t, h, http.MethodPost, "/api/flows/lc/undeploy", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	decodeBody(t, w, &deployResp)
	assert.Equal(t, dto.FlowStatusDraft, deployResp.Status)
	assert.False(t, e.IsDeployed("lc"))

	// The flow is still listed after undeploy.
	w = do(t, h, http.MethodGet, "/api/flows", nil)
	var list []dto.FlowSummary
	decodeBody(t, w, &list)
	require.Len(t, list, 1)
	assert.Equal(t, dto.FlowStatusDraft, list[0].Status)

	w = do(t, h, http.MethodDelete, "/api/flows/lc", nil)
	require.Equal(t, http.StatusOK, w.Code)
	w = do(t, h, http.MethodGet, "/api/flows/lc", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
	w = do(t, h, http.MethodDelete, "/api/flows/lc", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeployErrors(t *testing.T) {
	_, h := newTestServer(t)

	t.Run("unknown flow is 404", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows/nope/deploy", nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
		w = do(t, h, http.MethodPost, "/api/flows/nope/undeploy", nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid flow is 422 with a readable message", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "empty", Name: "Empty"})
		require.Equal(t, http.StatusCreated, w.Code)

		w = do(t, h, http.MethodPost, "/api/flows/empty/deploy", nil)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
		var resp dto.ErrorResponse
		decodeBody(t, w, &resp)
		assert.Contains(t, resp.Error, "at least one node")

		w = do(t, h, http.MethodGet, "/api/flows/empty", nil)
		var flow dto.Flow
		decodeBody(t, w, &flow)
		assert.Equal(t, dto.FlowStatusDraft, flow.Status, "a rejected deploy leaves the flow as it was")
	})

	t.Run("connection to a missing node is 422", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "badconn", Name: "Bad"})
		require.Equal(t, http.StatusCreated, w.Code)
		w = do(t, h, http.MethodPut, "/api/flows/badconn", dto.FlowUpdateRequest{
			Nodes:       map[string]dto.Node{"n1": {ID: "n1", Type: "debug", Config: map[string]interface{}{}}},
			Connections: []dto.Connection{{ID: "c1", SourceNode: "n1", TargetNode: "ghost"}},
		})
		require.Equal(t, http.StatusOK, w.Code)
		w = do(t, h, http.MethodPost, "/api/flows/badconn/deploy", nil)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
	})

	t.Run("unknown node type is 422", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "unknowntype", Name: "Unknown"})
		require.Equal(t, http.StatusCreated, w.Code)
		w = do(t, h, http.MethodPut, "/api/flows/unknowntype", dto.FlowUpdateRequest{
			Nodes: map[string]dto.Node{"n1": {ID: "n1", Type: "does-not-exist", Config: map[string]interface{}{}}},
		})
		require.Equal(t, http.StatusOK, w.Code)
		w = do(t, h, http.MethodPost, "/api/flows/unknowntype/deploy", nil)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())

		w = do(t, h, http.MethodGet, "/api/flows/unknowntype", nil)
		var flow dto.Flow
		decodeBody(t, w, &flow)
		assert.Equal(t, dto.FlowStatusError, flow.Status)
	})
}

func TestPathTraversalIDsNeverReachStorage(t *testing.T) {
	_, h := newTestServer(t)

	for _, target := range []string{
		"/api/flows/..%2F..%2Fetc%2Fpasswd",
		"/api/flows/..%2F..%2Fetc%2Fpasswd/deploy",
	} {
		w := do(t, h, http.MethodGet, target, nil)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusBadRequest || w.Code == http.StatusMethodNotAllowed, "%s -> %d", target, w.Code)
	}
	w := do(t, h, http.MethodDelete, "/api/flows/..%2F..%2Fetc%2Fpasswd", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestImportFlow(t *testing.T) {
	_, h := newTestServer(t)

	w := do(t, h, http.MethodPost, "/api/flows/import", dto.Flow{
		ID:   "original",
		Name: "Imported",
		Nodes: map[string]dto.Node{
			"n1": {ID: "n1", Type: "inject", Config: map[string]interface{}{}},
			"n2": {ID: "n2", Type: "debug", Config: map[string]interface{}{}},
		},
		Connections: []dto.Connection{{ID: "c1", SourceNode: "n1", TargetNode: "n2"}},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp map[string]interface{}
	decodeBody(t, w, &resp)
	assert.Equal(t, "original", resp["originalId"])
	newID, _ := resp["flowId"].(string)
	require.NotEmpty(t, newID)
	assert.NotEqual(t, "original", newID)

	// The imported flow is immediately visible and deployable.
	w = do(t, h, http.MethodGet, "/api/flows/"+newID, nil)
	require.Equal(t, http.StatusOK, w.Code)
	var flow dto.Flow
	decodeBody(t, w, &flow)
	assert.Len(t, flow.Nodes, 2)
	assert.Len(t, flow.Connections, 1)

	w = do(t, h, http.MethodPost, "/api/flows/"+newID+"/deploy", nil)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	t.Run("name is required", func(t *testing.T) {
		w := do(t, h, http.MethodPost, "/api/flows/import", dto.Flow{ID: "x"})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHealth(t *testing.T) {
	_, h := newTestServer(t)
	w := do(t, h, http.MethodGet, "/api/health", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	decodeBody(t, w, &resp)
	assert.Equal(t, "ok", resp["status"])
}

func TestSPAFallback(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log(1)"), 0o644))

	for name, fsys := range map[string]fs.FS{
		"directory": os.DirFS(dir),
		"in-memory": fstest.MapFS{
			"index.html":    {Data: []byte("<html>app</html>")},
			"assets/app.js": {Data: []byte("console.log(1)")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := spaHandler(fsys)
			get := func(target string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
				return w
			}

			assert.Equal(t, "<html>app</html>", get("/").Body.String())
			assert.Equal(t, "<html>app</html>", get("/flow/abc").Body.String(), "client-side routes fall back to index.html")
			assert.Equal(t, "console.log(1)", get("/assets/app.js").Body.String())
			assert.Equal(t, http.StatusNotFound, get("/assets/missing.js").Code, "missing assets are a real 404")
			assert.Equal(t, "<html>app</html>", get("/../../etc/passwd").Body.String(), "traversal attempts stay inside the web dir")
		})
	}
}

func TestEmbeddedEditorOrHint(t *testing.T) {
	e, _ := newTestServer(t)
	h := newRouter(e, registry.GetGlobalRegistry(), nil, routerOptions{})
	w := serve(h, request(http.MethodGet, "/flow/abc", nil))
	if _, built := webui.Dist(); built {
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "<div id=\"root\">")
	} else {
		assert.Equal(t, http.StatusServiceUnavailable, w.Code, "a binary without the editor says so")
		assert.Contains(t, w.Body.String(), "npm run build")
		assert.Equal(t, http.StatusNotFound, serve(h, request(http.MethodGet, "/assets/app.js", nil)).Code)
	}
	assert.Equal(t, http.StatusOK, serve(h, request(http.MethodGet, "/api/health", nil)).Code, "the API works either way")
}

func TestFlowValidation(t *testing.T) {
	t.Run("should pass validation with valid nodes and connections", func(t *testing.T) {
		flow := engine.NewFlow("valid-flow", "Valid Flow")
		flow.Nodes["node-1"] = &engine.Node{ID: "node-1", Type: "debug"}
		flow.Nodes["node-2"] = &engine.Node{ID: "node-2", Type: "debug"}
		flow.Connections = []engine.NodeConnection{{ID: "conn-1", SourceNode: "node-1", TargetNode: "node-2"}}
		assert.NoError(t, flow.Validate())
	})

	t.Run("should fail validation with no nodes", func(t *testing.T) {
		flow := engine.NewFlow("no-nodes-flow", "No Nodes Flow")
		err := flow.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one node")
	})

	t.Run("should fail validation with connection to non-existent node", func(t *testing.T) {
		flow := engine.NewFlow("bad-conn-flow", "Bad Connection Flow")
		flow.Nodes["node-1"] = &engine.Node{ID: "node-1", Type: "debug"}
		flow.Connections = []engine.NodeConnection{{ID: "conn-1", SourceNode: "node-1", TargetNode: "node-does-not-exist"}}
		err := flow.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-existent")
	})

	t.Run("should fail validation with connection from non-existent node", func(t *testing.T) {
		flow := engine.NewFlow("bad-source-flow", "Bad Source Flow")
		flow.Nodes["node-1"] = &engine.Node{ID: "node-1", Type: "debug"}
		flow.Connections = []engine.NodeConnection{{ID: "conn-1", SourceNode: "node-does-not-exist", TargetNode: "node-1"}}
		err := flow.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-existent")
	})
}
