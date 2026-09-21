package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

// handleVersion answers GET /api/version.
func (s *server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":   version,
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	})
}

// handleMetrics answers GET /metrics in the Prometheus text exposition
// format, written by hand so the server needs no client library for a
// handful of series.
func (s *server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var b strings.Builder
	header := func(name, help, kind string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
	}

	header("gored_build_info", "Build information of the running server.", "gauge")
	fmt.Fprintf(&b, "gored_build_info{version=%s,go=%s} 1\n", quoteLabel(version), quoteLabel(runtime.Version()))

	header("gored_uptime_seconds", "Seconds since the server started.", "gauge")
	fmt.Fprintf(&b, "gored_uptime_seconds %d\n", int(time.Since(s.startedAt).Seconds()))

	header("gored_flows", "Known flows by status.", "gauge")
	counts := s.engine.FlowCounts()
	for _, status := range []engine.FlowStatus{engine.FlowStatusActive, engine.FlowStatusInactive, engine.FlowStatusError} {
		fmt.Fprintf(&b, "gored_flows{status=%s} %d\n", quoteLabel(statusLabel(status)), counts[status])
	}

	stats := s.engine.NodeStats()
	header("gored_node_messages_total", "Messages handled per node of every running flow since its deploy.", "counter")
	for _, st := range stats {
		fmt.Fprintf(&b, "gored_node_messages_total{flow=%s,node=%s,type=%s} %d\n", quoteLabel(st.FlowID), quoteLabel(st.NodeID), quoteLabel(st.NodeType), st.Messages)
	}
	header("gored_node_errors_total", "Failed node executions per node of every running flow since its deploy.", "counter")
	for _, st := range stats {
		fmt.Fprintf(&b, "gored_node_errors_total{flow=%s,node=%s,type=%s} %d\n", quoteLabel(st.FlowID), quoteLabel(st.NodeID), quoteLabel(st.NodeType), st.Errors)
	}

	header("gored_messages_dropped_total", "Messages dropped because a flow queue was full.", "counter")
	fmt.Fprintf(&b, "gored_messages_dropped_total %d\n", s.engine.MessagesDropped())
	header("gored_events_dropped_total", "Runtime events dropped because the event queue was full.", "counter")
	fmt.Fprintf(&b, "gored_events_dropped_total %d\n", s.engine.EventsDropped())

	if counter, ok := s.notify.(interface{ ClientCount() int }); ok {
		header("gored_websocket_clients", "Connected editor WebSocket clients.", "gauge")
		fmt.Fprintf(&b, "gored_websocket_clients %d\n", counter.ClientCount())
	}

	header("go_goroutines", "Number of goroutines.", "gauge")
	fmt.Fprintf(&b, "go_goroutines %d\n", runtime.NumGoroutine())
	header("process_start_time_seconds", "Start time of the process since unix epoch in seconds.", "gauge")
	fmt.Fprintf(&b, "process_start_time_seconds %d\n", s.startedAt.Unix())

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

// statusLabel maps the engine's flow status to the editor's vocabulary.
func statusLabel(status engine.FlowStatus) string {
	switch status {
	case engine.FlowStatusActive:
		return "running"
	case engine.FlowStatusInactive:
		return "draft"
	default:
		return string(status)
	}
}

// quoteLabel escapes a label value for the exposition format.
func quoteLabel(v string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(v) + `"`
}
