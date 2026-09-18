// Package state provides persistence for flows and configurations.
package state

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

// FileStateManager persists one JSON file per flow under <basePath>/flows.
type FileStateManager struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStateManager creates the data directories if needed.
func NewFileStateManager(basePath string) (*FileStateManager, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(basePath, "flows"), 0o755); err != nil {
		return nil, err
	}
	return &FileStateManager{basePath: basePath}, nil
}

// flowPath returns the file for flowID, refusing any ID that could escape
// the flows directory (see engine.ValidateFlowID).
func (sm *FileStateManager) flowPath(flowID string) (string, error) {
	if err := engine.ValidateFlowID(flowID); err != nil {
		return "", err
	}
	return filepath.Join(sm.basePath, "flows", flowID+".json"), nil
}

// SaveFlow writes the flow atomically: the JSON goes to a temporary file in
// the same directory, is fsynced, and is then renamed over the target, so a
// crash mid-write never leaves a truncated flow file behind.
func (sm *FileStateManager) SaveFlow(flow *engine.Flow) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	path, err := sm.flowPath(flow.ID)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal flow: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+flow.ID+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp flow file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to write flow file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("failed to sync flow file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("failed to close flow file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		cleanup()
		return fmt.Errorf("failed to set flow file mode: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("failed to replace flow file: %w", err)
	}
	return nil
}

// LoadFlow reads a single flow. A missing file yields engine.ErrFlowNotFound.
func (sm *FileStateManager) LoadFlow(flowID string) (*engine.Flow, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.loadFlowLocked(flowID)
}

func (sm *FileStateManager) loadFlowLocked(flowID string) (*engine.Flow, error) {
	path, err := sm.flowPath(flowID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", engine.ErrFlowNotFound, flowID)
		}
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	var flow engine.Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow: %w", err)
	}
	if flow.ID == "" {
		flow.ID = flowID
	}
	if flow.Nodes == nil {
		flow.Nodes = make(map[string]*engine.Node)
	}
	return &flow, nil
}

// LoadAllFlows reads every *.json file in the flows directory. Files that
// cannot be parsed are logged and skipped rather than failing the whole
// load, so one corrupt file never takes every other flow down with it.
func (sm *FileStateManager) LoadAllFlows() ([]*engine.Flow, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	flowsPath := filepath.Join(sm.basePath, "flows")
	entries, err := os.ReadDir(flowsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read flows directory: %w", err)
	}

	var flows []*engine.Flow
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".json" || strings.HasPrefix(name, ".") {
			continue
		}
		flowID := strings.TrimSuffix(name, ".json")
		flow, err := sm.loadFlowLocked(flowID)
		if err != nil {
			slog.Warn("skipping unreadable flow file", "file", filepath.Join(flowsPath, name), "err", err)
			continue
		}
		flows = append(flows, flow)
	}
	return flows, nil
}

// DeleteFlow removes a flow file. A missing file yields engine.ErrFlowNotFound.
func (sm *FileStateManager) DeleteFlow(flowID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	path, err := sm.flowPath(flowID)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", engine.ErrFlowNotFound, flowID)
		}
		return fmt.Errorf("failed to delete flow file: %w", err)
	}
	return nil
}

var _ engine.StateManager = (*FileStateManager)(nil)
