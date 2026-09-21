package base

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToFloat(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want float64
		ok   bool
	}{
		{"float64", 1.5, 1.5, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 3, 3, true},
		{"int8", int8(-4), -4, true},
		{"int16", int16(5), 5, true},
		{"int32", int32(6), 6, true},
		{"int64", int64(7), 7, true},
		{"uint", uint(8), 8, true},
		{"uint8", uint8(9), 9, true},
		{"uint16", uint16(10), 10, true},
		{"uint32", uint32(11), 11, true},
		{"uint64", uint64(12), 12, true},
		{"json.Number", json.Number("13.5"), 13.5, true},
		{"json.Number invalid", json.Number("abc"), 0, false},
		{"numeric string is not parsed", "14", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
		{"map", map[string]interface{}{}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want float64
		ok   bool
	}{
		{"float64", 1.5, 1.5, true},
		{"int", 2, 2, true},
		{"json.Number", json.Number("3"), 3, true},
		{"string integer", "5", 5, true},
		{"string float", "-0.25", -0.25, true},
		{"string exponent", "1e3", 1000, true},
		{"string with spaces", " 5", 0, false},
		{"string text", "five", 0, false},
		{"empty string", "", 0, false},
		{"nil", nil, 0, false},
		{"bool", false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseFloat(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want int
		ok   bool
	}{
		{"int", 1, 1, true},
		{"int8", int8(2), 2, true},
		{"int16", int16(3), 3, true},
		{"int32", int32(4), 4, true},
		{"int64", int64(5), 5, true},
		{"uint", uint(6), 6, true},
		{"uint8", uint8(7), 7, true},
		{"uint16", uint16(8), 8, true},
		{"uint32", uint32(9), 9, true},
		{"uint64", uint64(10), 10, true},
		{"uint64 overflow", uint64(math.MaxUint64), 0, false},
		{"float64 integral", 11.0, 11, true},
		{"float64 truncates toward zero", 11.9, 11, true},
		{"negative float truncates toward zero", -11.9, -11, true},
		{"float32", float32(12), 12, true},
		{"float NaN", math.NaN(), 0, false},
		{"float +Inf", math.Inf(1), 0, false},
		{"float -Inf", math.Inf(-1), 0, false},
		{"float too large", 1e19, 0, false},
		{"json.Number integer", json.Number("13"), 13, true},
		{"json.Number fraction truncates", json.Number("14.7"), 14, true},
		{"json.Number invalid", json.Number("x"), 0, false},
		{"numeric string is not parsed", "15", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToInt(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want bool
		ok   bool
	}{
		{"true", true, true, true},
		{"false", false, false, true},
		{"string true", "true", true, true},
		{"string false", "false", false, true},
		{"string 1", "1", true, true},
		{"string TRUE", "TRUE", true, true},
		{"string yes is not a bool", "yes", false, false},
		{"number", 1.0, false, false},
		{"nil", nil, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToBool(tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name string
		in   interface{}
		want string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"float integral", 5.0, "5"},
		{"float fraction", 1.25, "1.25"},
		{"float large has no exponent", 1000000.0, "1000000"},
		{"float small has no exponent", 0.000001, "0.000001"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"int is JSON", 7, "7"},
		{"map is JSON", map[string]interface{}{"a": 1.0, "b": "x"}, `{"a":1,"b":"x"}`},
		{"slice is JSON", []interface{}{1.0, "two", nil}, `[1,"two",null]`},
		{"json.Number", json.Number("42"), "42"},
		{"unencodable falls back to %v", func() {}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToString(tt.in)
			if tt.name == "unencodable falls back to %v" {
				assert.NotEmpty(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToBytes(t *testing.T) {
	tests := []struct {
		name    string
		in      interface{}
		want    []byte
		wantErr bool
	}{
		{"bytes as is", []byte{1, 2, 3}, []byte{1, 2, 3}, false},
		{"string", "hi", []byte("hi"), false},
		{"nil is empty", nil, []byte{}, false},
		{"bool", true, []byte("true"), false},
		{"float", 1.5, []byte("1.5"), false},
		{"float integral", 3.0, []byte("3"), false},
		{"map is JSON", map[string]interface{}{"a": 1.0}, []byte(`{"a":1}`), false},
		{"slice is JSON", []interface{}{"x", 2.0}, []byte(`["x",2]`), false},
		{"int is JSON", 9, []byte("9"), false},
		{"unencodable", func() {}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToBytes(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "payload cannot be encoded")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCloneMap(t *testing.T) {
	inner := map[string]interface{}{"deep": 1.0}
	src := map[string]interface{}{"a": 1.0, "b": inner}

	dst := CloneMap(src)
	assert.Equal(t, src, dst)

	dst["a"] = 2.0
	dst["c"] = "new"
	assert.Equal(t, 1.0, src["a"], "top-level change must not reach the source")
	assert.NotContains(t, src, "c")

	// Shallow: nested values are shared, as every node's cloneMap did.
	dst["b"].(map[string]interface{})["deep"] = 2.0
	assert.Equal(t, 2.0, inner["deep"])

	nilClone := CloneMap(nil)
	require.NotNil(t, nilClone)
	assert.Empty(t, nilClone)
}

func TestFloatPtr(t *testing.T) {
	p := FloatPtr(1.5)
	require.NotNil(t, p)
	assert.Equal(t, 1.5, *p)
	q := FloatPtr(1.5)
	assert.NotSame(t, p, q, "each call returns its own pointer")
}

func TestStringsToConfig(t *testing.T) {
	assert.Equal(t, []interface{}{"a", "b"}, StringsToConfig([]string{"a", "b"}))
	empty := StringsToConfig(nil)
	require.NotNil(t, empty)
	assert.Empty(t, empty)
}
