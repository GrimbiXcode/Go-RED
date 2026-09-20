package base

import "github.com/GrimbiXcode/Go-RED/internal/typedvalue"

// ParseValue reads a typed input ({"type": "str", "value": "hello"}) from
// its config-map form. Anything that is not a map, or a map without
// string "type"/"value" entries, yields the zero Value (an empty type,
// which typedvalue reports as unsupported when resolved), so a node's
// Validate can reject a missing field with its own message.
func ParseValue(raw interface{}) typedvalue.Value {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return typedvalue.Value{}
	}
	t, _ := m["type"].(string)
	v, _ := m["value"].(string)
	return typedvalue.Value{Type: typedvalue.Type(t), Value: v}
}

// ValueToConfig is the inverse of ParseValue, for GetConfig.
func ValueToConfig(v typedvalue.Value) map[string]interface{} {
	return map[string]interface{}{"type": string(v.Type), "value": v.Value}
}

// ParsePropertyRef reads a property reference ({"type": "msg", "path":
// "payload"}) from its config-map form, returning def when raw is not a
// map or has no "type" (the usual default being msg.payload).
func ParsePropertyRef(raw interface{}, def typedvalue.PropertyRef) typedvalue.PropertyRef {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return def
	}
	t, _ := m["type"].(string)
	p, _ := m["path"].(string)
	if t == "" {
		return def
	}
	return typedvalue.PropertyRef{Type: typedvalue.Type(t), Path: p}
}

// PropertyRefToConfig is the inverse of ParsePropertyRef, for GetConfig.
func PropertyRefToConfig(ref typedvalue.PropertyRef) map[string]interface{} {
	return map[string]interface{}{"type": string(ref.Type), "path": ref.Path}
}
