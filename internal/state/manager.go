// Package state provides persistence for flows and configurations.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

type FileStateManager struct {
	basePath string
	mu sync.RWMutex
}

func NewFileStateManager(basePath string) (*FileStateManager, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(basePath, "flows"), 0755); err != nil {
		return nil, err
	}
	return &FileStateManager{basePath: basePath}, nil
}

func (sm *FileStateManager) SaveFlow(flow *engine.Flow) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return errors.New("failed to marshal flow: " + err.Error())
	}
	
	path := filepath.Join(sm.basePath, "flows", flow.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return errors.New("failed to write flow file: " + err.Error())
	}
	return nil
}

func (sm *FileStateManager) LoadFlow(flowID string) (*engine.Flow, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	path := filepath.Join(sm.basePath, "flows", flowID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("flow not found: " + flowID)
		}
		return nil, errors.New("failed to read flow file: " + err.Error())
	}
	
	var flow engine.Flow
	if err := json.Unmarshal(data, &flow); err != nil {
		return nil, errors.New("failed to unmarshal flow: " + err.Error())
	}
	return &flow, nil
}

func (sm *FileStateManager) LoadAllFlows() ([]*engine.Flow, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	flowsPath := filepath.Join(sm.basePath, "flows")
	entries, err := os.ReadDir(flowsPath)
	if err != nil {
		return nil, errors.New("failed to read flows directory: " + err.Error())
	}
	
	var flows []*engine.Flow
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		flowID := entry.Name()[:len(entry.Name())-5]
		flow, err := sm.LoadFlow(flowID)
		if err != nil {
			continue
		}
		flows = append(flows, flow)
	}
	return flows, nil
}

func (sm *FileStateManager) DeleteFlow(flowID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	path := filepath.Join(sm.basePath, "flows", flowID+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return errors.New("flow not found: " + flowID)
		}
		return errors.New("failed to delete flow file: " + err.Error())
	}
	return nil
}
