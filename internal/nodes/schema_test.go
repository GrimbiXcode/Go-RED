package nodes_test

import (
	"strings"
	"testing"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/batch"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/catch"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/change"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/comment"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/complete"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/csvnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/debug"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/delay"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/execnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/file"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/filein"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/function"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/htmlnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httpin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httpproxy"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httprequest"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/httpresponse"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/inject"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/join"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/jsonnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/junction"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/linkin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/linkout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttbroker"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/mqttout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/rangenode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/rbe"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/sortnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/split"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/status"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/switchnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tcpin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tcpout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tcprequest"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/template"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/tlsconfig"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/trigger"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/udpin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/udpout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/watch"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketclient"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketin"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketlistener"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/websocketout"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/xmlnode"
	_ "github.com/GrimbiXcode/Go-RED/internal/nodes/yamlnode"
)

// TestEveryNodeSchemaIsWellFormed guards the editor contract: every built-in
// node declares a schema the edit tray can render (known widgets, list item
// schemas, valid visibility rules), every property has a label and an
// order, and every node has help text.
func TestEveryNodeSchemaIsWellFormed(t *testing.T) {
	all := registry.GetGlobalRegistry().GetAllNodes()
	require.GreaterOrEqual(t, len(all), 47, "all built-in nodes are registered")

	for _, meta := range all {
		t.Run(meta.Type, func(t *testing.T) {
			require.NoError(t, meta.Check())
			assert.NotEmpty(t, meta.Help, "help text")
			assert.NotEmpty(t, meta.Description, "description")

			seenOrder := map[int]string{}
			for name, prop := range meta.ConfigSchema.Properties {
				assert.NotEmpty(t, prop.Label, "label of %s", name)
				assert.NotEmpty(t, prop.Widget, "widget of %s", name)
				assert.Greater(t, prop.Order, 0, "order of %s", name)
				if other, dup := seenOrder[prop.Order]; dup {
					t.Errorf("properties %s and %s share order %d", name, other, prop.Order)
				}
				seenOrder[prop.Order] = name

				switch prop.Widget {
				case registry.WidgetDuration:
					assert.NotEmpty(t, prop.Unit, "unit of %s", name)
				case registry.WidgetCode:
					assert.NotEmpty(t, prop.Language, "language of %s", name)
				case registry.WidgetTypedInput:
					assert.Equal(t, "object", prop.Type, "typed input %s is stored as an object", name)
				case registry.WidgetList, registry.WidgetStringList:
					assert.Equal(t, "array", prop.Type, "%s is stored as an array", name)
				case registry.WidgetKeyValue, registry.WidgetJSON:
					assert.Contains(t, []string{"object", "array"}, prop.Type, "type of %s", name)
				}
			}
		})
	}
}

func TestSchemaValidateAgainstNodeDefaults(t *testing.T) {
	// A node's own defaults must pass its schema, except for required
	// properties whose default is a placeholder the user has to replace
	// (an empty URL, port 0).
	for _, meta := range registry.GetGlobalRegistry().GetAllNodes() {
		config := map[string]interface{}{}
		for name, prop := range meta.ConfigSchema.Properties {
			if prop.Default != nil {
				config[name] = prop.Default
			}
		}
		required := map[string]bool{}
		for _, name := range meta.ConfigSchema.Required {
			required[name] = true
		}
		for _, problem := range meta.ConfigSchema.Validate(config) {
			name := strings.SplitN(problem, " ", 2)[0]
			assert.True(t, required[name], "%s: %s (only required placeholders may fail)", meta.Type, problem)
		}
	}
}
