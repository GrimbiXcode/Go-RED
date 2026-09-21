package state

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
)

// backupsDir is where SaveFlow keeps earlier versions of flow files.
func (sm *FileStateManager) backupsDir() string {
	return filepath.Join(sm.basePath, "backups")
}

// backupName builds <flowID>.<UTC stamp>.json.
func backupName(flowID string, at time.Time) string {
	return flowID + "." + at.UTC().Format(fileTimeLayout) + ".json"
}

// parseBackupName extracts the timestamp from a backup file name of flowID.
// Flow IDs never contain a dot (engine.ValidateFlowID), so "<flowID>." is an
// unambiguous prefix and "a" does not claim the backups of "a-2".
func parseBackupName(name, flowID string) (time.Time, bool) {
	prefix := flowID + "."
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
		return time.Time{}, false
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".json")
	at, err := time.Parse(fileTimeLayout, stamp)
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

// Backups returns the paths of the flow's backup files, oldest first. A flow
// without backups (or a data directory without a backups folder) yields an
// empty list, not an error.
func (sm *FileStateManager) Backups(flowID string) ([]string, error) {
	if err := engine.ValidateFlowID(flowID); err != nil {
		return nil, err
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.listBackupsLocked(flowID)
}

// listBackupsLocked lists the flow's backups sorted oldest first. The stamps
// are fixed width, so sorting by name is sorting by time.
func (sm *FileStateManager) listBackupsLocked(flowID string) ([]string, error) {
	dir := sm.backupsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read backups directory: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := parseBackupName(entry.Name(), flowID); !ok {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

// backupLocked copies the flow file at path (the version about to be
// overwritten) to the backups directory, unless there is nothing to back up
// yet, backups are disabled, or the newest backup is younger than
// Options.BackupMinInterval. Afterwards it prunes the oldest backups so at
// most Options.BackupKeep remain. The caller holds the write lock.
func (sm *FileStateManager) backupLocked(flowID, path string) error {
	if sm.opts.BackupKeep <= 0 {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first save: nothing to preserve
		}
		return fmt.Errorf("failed to read flow file for backup: %w", err)
	}

	existing, err := sm.listBackupsLocked(flowID)
	if err != nil {
		return err
	}
	// Compare at the resolution of the file name, so two saves within the
	// same second can never produce the same name.
	stamp := sm.now().UTC().Truncate(time.Second)
	if len(existing) > 0 {
		newest, _ := parseBackupName(filepath.Base(existing[len(existing)-1]), flowID)
		if stamp.Sub(newest) <= sm.opts.BackupMinInterval {
			return nil
		}
	}

	dir := sm.backupsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create backups directory: %w", err)
	}
	target := filepath.Join(dir, backupName(flowID, stamp))
	if err := writeFileAtomic(target, "."+flowID+".*.tmp", data); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	backups, err := sm.listBackupsLocked(flowID)
	if err != nil {
		return err
	}
	for len(backups) > sm.opts.BackupKeep {
		oldest := backups[0]
		backups = backups[1:]
		if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to prune old flow backup", "flow", flowID, "file", oldest, "err", err)
		}
	}
	return nil
}
