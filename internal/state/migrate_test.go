package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GrimbiXcode/Go-RED/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useSchema swaps the package-level schema version and migration chain for
// the duration of the test and restores the real ones afterwards.
func useSchema(t *testing.T, current int, chain map[int]func(map[string]any) error) {
	t.Helper()
	savedVersion, savedMigrations := currentSchemaVersion, migrations
	currentSchemaVersion, migrations = current, chain
	t.Cleanup(func() {
		currentSchemaVersion, migrations = savedVersion, savedMigrations
	})
}

// writeRawFlowFile puts content verbatim at <basePath>/flows/<flowID>.json,
// bypassing SaveFlow so tests can produce legacy, foreign or broken files.
func writeRawFlowFile(t *testing.T, basePath, flowID, content string) string {
	t.Helper()
	path := filepath.Join(basePath, "flows", flowID+".json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// readRawFlowFile decodes a flow file into a plain map, so tests can look at
// keys engine.Flow does not know about (schemaVersion).
func readRawFlowFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	return raw
}

// appendToName returns a migration that tags the flow name, so a test can
// read off which steps ran and in which order.
func appendToName(suffix string) func(map[string]any) error {
	return func(raw map[string]any) error {
		name, _ := raw["name"].(string)
		raw["name"] = name + suffix
		return nil
	}
}

func TestSchemaVersionOf(t *testing.T) {
	tests := []struct {
		name    string
		raw     map[string]any
		want    int
		wantErr bool
	}{
		{name: "absent counts as 1", raw: map[string]any{"id": "x"}, want: 1},
		{name: "null counts as 1", raw: map[string]any{schemaVersionKey: nil}, want: 1},
		{name: "zero counts as 1", raw: map[string]any{schemaVersionKey: json.Number("0")}, want: 1},
		{name: "json.Number", raw: map[string]any{schemaVersionKey: json.Number("3")}, want: 3},
		{name: "float64 from plain Unmarshal", raw: map[string]any{schemaVersionKey: float64(2)}, want: 2},
		{name: "int", raw: map[string]any{schemaVersionKey: 4}, want: 4},
		{name: "fraction is rejected", raw: map[string]any{schemaVersionKey: 1.5}, wantErr: true},
		{name: "fraction as json.Number is rejected", raw: map[string]any{schemaVersionKey: json.Number("1.5")}, wantErr: true},
		{name: "negative is rejected", raw: map[string]any{schemaVersionKey: json.Number("-1")}, wantErr: true},
		{name: "string is rejected", raw: map[string]any{schemaVersionKey: "1"}, wantErr: true},
		{name: "bool is rejected", raw: map[string]any{schemaVersionKey: true}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := schemaVersionOf(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileStateManagerSchemaVersion(t *testing.T) {
	t.Run("should write schemaVersion as a top-level key next to the flow fields", func(t *testing.T) {
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)

		flow := engine.NewFlow("versioned", "Versioned Flow")
		flow.AddNode(&engine.Node{ID: "node-1", Type: "function", X: 1, Y: 2})
		require.NoError(t, manager.SaveFlow(flow))

		raw := readRawFlowFile(t, filepath.Join(manager.basePath, "flows", "versioned.json"))
		assert.Equal(t, float64(currentSchemaVersion), raw[schemaVersionKey])
		// Flat object: the flow's own fields sit beside the version, not
		// under a wrapper key.
		assert.Equal(t, "versioned", raw["id"])
		assert.Equal(t, "Versioned Flow", raw["name"])
		assert.Contains(t, raw, "nodes")
		assert.NotContains(t, raw, "flow")

		// And the extra key does not leak into the decoded flow.
		loaded, err := manager.LoadFlow("versioned")
		require.NoError(t, err)
		assert.Equal(t, "Versioned Flow", loaded.Name)
		assert.Len(t, loaded.Nodes, 1)
	})

	t.Run("should load a legacy file without schemaVersion as version 1", func(t *testing.T) {
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		writeRawFlowFile(t, manager.basePath, "legacy", `{"id":"legacy","name":"Legacy","nodes":{"n1":{"id":"n1","type":"debug","config":{},"x":1,"y":2}}}`)

		loaded, err := manager.LoadFlow("legacy")
		require.NoError(t, err)
		assert.Equal(t, "Legacy", loaded.Name)
		assert.Len(t, loaded.Nodes, 1)
	})

	t.Run("should load a file with schemaVersion 0 as version 1", func(t *testing.T) {
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		writeRawFlowFile(t, manager.basePath, "zero", `{"schemaVersion":0,"id":"zero","name":"Zero"}`)

		loaded, err := manager.LoadFlow("zero")
		require.NoError(t, err)
		assert.Equal(t, "Zero", loaded.Name)
		assert.NotNil(t, loaded.Nodes)
	})

	t.Run("should keep large durations exact through the raw decode", func(t *testing.T) {
		// Timeouts are time.Duration in nanoseconds; the raw object path
		// must not squeeze them through a float.
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		writeRawFlowFile(t, manager.basePath, "exact", `{"id":"exact","name":"Exact","config":{"timeout":9007199254740993}}`)

		loaded, err := manager.LoadFlow("exact")
		require.NoError(t, err)
		assert.Equal(t, int64(9007199254740993), int64(loaded.Config.Timeout))
	})
}

func TestFileStateManagerMigration(t *testing.T) {
	t.Run("should run the chain from the file version up to current", func(t *testing.T) {
		useSchema(t, 3, map[int]func(map[string]any) error{
			1: appendToName("+m1"),
			2: appendToName("+m2"),
		})
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)

		writeRawFlowFile(t, manager.basePath, "absent", `{"id":"absent","name":"A"}`)
		writeRawFlowFile(t, manager.basePath, "zero", `{"schemaVersion":0,"id":"zero","name":"A"}`)
		writeRawFlowFile(t, manager.basePath, "one", `{"schemaVersion":1,"id":"one","name":"A"}`)
		writeRawFlowFile(t, manager.basePath, "two", `{"schemaVersion":2,"id":"two","name":"A"}`)
		writeRawFlowFile(t, manager.basePath, "three", `{"schemaVersion":3,"id":"three","name":"A"}`)

		want := map[string]string{
			"absent": "A+m1+m2",
			"zero":   "A+m1+m2",
			"one":    "A+m1+m2",
			"two":    "A+m2",
			"three":  "A",
		}
		for flowID, name := range want {
			loaded, err := manager.LoadFlow(flowID)
			require.NoError(t, err, flowID)
			assert.Equal(t, name, loaded.Name, flowID)
		}

		flows, err := manager.LoadAllFlows()
		require.NoError(t, err)
		assert.Len(t, flows, 5)
	})

	t.Run("should let a migration reshape the raw object before decoding", func(t *testing.T) {
		// A realistic step: version 1 called the field "title", version 2
		// calls it "name". engine.Flow would never see "title".
		useSchema(t, 2, map[int]func(map[string]any) error{
			1: func(raw map[string]any) error {
				raw["name"] = raw["title"]
				delete(raw, "title")
				return nil
			},
		})
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		writeRawFlowFile(t, manager.basePath, "renamed", `{"schemaVersion":1,"id":"renamed","title":"From Title"}`)

		loaded, err := manager.LoadFlow("renamed")
		require.NoError(t, err)
		assert.Equal(t, "From Title", loaded.Name)
	})

	t.Run("should stamp the current version on save so the next load skips the chain", func(t *testing.T) {
		calls := 0
		useSchema(t, 2, map[int]func(map[string]any) error{
			1: func(raw map[string]any) error { calls++; return nil },
		})
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)

		require.NoError(t, manager.SaveFlow(engine.NewFlow("fresh", "Fresh")))
		raw := readRawFlowFile(t, filepath.Join(manager.basePath, "flows", "fresh.json"))
		assert.Equal(t, float64(2), raw[schemaVersionKey])

		_, err = manager.LoadFlow("fresh")
		require.NoError(t, err)
		assert.Equal(t, 0, calls, "a file at the current version must not be migrated")
	})

	t.Run("should refuse a file newer than the current version without touching it", func(t *testing.T) {
		// Default schema: one version beyond what this build writes.
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		require.NoError(t, manager.SaveFlow(engine.NewFlow("ok", "OK")))
		content := fmt.Sprintf(`{"schemaVersion":%d,"id":"future","name":"From The Future"}`, currentSchemaVersion+1)
		path := writeRawFlowFile(t, manager.basePath, "future", content)

		_, err = manager.LoadFlow("future")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrSchemaVersionTooNew)
		assert.NotErrorIs(t, err, ErrCorruptFlowFile)

		flows, err := manager.LoadAllFlows()
		require.NoError(t, err)
		require.Len(t, flows, 1)
		assert.Equal(t, "ok", flows[0].ID)

		// Skipped, not quarantined and not modified.
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, string(data))
		_, err = os.Stat(filepath.Join(manager.basePath, "quarantine"))
		assert.True(t, os.IsNotExist(err), "a too-new file must not be quarantined")
	})

	t.Run("should fail when a step of the chain is missing", func(t *testing.T) {
		useSchema(t, 2, map[int]func(map[string]any) error{})
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		writeRawFlowFile(t, manager.basePath, "gap", `{"schemaVersion":1,"id":"gap","name":"Gap"}`)

		_, err = manager.LoadFlow("gap")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no migration from schema version 1 to 2")
		assert.NotErrorIs(t, err, ErrCorruptFlowFile)
	})

	t.Run("should report which step failed", func(t *testing.T) {
		useSchema(t, 2, map[int]func(map[string]any) error{
			1: func(raw map[string]any) error { return assert.AnError },
		})
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		writeRawFlowFile(t, manager.basePath, "broken", `{"schemaVersion":1,"id":"broken","name":"Broken"}`)

		_, err = manager.LoadFlow("broken")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "migration from schema version 1 to 2 failed")
	})

	t.Run("should reject a malformed schemaVersion without quarantining", func(t *testing.T) {
		manager, err := NewFileStateManager(t.TempDir())
		require.NoError(t, err)
		path := writeRawFlowFile(t, manager.basePath, "badver", `{"schemaVersion":"one","id":"badver"}`)

		_, err = manager.LoadFlow("badver")
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrCorruptFlowFile)

		flows, err := manager.LoadAllFlows()
		require.NoError(t, err)
		assert.Empty(t, flows)
		_, err = os.Stat(path)
		assert.NoError(t, err, "a well-formed object with a bad version stays in place")
	})
}
