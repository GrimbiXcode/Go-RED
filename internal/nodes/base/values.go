package base

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// ToFloat converts any Go numeric value (float32/float64, every int and
// uint width, json.Number) to a float64. Strings are not parsed; see
// ParseFloat for that.
func ToFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// ParseFloat is ToFloat plus strings: a string is parsed with
// strconv.ParseFloat, so "5", "1e3" and "-0.5" all count as numbers. This
// is the numeric coercion Switch and Sort apply before comparing values;
// a node that must not treat "5" as a number (RBE's deadband, Range) uses
// ToFloat instead.
func ParseFloat(v interface{}) (float64, bool) {
	if s, ok := v.(string); ok {
		f, err := strconv.ParseFloat(s, 64)
		return f, err == nil
	}
	return ToFloat(v)
}

// ToInt converts a numeric value to an int. Integer types are converted
// directly; float32/float64 and json.Number are truncated toward zero, the
// way JSON-decoded config numbers have always been read by nodes
// (int(f)). NaN, ±Inf and values outside the int range yield false.
func ToInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		if int64(int(n)) != n {
			return 0, false
		}
		return int(n), true
	case uint:
		if n > math.MaxInt {
			return 0, false
		}
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		if n > math.MaxInt {
			return 0, false
		}
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return ToInt(i)
		}
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return floatToInt(f)
	case float32:
		return floatToInt(float64(n))
	case float64:
		return floatToInt(n)
	default:
		return 0, false
	}
}

// floatToInt truncates f toward zero, rejecting NaN, ±Inf and anything
// that does not fit an int.
func floatToInt(f float64) (int, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	if f >= math.MaxInt || f < math.MinInt {
		return 0, false
	}
	return int(f), true
}

// ToBool converts a bool, or a string strconv.ParseBool understands
// ("true", "false", "1", "0", "t", "f", ...), to a bool. Anything else
// yields false, false.
func ToBool(v interface{}) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		parsed, err := strconv.ParseBool(b)
		return parsed, err == nil
	default:
		return false, false
	}
}

// ToString renders a message value the way Node-RED's template node
// does: nil is "", a string is returned as is, a float64 is formatted
// without an exponent (1000000, not 1e+06) and without trailing zeros,
// a bool is "true"/"false", and anything else (maps, slices, other
// numbers) is JSON. A value JSON cannot encode falls back to fmt's %v.
func ToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// ToBytes turns a message payload into the bytes a network or file
// output node writes: a []byte is used as is, a string is its bytes, nil
// is empty, a bool or float64 is its text form (fmt.Sprint), and
// anything else is JSON. The error wraps json.Marshal's for a value it
// cannot encode.
func ToBytes(v interface{}) ([]byte, error) {
	switch p := v.(type) {
	case []byte:
		return p, nil
	case string:
		return []byte(p), nil
	case nil:
		return []byte{}, nil
	case bool, float64:
		return []byte(fmt.Sprint(p)), nil
	default:
		encoded, err := json.Marshal(p)
		if err != nil {
			return nil, fmt.Errorf("payload cannot be encoded: %w", err)
		}
		return encoded, nil
	}
}

// CloneMap returns a shallow copy of src: a new map with the same keys
// and the same (not copied) values, so a node can add or replace top-level
// message properties without mutating the message it was given. A nil src
// yields an empty, non-nil map.
func CloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// FloatPtr returns a pointer to f, for the Min/Max fields of a
// registry.Property in a node's config schema.
func FloatPtr(f float64) *float64 { return &f }

// StringsToConfig converts a []string to the []interface{} form a
// GetConfig map holds (the shape JSON decoding produces). A nil or empty
// input yields an empty, non-nil slice.
func StringsToConfig(values []string) []interface{} {
	out := make([]interface{}, len(values))
	for i, s := range values {
		out[i] = s
	}
	return out
}
