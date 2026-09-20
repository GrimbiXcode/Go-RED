package main

import (
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionEndpoint(t *testing.T) {
	_, h := newTestServer(t)
	w := do(t, h, http.MethodGet, "/api/version", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]string
	decodeBody(t, w, &body)
	assert.Equal(t, version, body["version"])
	assert.Equal(t, runtime.Version(), body["goVersion"])
	assert.Equal(t, runtime.GOOS, body["os"])
	assert.Equal(t, runtime.GOARCH, body["arch"])
}

func TestMetricsEndpoint(t *testing.T) {
	e, h := newTestServer(t)

	w := do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "m1", Name: "Metrics"})
	require.Equal(t, http.StatusCreated, w.Code)
	w = do(t, h, http.MethodPut, "/api/flows/m1", dto.FlowUpdateRequest{
		Nodes: map[string]dto.Node{
			"n1": {ID: "n1", Type: "inject", Position: dto.Position{}, Config: map[string]interface{}{"payload": map[string]interface{}{"payload": "tick"}, "interval": float64(20)}},
			"n2": {ID: "n2", Type: "debug", Position: dto.Position{}, Config: map[string]interface{}{}},
		},
		Connections: []dto.Connection{{ID: "c1", SourceNode: "n1", SourcePort: "output", TargetNode: "n2", TargetPort: "input"}},
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = do(t, h, http.MethodPost, "/api/flows/m1/deploy", nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	t.Cleanup(func() { _ = e.Undeploy("m1") })

	require.Eventually(t, func() bool { return e.GetMetrics("m1")["n2"].Messages > 0 }, 5*time.Second, 20*time.Millisecond)

	w = do(t, h, http.MethodGet, "/metrics", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	body := w.Body.String()
	assert.Contains(t, body, `gored_build_info{version="`+version+`"`)
	assert.Contains(t, body, `gored_flows{status="running"} 1`)
	assert.Contains(t, body, `gored_node_messages_total{flow="m1",node="n1",type="inject"} `)
	assert.Regexp(t, `gored_node_messages_total\{flow="m1",node="n2",type="debug"\} [1-9]`, body)
	assert.Contains(t, body, `gored_node_errors_total{flow="m1",node="n2",type="debug"} 0`)
	assert.Contains(t, body, "gored_messages_dropped_total 0")
	assert.Contains(t, body, "gored_events_dropped_total 0")
	assert.Contains(t, body, "# TYPE go_goroutines gauge")
	assert.Contains(t, body, "process_start_time_seconds ")
}

func TestQuoteLabel(t *testing.T) {
	assert.Equal(t, `"plain"`, quoteLabel("plain"))
	assert.Equal(t, `"a\"b\\c\nd"`, quoteLabel("a\"b\\c\nd"))
}
