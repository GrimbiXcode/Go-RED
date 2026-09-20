package state

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateV1_MaxConcurrencyPlaceholder(t *testing.T) {
	manager, err := NewFileStateManager(t.TempDir())
	require.NoError(t, err)

	cases := map[string]struct {
		content string
		want    int
	}{
		"v1 placeholder 100 becomes engine default": {`{"id":"a","name":"A","config":{"maxConcurrency":100}}`, 0},
		"v1 explicit value stays":                   {`{"schemaVersion":1,"id":"a","name":"A","config":{"maxConcurrency":7}}`, 7},
		"v1 without config":                         {`{"id":"a","name":"A"}`, 0},
		"v2 keeps 100":                              {`{"schemaVersion":2,"id":"a","name":"A","config":{"maxConcurrency":100}}`, 100},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			writeRawFlowFile(t, manager.basePath, "a", tc.content)
			flow, err := manager.LoadFlow("a")
			require.NoError(t, err)
			assert.Equal(t, tc.want, flow.Config.MaxConcurrency)

			// Saving rewrites the file at the current version.
			require.NoError(t, manager.SaveFlow(flow))
			raw := readRawFlowFile(t, filepath.Join(manager.basePath, "flows", "a.json"))
			assert.Equal(t, float64(currentSchemaVersion), raw[schemaVersionKey])
		})
	}
}
