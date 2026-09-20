package base

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigHas(t *testing.T) {
	c := Config{"present": nil, "value": 1.0}
	assert.True(t, c.Has("present"), "a nil value still counts as present")
	assert.True(t, c.Has("value"))
	assert.False(t, c.Has("missing"))
	assert.False(t, Config(nil).Has("anything"))
}

func TestConfigString(t *testing.T) {
	c := Config{"s": "hello", "empty": "", "n": 1.0, "nil": nil}
	tests := []struct {
		name, key, def, want string
	}{
		{"string", "s", "d", "hello"},
		{"empty string is returned as is", "empty", "d", ""},
		{"number falls back", "n", "d", "d"},
		{"nil falls back", "nil", "d", "d"},
		{"missing falls back", "missing", "d", "d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.String(tt.key, tt.def))
		})
	}
}

func TestConfigInt(t *testing.T) {
	c := Config{
		"f":        5000.0,
		"frac":     7.9,
		"i":        3,
		"i64":      int64(4),
		"num":      json.Number("12"),
		"numfrac":  json.Number("12.5"),
		"str":      "42",
		"strfrac":  "42.9",
		"strtext":  "many",
		"bool":     true,
		"nil":      nil,
		"negative": -2.0,
	}
	tests := []struct {
		name string
		key  string
		def  int
		want int
	}{
		{"float64", "f", 1, 5000},
		{"float64 truncates", "frac", 1, 7},
		{"int", "i", 1, 3},
		{"int64", "i64", 1, 4},
		{"json.Number", "num", 1, 12},
		{"json.Number fraction truncates", "numfrac", 1, 12},
		{"numeric string", "str", 1, 42},
		{"numeric string fraction truncates", "strfrac", 1, 42},
		{"text string falls back", "strtext", 1, 1},
		{"bool falls back", "bool", 1, 1},
		{"nil falls back", "nil", 1, 1},
		{"missing falls back", "missing", 9, 9},
		{"negative", "negative", 1, -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Int(tt.key, tt.def))
		})
	}
}

func TestConfigInt64(t *testing.T) {
	c := Config{"f": 5000.0, "num": json.Number("7"), "str": "8", "bad": "x", "big": 1e19}
	assert.Equal(t, int64(5000), c.Int64("f", 1))
	assert.Equal(t, int64(7), c.Int64("num", 1))
	assert.Equal(t, int64(8), c.Int64("str", 1))
	assert.Equal(t, int64(1), c.Int64("bad", 1))
	assert.Equal(t, int64(1), c.Int64("big", 1), "out of range falls back")
	assert.Equal(t, int64(1), c.Int64("missing", 1))
}

func TestConfigFloat(t *testing.T) {
	c := Config{"f": 1.5, "i": 2, "num": json.Number("2.5"), "str": "3.5", "bad": "x", "nil": nil}
	tests := []struct {
		name string
		key  string
		def  float64
		want float64
	}{
		{"float64", "f", 0, 1.5},
		{"int", "i", 0, 2},
		{"json.Number", "num", 0, 2.5},
		{"numeric string", "str", 0, 3.5},
		{"text falls back", "bad", 100, 100},
		{"nil falls back", "nil", 100, 100},
		{"missing falls back", "missing", 100, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Float(tt.key, tt.def))
		})
	}
}

func TestConfigBool(t *testing.T) {
	c := Config{"t": true, "f": false, "st": "true", "sf": "false", "n": 1.0, "bad": "maybe"}
	tests := []struct {
		name string
		key  string
		def  bool
		want bool
	}{
		{"true", "t", false, true},
		{"false", "f", true, false},
		{"string true", "st", false, true},
		{"string false", "sf", true, false},
		{"number falls back", "n", true, true},
		{"text falls back", "bad", false, false},
		{"missing falls back to default true", "missing", true, true},
		{"missing falls back to default false", "missing", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Bool(tt.key, tt.def))
		})
	}
}

func TestConfigDuration(t *testing.T) {
	c := Config{
		"ms":     1500.0,
		"int":    2,
		"num":    json.Number("250"),
		"str":    "1.5s",
		"strms":  "300",
		"bad":    "soon",
		"bool":   true,
		"nil":    nil,
		"frac":   0.5,
		"negate": -10.0,
	}
	tests := []struct {
		name string
		key  string
		def  time.Duration
		want time.Duration
	}{
		{"number is milliseconds", "ms", time.Second, 1500 * time.Millisecond},
		{"int is milliseconds", "int", time.Second, 2 * time.Millisecond},
		{"json.Number is milliseconds", "num", time.Second, 250 * time.Millisecond},
		{"string is parsed", "str", time.Second, 1500 * time.Millisecond},
		{"numeric string is milliseconds", "strms", time.Second, 300 * time.Millisecond},
		{"text falls back", "bad", time.Second, time.Second},
		{"bool falls back", "bool", time.Second, time.Second},
		{"nil falls back", "nil", time.Second, time.Second},
		{"missing falls back", "missing", time.Minute, time.Minute},
		{"fractional milliseconds", "frac", time.Second, 500 * time.Microsecond},
		{"negative", "negate", time.Second, -10 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Duration(tt.key, tt.def))
		})
	}
}

func TestConfigStringSlice(t *testing.T) {
	c := Config{
		"list":   []interface{}{"a", "b"},
		"mixed":  []interface{}{"a", 1.0, nil, "b"},
		"empty":  []interface{}{},
		"single": "solo",
		"typed":  []string{"x", "y"},
		"number": 1.0,
	}
	tests := []struct {
		name string
		key  string
		want []string
	}{
		{"list of strings", "list", []string{"a", "b"}},
		{"non-strings are skipped", "mixed", []string{"a", "b"}},
		{"empty list is nil", "empty", nil},
		{"single string", "single", []string{"solo"}},
		{"[]string as is", "typed", []string{"x", "y"}},
		{"number is nil", "number", nil},
		{"missing is nil", "missing", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.StringSlice(tt.key))
		})
	}
}

func TestConfigMapAndSlice(t *testing.T) {
	inner := map[string]interface{}{"k": "v"}
	list := []interface{}{1.0, "two"}
	c := Config{"m": inner, "l": list, "s": "str"}

	assert.Equal(t, inner, c.Map("m"))
	assert.Nil(t, c.Map("l"))
	assert.Nil(t, c.Map("s"))
	assert.Nil(t, c.Map("missing"))

	assert.Equal(t, list, c.Slice("l"))
	assert.Nil(t, c.Slice("m"))
	assert.Nil(t, c.Slice("s"))
	assert.Nil(t, c.Slice("missing"))
}

func TestDecode(t *testing.T) {
	type inner struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	type cfg struct {
		Mode    string   `json:"mode"`
		DelayMs int64    `json:"delayMs"`
		Rate    int      `json:"rate"`
		Factor  float64  `json:"factor"`
		Drop    bool     `json:"drop"`
		Scope   []string `json:"scope"`
		Target  inner    `json:"target"`
		Rules   []inner  `json:"rules"`
		Missing string   `json:"missing"`
	}

	t.Run("fills a struct through json tags", func(t *testing.T) {
		config := map[string]interface{}{
			"mode":    "rate",
			"delayMs": 5000.0,
			"rate":    3.0,
			"factor":  1.5,
			"drop":    true,
			"scope":   []interface{}{"n1", "n2"},
			"target":  map[string]interface{}{"type": "msg", "value": "payload"},
			"rules":   []interface{}{map[string]interface{}{"type": "str", "value": "a"}},
			"unknown": "ignored",
		}
		var c cfg
		require.NoError(t, Decode(config, &c))
		assert.Equal(t, cfg{
			Mode:    "rate",
			DelayMs: 5000,
			Rate:    3,
			Factor:  1.5,
			Drop:    true,
			Scope:   []string{"n1", "n2"},
			Target:  inner{Type: "msg", Value: "payload"},
			Rules:   []inner{{Type: "str", Value: "a"}},
		}, c)
	})

	t.Run("json.Number decodes into numeric fields", func(t *testing.T) {
		var c cfg
		require.NoError(t, Decode(map[string]interface{}{"delayMs": json.Number("250"), "factor": json.Number("0.5")}, &c))
		assert.Equal(t, int64(250), c.DelayMs)
		assert.Equal(t, 0.5, c.Factor)
	})

	t.Run("missing keys keep the field's prior value", func(t *testing.T) {
		c := cfg{Mode: "delay", DelayMs: 1}
		require.NoError(t, Decode(map[string]interface{}{"rate": 2.0}, &c))
		assert.Equal(t, "delay", c.Mode)
		assert.Equal(t, int64(1), c.DelayMs)
		assert.Equal(t, 2, c.Rate)
	})

	t.Run("nil config is a no-op", func(t *testing.T) {
		var c cfg
		require.NoError(t, Decode(nil, &c))
		assert.Equal(t, cfg{}, c)
	})

	t.Run("wrong type names the key", func(t *testing.T) {
		var c cfg
		err := Decode(map[string]interface{}{"delayMs": "soon"}, &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"delayMs"`)
		assert.Contains(t, err.Error(), "string")
	})

	t.Run("fraction into an integer field names the key", func(t *testing.T) {
		var c cfg
		err := Decode(map[string]interface{}{"rate": 2.5}, &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"rate"`)
	})

	t.Run("wrong type in a nested struct names the path", func(t *testing.T) {
		var c cfg
		err := Decode(map[string]interface{}{"target": map[string]interface{}{"type": 1.0}}, &c)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "target.type")
	})

	t.Run("non-pointer destination is an error", func(t *testing.T) {
		var c cfg
		assert.Error(t, Decode(map[string]interface{}{}, c))
	})

	t.Run("unencodable config value is an error", func(t *testing.T) {
		var c cfg
		assert.Error(t, Decode(map[string]interface{}{"mode": func() {}}, &c))
	})
}
