package base

import (
	"testing"

	"github.com/GrimbiXcode/Go-RED/internal/typedvalue"
	"github.com/stretchr/testify/assert"
)

func TestParseValue(t *testing.T) {
	tests := []struct {
		name string
		raw  interface{}
		want typedvalue.Value
	}{
		{"typed map", map[string]interface{}{"type": "str", "value": "hello"}, typedvalue.Value{Type: typedvalue.TypeString, Value: "hello"}},
		{"msg path", map[string]interface{}{"type": "msg", "value": "payload.x"}, typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload.x"}},
		{"missing value", map[string]interface{}{"type": "num"}, typedvalue.Value{Type: typedvalue.TypeNumber}},
		{"non-string value is dropped", map[string]interface{}{"type": "num", "value": 5.0}, typedvalue.Value{Type: typedvalue.TypeNumber}},
		{"missing type", map[string]interface{}{"value": "x"}, typedvalue.Value{Value: "x"}},
		{"not a map", "hello", typedvalue.Value{}},
		{"nil", nil, typedvalue.Value{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseValue(tt.raw))
		})
	}
}

func TestValueToConfigRoundTrip(t *testing.T) {
	v := typedvalue.Value{Type: typedvalue.TypeJSON, Value: `{"a":1}`}
	cfg := ValueToConfig(v)
	assert.Equal(t, map[string]interface{}{"type": "json", "value": `{"a":1}`}, cfg)
	assert.Equal(t, v, ParseValue(cfg))
	assert.Equal(t, map[string]interface{}{"type": "", "value": ""}, ValueToConfig(typedvalue.Value{}))
}

func TestParsePropertyRef(t *testing.T) {
	def := typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}
	tests := []struct {
		name string
		raw  interface{}
		want typedvalue.PropertyRef
	}{
		{"msg ref", map[string]interface{}{"type": "msg", "path": "topic"}, typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "topic"}},
		{"flow ref", map[string]interface{}{"type": "flow", "path": "count"}, typedvalue.PropertyRef{Type: typedvalue.TypeFlow, Path: "count"}},
		{"missing path keeps the type", map[string]interface{}{"type": "global"}, typedvalue.PropertyRef{Type: typedvalue.TypeGlobal}},
		{"empty type falls back", map[string]interface{}{"type": "", "path": "x"}, def},
		{"missing type falls back", map[string]interface{}{"path": "x"}, def},
		{"not a map falls back", "payload", def},
		{"nil falls back", nil, def},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParsePropertyRef(tt.raw, def))
		})
	}
}

func TestPropertyRefToConfigRoundTrip(t *testing.T) {
	ref := typedvalue.PropertyRef{Type: typedvalue.TypeFlow, Path: "counter"}
	cfg := PropertyRefToConfig(ref)
	assert.Equal(t, map[string]interface{}{"type": "flow", "path": "counter"}, cfg)
	assert.Equal(t, ref, ParsePropertyRef(cfg, typedvalue.PropertyRef{}))
}
