package main

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/state"
)

func getFlow(t *testing.T, h http.Handler, id string) dto.Flow {
	t.Helper()
	w := do(t, h, http.MethodGet, "/api/flows/"+id, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var flow dto.Flow
	decodeBody(t, w, &flow)
	return flow
}

func TestUpdateWithBaseRevision(t *testing.T) {
	_, h := newTestServer(t)
	require.Equal(t, http.StatusCreated, do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "rev", Name: "Rev"}).Code)
	base := getFlow(t, h, "rev").Revision
	require.NotEmpty(t, base)

	// A save on the current revision passes and makes a new one.
	name := "First"
	w := do(t, h, http.MethodPut, "/api/flows/rev", dto.FlowUpdateRequest{Name: &name, BaseRevision: base})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var saved dto.Flow
	decodeBody(t, w, &saved)
	assert.NotEqual(t, base, saved.Revision)
	assert.Equal(t, saved.Revision, getFlow(t, h, "rev").Revision, "the response carries the stored revision")

	// A save on the old revision changes nothing and returns the flow as it is.
	other := "Second"
	w = do(t, h, http.MethodPut, "/api/flows/rev", dto.FlowUpdateRequest{Name: &other, BaseRevision: base})
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var conflict dto.FlowConflictResponse
	decodeBody(t, w, &conflict)
	assert.Equal(t, dto.RevisionConflictCode, conflict.Code)
	assert.Equal(t, "First", conflict.Flow.Name)
	assert.Equal(t, saved.Revision, conflict.Flow.Revision)
	assert.Equal(t, "First", getFlow(t, h, "rev").Name)

	// Without a base revision the last writer wins, as before.
	w = do(t, h, http.MethodPut, "/api/flows/rev", dto.FlowUpdateRequest{Name: &other})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Second", getFlow(t, h, "rev").Name)

	// Moving the tab is not an edit: the revision stays.
	before := getFlow(t, h, "rev").Revision
	order := 3
	require.Equal(t, http.StatusOK, do(t, h, http.MethodPut, "/api/flows/rev", dto.FlowUpdateRequest{Order: &order}).Code)
	assert.Equal(t, before, getFlow(t, h, "rev").Revision)
}

func TestConcurrentSavesOnOneRevisionLetExactlyOneThrough(t *testing.T) {
	_, h := newTestServer(t)
	require.Equal(t, http.StatusCreated, do(t, h, http.MethodPost, "/api/flows", dto.FlowCreateRequest{ID: "race", Name: "Race"}).Code)
	base := getFlow(t, h, "race").Revision

	const writers = 20
	codes := make([]int, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "writer"
			codes[i] = do(t, h, http.MethodPut, "/api/flows/race", dto.FlowUpdateRequest{Name: &name, BaseRevision: base}).Code
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, code := range codes {
		if code == http.StatusOK {
			ok++
		} else {
			assert.Equal(t, http.StatusConflict, code)
		}
	}
	assert.Equal(t, 1, ok)
}

func TestRevisionsAreStrictlyOrderedAndSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	open := func() *engine.FlowEngine {
		sm, err := state.NewFileStateManagerWithOptions(dir, state.Options{})
		require.NoError(t, err)
		e := engine.NewFlowEngine(engine.EngineConfig{MessageBufferSize: 10, DefaultTimeout: time.Second}, registry.GetGlobalRegistry())
		e.SetStateManager(sm)
		require.NoError(t, e.Start())
		t.Cleanup(func() { e.Stop() })
		require.NoError(t, e.LoadAllFlows())
		return e
	}

	e := open()
	_, err := e.CreateFlow("persisted", "Persisted", "")
	require.NoError(t, err)
	seen := map[string]bool{}
	var last *engine.Flow
	for i := 0; i < 50; i++ {
		last, err = e.UpdateFlow("persisted", func(f *engine.Flow) error { f.Description = "edit"; return nil })
		require.NoError(t, err)
		require.False(t, seen[last.Revision()], "edit %d repeated a revision", i)
		seen[last.Revision()] = true
	}

	restarted := open()
	flow, err := restarted.GetFlow("persisted")
	require.NoError(t, err)
	assert.Equal(t, last.Revision(), flow.Revision(), "an open editor's base revision is still current after a restart")
}

func TestFlowStatusEventCarriesTheRevision(t *testing.T) {
	updated := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	wire := dto.FlowStatusEventToWire(engine.FlowStatusEvent{FlowID: "f", Status: engine.FlowStatusInactive, UpdatedAt: updated})
	assert.Equal(t, engine.RevisionOf(updated), wire.Revision)
	assert.Equal(t, "2026-01-02T03:04:05.123456789Z", wire.Revision)
	assert.Empty(t, dto.FlowStatusEventToWire(engine.FlowStatusEvent{FlowID: "f"}).Revision)
}
