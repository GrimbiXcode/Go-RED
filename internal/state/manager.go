// Package state provides persistence for flows and configurations.
package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

var (
	// ErrCorruptFlowFile means a flow file is not valid JSON or not a JSON
	// object. LoadFlow reports it and leaves the file alone; LoadAllFlows
	// moves the file to the quarantine directory so the next start does not
	// trip over it again.
	ErrCorruptFlowFile = errors.New("corrupt flow file")

	// ErrSchemaVersionTooNew means a flow file was written by a newer Go-RED
	// whose schema this build cannot read. The file is skipped and never
	// modified, so downgrading and upgrading again loses nothing.
	ErrSchemaVersionTooNew = errors.New("flow file schema version too new")
)

// fileTimeLayout formats the UTC timestamp embedded in backup and quarantine
// file names. It is fixed width, so names sort chronologically.
const fileTimeLayout = "20060102T150405Z"

// Options tunes the FileStateManager's backup rotation.
type Options struct {
	// BackupKeep is how many backups SaveFlow keeps per flow; beyond that the
	// oldest are pruned. 0 disables backups.
	BackupKeep int

	// BackupMinInterval is how old the newest backup of a flow must be before
	// SaveFlow makes another one. SaveFlow runs on every autosave, several
	// times a second while someone edits, so without the interval a burst of
	// keystrokes would rotate every useful backup out. 0 backs up on every
	// overwrite.
	BackupMinInterval time.Duration
}

// DefaultOptions returns the options NewFileStateManager uses: five backups
// per flow, at most one every ten minutes.
func DefaultOptions() Options {
	return Options{
		BackupKeep:        5,
		BackupMinInterval: 10 * time.Minute,
	}
}

// FileStateManager persists one JSON file per flow under <basePath>/flows.
//
// Every file is the flow's own JSON object plus a top-level "schemaVersion"
// key (see migrate.go), so the engine's wire format is untouched and old
// files can be upgraded on load. The layout under basePath is:
//
//	flows/<flowID>.json                      the live flows
//	backups/<flowID>.<UTC stamp>.json        earlier versions, rotated by SaveFlow
//	quarantine/<flowID>.<UTC stamp>.json     files LoadAllFlows could not parse
type FileStateManager struct {
	basePath string
	opts     Options
	// now supplies the timestamps in backup and quarantine file names; tests
	// inject a clock here instead of sleeping.
	now func() time.Time
	mu  sync.RWMutex
}

// NewFileStateManager creates the data directories if needed and uses
// DefaultOptions for backups.
func NewFileStateManager(basePath string) (*FileStateManager, error) {
	return NewFileStateManagerWithOptions(basePath, DefaultOptions())
}

// NewFileStateManagerWithOptions creates the data directories if needed.
// Negative option values count as 0.
func NewFileStateManagerWithOptions(basePath string, opts Options) (*FileStateManager, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(basePath, "flows"), 0o755); err != nil {
		return nil, err
	}
	if opts.BackupKeep < 0 {
		opts.BackupKeep = 0
	}
	if opts.BackupMinInterval < 0 {
		opts.BackupMinInterval = 0
	}
	return &FileStateManager{basePath: basePath, opts: opts, now: time.Now}, nil
}

// flowPath returns the file for flowID, refusing any ID that could escape
// the flows directory (see engine.ValidateFlowID).
func (sm *FileStateManager) flowPath(flowID string) (string, error) {
	if err := engine.ValidateFlowID(flowID); err != nil {
		return "", err
	}
	return filepath.Join(sm.basePath, "flows", flowID+".json"), nil
}

// flowFile is the on-disk shape of a flow: the flow's own fields, flattened
// into the same object, plus the schema version they were written under.
type flowFile struct {
	SchemaVersion int `json:"schemaVersion"`
	*engine.Flow
}

// SaveFlow writes the flow atomically: the JSON goes to a temporary file in
// the same directory, is fsynced, and is then renamed over the target, so a
// crash mid-write never leaves a truncated flow file behind.
//
// When the flow already has a file, its current content is first copied to
// the backups directory, rate-limited and pruned as Options describes. A
// failed backup is logged and does not stop the save: losing a safety copy
// is better than losing the edit.
func (sm *FileStateManager) SaveFlow(flow *engine.Flow) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	path, err := sm.flowPath(flow.ID)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(flowFile{SchemaVersion: currentSchemaVersion, Flow: flow}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal flow: %w", err)
	}

	if err := sm.backupLocked(flow.ID, path); err != nil {
		slog.Error("flow backup failed, saving anyway", "flow", flow.ID, "file", path, "err", err)
	}

	return writeFileAtomic(path, "."+flow.ID+".*.tmp", data)
}

// writeFileAtomic writes data to path via a temporary file (created next to
// path with tmpPattern) that is fsynced, given mode 0644 and renamed over
// the target, so readers see either the old file or the complete new one.
func writeFileAtomic(path, tmpPattern string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), tmpPattern)
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

// LoadFlow reads a single flow. A missing file yields engine.ErrFlowNotFound,
// an unparseable one ErrCorruptFlowFile (the file stays where it is) and one
// from a newer Go-RED ErrSchemaVersionTooNew. Files from older schema
// versions are migrated in memory; the file itself is rewritten on the next
// SaveFlow.
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
	return decodeFlowFile(data, flowID)
}

// decodeFlowFile turns the bytes of a flow file into a Flow: it decodes the
// raw JSON object, runs the schema migrations it needs, and only then
// decodes into engine.Flow. flowID fills in a missing "id".
func decodeFlowFile(data []byte, flowID string) (*engine.Flow, error) {
	raw, err := decodeRawObject(data)
	if err != nil {
		return nil, err
	}
	if _, err := migrate(raw); err != nil {
		return nil, err
	}

	migrated, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to re-encode migrated flow: %w", err)
	}
	var flow engine.Flow
	if err := json.Unmarshal(migrated, &flow); err != nil {
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

// decodeRawObject decodes data as a JSON object, keeping numbers as
// json.Number so a later re-encode reproduces them exactly. Anything that is
// not a JSON object is reported as ErrCorruptFlowFile.
func decodeRawObject(data []byte) (map[string]any, error) {
	if !json.Valid(data) {
		return nil, fmt.Errorf("%w: invalid JSON", ErrCorruptFlowFile)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCorruptFlowFile, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: JSON null is not a flow object", ErrCorruptFlowFile)
	}
	return raw, nil
}

// LoadAllFlows reads every *.json file in the flows directory. One bad file
// never takes the other flows down with it: a file that is not a JSON
// object is moved to <basePath>/quarantine and logged; a file written by a
// newer schema version is logged and left in place; anything else that
// fails to load is logged and skipped.
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
		path := filepath.Join(flowsPath, name)
		flow, err := sm.loadFlowLocked(flowID)
		switch {
		case err == nil:
			flows = append(flows, flow)
		case errors.Is(err, ErrCorruptFlowFile):
			quarantined, moveErr := sm.quarantine(flowID, path)
			if moveErr != nil {
				slog.Error("corrupt flow file could not be quarantined", "file", path, "err", err, "quarantineErr", moveErr)
				continue
			}
			slog.Error("corrupt flow file moved to quarantine", "file", path, "quarantined", quarantined, "err", err)
		case errors.Is(err, ErrSchemaVersionTooNew):
			slog.Error("skipping flow file with unsupported schema version", "file", path, "err", err)
		default:
			slog.Warn("skipping unreadable flow file", "file", path, "err", err)
		}
	}
	return flows, nil
}

// quarantine moves the flow file at path to
// <basePath>/quarantine/<flowID>.<UTC stamp>.json and returns that path.
func (sm *FileStateManager) quarantine(flowID, path string) (string, error) {
	dir := filepath.Join(sm.basePath, "quarantine")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create quarantine directory: %w", err)
	}
	target := filepath.Join(dir, flowID+"."+sm.now().UTC().Format(fileTimeLayout)+".json")
	if err := os.Rename(path, target); err != nil {
		return "", fmt.Errorf("failed to move flow file to quarantine: %w", err)
	}
	return target, nil
}

// DeleteFlow removes a flow file. A missing file yields engine.ErrFlowNotFound.
// Backups of the flow are kept, so a deletion can still be undone by hand.
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
