package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaCheck(t *testing.T) {
	good := Schema{
		Properties: map[string]Property{
			"mode":  {Type: "string", Enum: []string{"a", "b"}, Widget: WidgetSelect},
			"count": {Type: "number", VisibleWhen: &Condition{Property: "mode", Values: []string{"a"}}},
			"rules": {Type: "array", Widget: WidgetList, Items: &Schema{
				Properties: map[string]Property{
					"value": {Type: "object", Widget: WidgetTypedInput, TypedInput: &TypedInputOptions{Types: ValueTypes, Default: TypedString}},
				},
				Required: []string{"value"},
			}},
			"code": {Type: "string", Widget: WidgetCode, Language: "javascript"},
			"wait": {Type: "number", Widget: WidgetDuration, Unit: "ms"},
		},
		Required: []string{"mode"},
	}
	require.NoError(t, good.Check())

	bad := []struct {
		name   string
		schema Schema
		want   string
	}{
		{"unknown widget", Schema{Properties: map[string]Property{"x": {Widget: "slider"}}}, "unknown widget"},
		{"unknown language", Schema{Properties: map[string]Property{"x": {Widget: WidgetCode, Language: "cobol"}}}, "unknown language"},
		{"unknown unit", Schema{Properties: map[string]Property{"x": {Widget: WidgetDuration, Unit: "min"}}}, "unknown unit"},
		{"list without items", Schema{Properties: map[string]Property{"x": {Widget: WidgetList}}}, "needs Items"},
		{"typed input without types", Schema{Properties: map[string]Property{"x": {Widget: WidgetTypedInput}}}, "needs TypedInput.Types"},
		{"typed input unknown type", Schema{Properties: map[string]Property{"x": {Widget: WidgetTypedInput, TypedInput: &TypedInputOptions{Types: []string{"jsonata"}}}}}, "unknown typed input type"},
		{"select without choices", Schema{Properties: map[string]Property{"x": {Widget: WidgetSelect}}}, "needs Enum or Options"},
		{"visibleWhen unknown", Schema{Properties: map[string]Property{"x": {VisibleWhen: &Condition{Property: "nope", Values: []string{"1"}}}}}, "unknown property"},
		{"required unknown", Schema{Required: []string{"nope"}}, "not defined"},
		{"bad pattern", Schema{Properties: map[string]Property{"x": {Pattern: "("}}}, "invalid pattern"},
		{"nested", Schema{Properties: map[string]Property{"x": {Widget: WidgetList, Items: &Schema{Properties: map[string]Property{"y": {Widget: "slider"}}}}}}, "items: property \"y\""},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.schema.Check()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestNodeMetadataCheck_OutputsFrom(t *testing.T) {
	meta := NodeMetadata{ConfigSchema: Schema{Properties: map[string]Property{"rules": {Type: "array"}}}}
	meta.OutputsFrom = &OutputsFrom{Property: "rules"}
	require.NoError(t, meta.Check())

	meta.OutputsFrom = &OutputsFrom{Property: "nope"}
	assert.ErrorContains(t, meta.Check(), "unknown property")

	meta.ConfigSchema.Properties["mode"] = Property{Type: "string"}
	meta.OutputsFrom = &OutputsFrom{Property: "mode"}
	assert.ErrorContains(t, meta.Check(), "must be an array")
}

func TestSchemaValidate(t *testing.T) {
	min, max := 1.0, 10.0
	schema := Schema{
		Properties: map[string]Property{
			"url":   {Type: "string", Pattern: "^https?://"},
			"port":  {Type: "number", Min: &min, Max: &max},
			"mode":  {Type: "string", Enum: []string{"a", "b"}, Default: "a"},
			"count": {Type: "number", VisibleWhen: &Condition{Property: "mode", Values: []string{"b"}}},
			"list":  {Type: "array"},
		},
		Required: []string{"url", "count", "list"},
	}

	assert.Nil(t, schema.Validate(map[string]interface{}{"url": "http://x", "port": 5.0, "list": []interface{}{"a"}}))

	problems := schema.Validate(map[string]interface{}{"url": "ftp://x", "port": 11, "mode": "c"})
	assert.Equal(t, []string{
		"list is required",
		"mode must be one of a, b",
		"port must be at most 10",
		"url must match ^https?://",
	}, problems)

	// count is only required while mode is "b".
	problems = schema.Validate(map[string]interface{}{"url": "http://x", "list": []interface{}{1}, "mode": "b"})
	assert.Equal(t, []string{"count is required"}, problems)

	// Empty strings and empty collections count as missing.
	problems = schema.Validate(map[string]interface{}{"url": "  ", "list": []interface{}{}})
	assert.Equal(t, []string{"list is required", "url is required"}, problems)
}
