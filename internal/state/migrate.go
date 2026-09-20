package state

import (
	"encoding/json"
	"fmt"
	"math"
)

// schemaVersionKey is the top-level key SaveFlow adds to every flow file. It
// sits next to the flow's own fields, so the file stays a flat JSON object
// that decodes straight into engine.Flow (which ignores the extra key).
const schemaVersionKey = "schemaVersion"

// currentSchemaVersion is the schema version SaveFlow stamps into every flow
// file. Bump it whenever the on-disk shape changes and add the matching
// entry to migrations. It is a variable rather than a constant only so tests
// can raise it to exercise the migration chain.
//
// History:
//
//	1  the format up to Phase 5 (files without the key count as 1)
//	2  config.maxConcurrency 0 means "engine default"; the former default
//	   of 100 was never enforced and now becomes 0 (see migrateV1)
var currentSchemaVersion = 2

// migrations maps a schema version v to the function that upgrades a raw
// flow object (the decoded JSON, mutated in place) from v to v+1. Loading a
// file runs the chain from the file's version up to currentSchemaVersion, so
// a v1 file read by a v3 binary passes through migrations[1] then
// migrations[2].
var migrations = map[int]func(map[string]any) error{
	1: migrateV1,
}

// migrateV1 upgrades a version 1 flow to version 2. Version 1 files carry
// config.maxConcurrency = 100, the placeholder engine.NewFlow used to write
// while nothing enforced it. Since version 2 the engine honors the value
// (docs/PROTOCOL.md), and 0 selects the engine-wide default, so the
// placeholder becomes 0; a value a user set through the API stays.
func migrateV1(raw map[string]any) error {
	config, ok := raw["config"].(map[string]any)
	if !ok {
		return nil
	}
	if value, ok := config["maxConcurrency"]; ok && numberEquals(value, 100) {
		config["maxConcurrency"] = 0
	}
	return nil
}

// numberEquals reports whether a decoded JSON number equals n.
func numberEquals(value any, n float64) bool {
	switch v := value.(type) {
	case json.Number:
		f, err := v.Float64()
		return err == nil && f == n
	case float64:
		return v == n
	case int:
		return float64(v) == n
	case int64:
		return float64(v) == n
	}
	return false
}

// schemaVersionOf returns the schema version recorded in raw. Files written
// before the key existed carry no version and count as version 1, as does an
// explicit 0.
func schemaVersionOf(raw map[string]any) (int, error) {
	value, ok := raw[schemaVersionKey]
	if !ok || value == nil {
		return 1, nil
	}

	var version int64
	switch v := value.(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid %s %q: %w", schemaVersionKey, v.String(), err)
		}
		version = n
	case float64:
		if v != math.Trunc(v) {
			return 0, fmt.Errorf("invalid %s %v: not an integer", schemaVersionKey, v)
		}
		version = int64(v)
	case int:
		version = int64(v)
	default:
		return 0, fmt.Errorf("invalid %s: expected a number, got %T", schemaVersionKey, value)
	}

	if version < 0 || version > math.MaxInt32 {
		return 0, fmt.Errorf("invalid %s %d: out of range", schemaVersionKey, version)
	}
	if version == 0 {
		return 1, nil
	}
	return int(version), nil
}

// migrate upgrades raw in place from the version it records to
// currentSchemaVersion and returns the version it found. A file newer than
// this binary understands yields ErrSchemaVersionTooNew and is left alone;
// the caller must not treat it as corrupt.
func migrate(raw map[string]any) (int, error) {
	found, err := schemaVersionOf(raw)
	if err != nil {
		return 0, err
	}
	if found > currentSchemaVersion {
		return found, fmt.Errorf("%w: file has version %d, this build supports up to %d",
			ErrSchemaVersionTooNew, found, currentSchemaVersion)
	}

	for v := found; v < currentSchemaVersion; v++ {
		step, ok := migrations[v]
		if !ok {
			return found, fmt.Errorf("no migration from schema version %d to %d", v, v+1)
		}
		if err := step(raw); err != nil {
			return found, fmt.Errorf("migration from schema version %d to %d failed: %w", v, v+1, err)
		}
	}
	raw[schemaVersionKey] = currentSchemaVersion
	return found, nil
}
