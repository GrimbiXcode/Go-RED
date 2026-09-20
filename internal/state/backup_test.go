package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClock stands in for time.Now so backup tests advance time instead of
// sleeping through BackupMinInterval.
type fakeClock struct {
	current time.Time
}

func (c *fakeClock) Now() time.Time { return c.current }

func (c *fakeClock) Advance(d time.Duration) { c.current = c.current.Add(d) }

// newBackupTestManager returns a manager with the given options whose clock
// starts at a fixed instant, plus that clock.
func newBackupTestManager(t *testing.T, opts Options) (*FileStateManager, *fakeClock) {
	t.Helper()
	manager, err := NewFileStateManagerWithOptions(t.TempDir(), opts)
	require.NoError(t, err)
	clock := &fakeClock{current: time.Date(2026, 9, 20, 10, 15, 0, 0, time.UTC)}
	manager.now = clock.Now
	return manager, clock
}

// saveNamed saves a flow with the given name so tests can tell versions apart.
func saveNamed(t *testing.T, manager *FileStateManager, flowID, name string) {
	t.Helper()
	flow := engine.NewFlow(flowID, name)
	flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 1, Y: 2})
	require.NoError(t, manager.SaveFlow(flow))
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	assert.Equal(t, 5, opts.BackupKeep)
	assert.Equal(t, 10*time.Minute, opts.BackupMinInterval)

	t.Run("NewFileStateManager uses the defaults and a real clock", func(t *testing.T) {
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, DefaultOptions(), manager.opts)
		assert.NotNil(t, manager.now)
		assert.WithinDuration(t, time.Now(), manager.now(), time.Minute)
	})

	t.Run("negative options are clamped to zero", func(t *testing.T) {
		manager, err := NewFileStateManagerWithOptions(t.TempDir(), Options{BackupKeep: -1, BackupMinInterval: -time.Second})
		require.NoError(t, err)
		assert.Equal(t, Options{}, manager.opts)
	})
}

func TestFileTimeLayout(t *testing.T) {
	at := time.Date(2026, 9, 20, 10, 15, 42, 0, time.UTC)
	assert.Equal(t, "20260920T101542Z", at.Format(fileTimeLayout))
	parsed, err := time.Parse(fileTimeLayout, "20260920T101542Z")
	require.NoError(t, err)
	assert.True(t, parsed.Equal(at))
}

func TestFileStateManagerBackups(t *testing.T) {
	t.Run("should rotate backups on overwrite, throttled by the interval", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, DefaultOptions())
		backupsDir := filepath.Join(manager.basePath, "backups")

		// First save: nothing to back up.
		saveNamed(t, manager, "rot", "v1")
		backups, err := manager.Backups("rot")
		require.NoError(t, err)
		assert.Empty(t, backups)
		_, err = os.Stat(backupsDir)
		assert.True(t, os.IsNotExist(err), "the first save must not create the backups directory")

		// Second save: the previous content (v1) is preserved.
		clock.Advance(time.Second)
		saveNamed(t, manager, "rot", "v2")
		backups, err = manager.Backups("rot")
		require.NoError(t, err)
		require.Len(t, backups, 1)
		assert.Equal(t, filepath.Join(backupsDir, "rot.20260920T101501Z.json"), backups[0])
		raw := readRawFlowFile(t, backups[0])
		assert.Equal(t, "v1", raw["name"])
		assert.Equal(t, float64(currentSchemaVersion), raw[schemaVersionKey])

		// Autosave bursts within the interval add nothing.
		for i := 0; i < 5; i++ {
			clock.Advance(600 * time.Millisecond)
			saveNamed(t, manager, "rot", "v3")
		}
		clock.Advance(9 * time.Minute)
		saveNamed(t, manager, "rot", "v4")
		backups, err = manager.Backups("rot")
		require.NoError(t, err)
		assert.Len(t, backups, 1)

		// Once the newest backup is older than the interval, another one is made.
		clock.Advance(2 * time.Minute)
		saveNamed(t, manager, "rot", "v5")
		backups, err = manager.Backups("rot")
		require.NoError(t, err)
		require.Len(t, backups, 2)
		assert.Equal(t, filepath.Join(backupsDir, "rot.20260920T101501Z.json"), backups[0], "oldest first")
		assert.Equal(t, "v4", readRawFlowFile(t, backups[1])["name"], "the newest backup holds the content that was just overwritten")

		// The live file is untouched by all of this.
		loaded, err := manager.LoadFlow("rot")
		require.NoError(t, err)
		assert.Equal(t, "v5", loaded.Name)
	})

	t.Run("should prune the oldest backups beyond BackupKeep", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, Options{BackupKeep: 3, BackupMinInterval: 10 * time.Minute})
		start := clock.Now()

		saveNamed(t, manager, "prune", "v0")
		for i := 1; i <= 6; i++ {
			clock.Advance(11 * time.Minute)
			saveNamed(t, manager, "prune", "v"+string(rune('0'+i)))
		}

		backups, err := manager.Backups("prune")
		require.NoError(t, err)
		require.Len(t, backups, 3)
		// Saves 4, 5 and 6 backed up v3, v4 and v5 at start+44m, +55m, +66m.
		for i, minutes := range []int{44, 55, 66} {
			want := backupName("prune", start.Add(time.Duration(minutes)*time.Minute))
			assert.Equal(t, want, filepath.Base(backups[i]))
		}
		assert.Equal(t, "v3", readRawFlowFile(t, backups[0])["name"])
		assert.Equal(t, "v5", readRawFlowFile(t, backups[2])["name"])

		entries, err := os.ReadDir(manager.backupsDir())
		require.NoError(t, err)
		assert.Len(t, entries, 3, "pruned files are gone from disk, no temp files linger")
	})

	t.Run("should write backups that load as flows", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, DefaultOptions())
		saveNamed(t, manager, "restore", "before")
		clock.Advance(time.Minute)
		saveNamed(t, manager, "restore", "after")

		backups, err := manager.Backups("restore")
		require.NoError(t, err)
		require.Len(t, backups, 1)

		// Decoding the backup goes through the same path as a live file.
		data, err := os.ReadFile(backups[0])
		require.NoError(t, err)
		fromBackup, err := decodeFlowFile(data, "restore")
		require.NoError(t, err)
		assert.Equal(t, "before", fromBackup.Name)
		assert.Len(t, fromBackup.Nodes, 1)

		// And a hand restore (copy the backup over the live file) works.
		require.NoError(t, os.WriteFile(filepath.Join(manager.basePath, "flows", "restore.json"), data, 0o644))
		restored, err := manager.LoadFlow("restore")
		require.NoError(t, err)
		assert.Equal(t, "before", restored.Name)
	})

	t.Run("should keep backups when the flow is deleted", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, DefaultOptions())
		saveNamed(t, manager, "gone", "v1")
		clock.Advance(time.Minute)
		saveNamed(t, manager, "gone", "v2")

		require.NoError(t, manager.DeleteFlow("gone"))
		_, err := manager.LoadFlow("gone")
		assert.ErrorIs(t, err, engine.ErrFlowNotFound)

		backups, err := manager.Backups("gone")
		require.NoError(t, err)
		assert.Len(t, backups, 1)
	})

	t.Run("should make no backups when BackupKeep is 0", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, Options{BackupKeep: 0})
		saveNamed(t, manager, "off", "v1")
		clock.Advance(time.Hour)
		saveNamed(t, manager, "off", "v2")

		backups, err := manager.Backups("off")
		require.NoError(t, err)
		assert.Empty(t, backups)
		_, err = os.Stat(manager.backupsDir())
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("should back up every overwrite when the interval is 0 but never collide within a second", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, Options{BackupKeep: 10, BackupMinInterval: 0})
		saveNamed(t, manager, "fast", "v1")
		clock.Advance(time.Second)
		saveNamed(t, manager, "fast", "v2")
		clock.Advance(300 * time.Millisecond) // same second as the previous backup name
		saveNamed(t, manager, "fast", "v3")
		clock.Advance(time.Second)
		saveNamed(t, manager, "fast", "v4")

		backups, err := manager.Backups("fast")
		require.NoError(t, err)
		require.Len(t, backups, 2)
		assert.Equal(t, "v1", readRawFlowFile(t, backups[0])["name"])
		assert.Equal(t, "v3", readRawFlowFile(t, backups[1])["name"])
	})

	t.Run("should list only the backups of the asked flow, oldest first", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, Options{BackupKeep: 5})
		for _, id := range []string{"a", "a-2", "b"} {
			saveNamed(t, manager, id, "v1")
		}
		for i := 0; i < 2; i++ {
			clock.Advance(time.Minute)
			for _, id := range []string{"a", "a-2", "b"} {
				saveNamed(t, manager, id, "v2")
			}
		}
		// A stray file in the directory is ignored.
		require.NoError(t, os.WriteFile(filepath.Join(manager.backupsDir(), "a.notes.txt"), []byte("x"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(manager.backupsDir(), "a.json"), []byte("{}"), 0o644))

		backups, err := manager.Backups("a")
		require.NoError(t, err)
		require.Len(t, backups, 2)
		for _, path := range backups {
			_, ok := parseBackupName(filepath.Base(path), "a")
			assert.True(t, ok, path)
		}
		assert.Less(t, backups[0], backups[1])
	})

	t.Run("should return an empty list for a flow without backups", func(t *testing.T) {
		manager, _ := newBackupTestManager(t, DefaultOptions())
		backups, err := manager.Backups("nothing")
		require.NoError(t, err)
		assert.Empty(t, backups)
	})

	t.Run("should reject an invalid flow ID", func(t *testing.T) {
		manager, _ := newBackupTestManager(t, DefaultOptions())
		_, err := manager.Backups("../escape")
		assert.ErrorIs(t, err, engine.ErrInvalidFlowID)
	})

	t.Run("should still save when the backup cannot be written", func(t *testing.T) {
		manager, clock := newBackupTestManager(t, DefaultOptions())
		// A regular file where the backups directory should be makes every
		// backup attempt fail.
		require.NoError(t, os.WriteFile(manager.backupsDir(), []byte("blocker"), 0o644))

		saveNamed(t, manager, "nobackup", "v1")
		clock.Advance(time.Minute)
		saveNamed(t, manager, "nobackup", "v2")

		loaded, err := manager.LoadFlow("nobackup")
		require.NoError(t, err)
		assert.Equal(t, "v2", loaded.Name)
	})
}

func TestParseBackupName(t *testing.T) {
	at := time.Date(2026, 9, 20, 10, 15, 0, 0, time.UTC)
	tests := []struct {
		name   string
		file   string
		flowID string
		ok     bool
	}{
		{name: "own backup", file: "flow.20260920T101500Z.json", flowID: "flow", ok: true},
		{name: "other flow sharing the prefix", file: "flow-2.20260920T101500Z.json", flowID: "flow", ok: false},
		{name: "wrong extension", file: "flow.20260920T101500Z.txt", flowID: "flow", ok: false},
		{name: "no timestamp", file: "flow.json", flowID: "flow", ok: false},
		{name: "garbage timestamp", file: "flow.yesterday.json", flowID: "flow", ok: false},
		{name: "temp file", file: ".flow.123.tmp", flowID: "flow", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseBackupName(tt.file, tt.flowID)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.True(t, got.Equal(at))
			}
		})
	}
	assert.Equal(t, "flow.20260920T101500Z.json", backupName("flow", at))
}
