package base

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Config is a node's configuration map as SetConfig receives it: the
// result of decoding the editor's JSON, so numbers are float64, lists are
// []interface{} and objects are map[string]interface{}. Its accessors
// coerce those to what a node field needs and return the given default
// when the key is missing or holds an unusable value, replacing the
// per-node "if v, ok := config["x"].(float64)" ladders.
type Config map[string]interface{}

// Has reports whether key is present, even if its value is nil.
func (c Config) Has(key string) bool {
	_, ok := c[key]
	return ok
}

// String returns the string stored under key, or def when the key is
// absent or not a string. An empty string is returned as is; a node that
// wants "" to mean "unset" checks for it itself.
func (c Config) String(key, def string) string {
	if s, ok := c[key].(string); ok {
		return s
	}
	return def
}

// Int returns the value under key as an int: any Go integer or float
// (truncated toward zero), a json.Number, or a numeric string. Otherwise
// def.
func (c Config) Int(key string, def int) int {
	if i, ok := ToInt(c[key]); ok {
		return i
	}
	if s, ok := c[key].(string); ok {
		if f, ok := ParseFloat(s); ok {
			if i, ok := floatToInt(f); ok {
				return i
			}
		}
	}
	return def
}

// Int64 is Int for an int64 field.
func (c Config) Int64(key string, def int64) int64 {
	if f, ok := c.float(key); ok {
		if i, ok := floatToInt(f); ok {
			return int64(i)
		}
	}
	return def
}

// Float returns the value under key as a float64 (any numeric type, a
// json.Number or a numeric string), or def.
func (c Config) Float(key string, def float64) float64 {
	if f, ok := c.float(key); ok {
		return f
	}
	return def
}

func (c Config) float(key string) (float64, bool) {
	return ParseFloat(c[key])
}

// Bool returns the value under key as a bool: a bool, or a string
// strconv.ParseBool accepts ("true"/"false", "1"/"0", ...). Otherwise def.
func (c Config) Bool(key string, def bool) bool {
	if b, ok := ToBool(c[key]); ok {
		return b
	}
	return def
}

// Duration returns the value under key as a time.Duration. A number is
// taken as milliseconds (the unit node schemas store durations in); a
// string goes through time.ParseDuration ("1.5s", "200ms"), or, if that
// fails, is read as a number of milliseconds. Otherwise def.
func (c Config) Duration(key string, def time.Duration) time.Duration {
	raw, ok := c[key]
	if !ok {
		return def
	}
	if s, ok := raw.(string); ok {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
	}
	if f, ok := ParseFloat(raw); ok {
		return time.Duration(f * float64(time.Millisecond))
	}
	return def
}

// StringSlice returns the strings under key: the string elements of a
// []interface{} (non-string elements are skipped), a []string as is, or a
// single string as a one-element slice. Anything else, including an
// empty list, yields nil.
func (c Config) StringSlice(key string) []string {
	var out []string
	switch v := c[key].(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = v
	case string:
		out = []string{v}
	}
	return out
}

// Map returns the object under key, or nil when the key is absent or not
// a map.
func (c Config) Map(key string) map[string]interface{} {
	m, _ := c[key].(map[string]interface{})
	return m
}

// Slice returns the list under key, or nil when the key is absent or not
// a list.
func (c Config) Slice(key string) []interface{} {
	s, _ := c[key].([]interface{})
	return s
}

// Decode fills dst, a pointer to a struct, from config through a JSON
// round trip: the struct's json tags name the config keys, nested structs
// and slices decode naturally, keys the struct does not declare are
// ignored, and a value of the wrong type is an error that names the key
// ("delayMs"). Numbers decode into any Go numeric field the way
// encoding/json does, so 5000 fits an int64 field but 5000.5 does not.
//
//	var c struct {
//		DelayMs int64 `json:"delayMs"`
//	}
//	if err := base.Decode(config, &c); err != nil { ... }
func Decode(config map[string]interface{}, dst interface{}) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			key := typeErr.Field
			if key == "" {
				key = "(root)"
			}
			return fmt.Errorf("config: key %q: cannot use %s as %s", key, typeErr.Value, typeErr.Type)
		}
		return fmt.Errorf("config: %w", err)
	}
	return nil
}
