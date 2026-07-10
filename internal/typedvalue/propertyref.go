package typedvalue

import (
    "fmt"
    "strings"
)

// ContextAccessor is the read-write counterpart to ContextGetter, needed by
// PropertyRef.Set/Delete for flow/global targets. registry.ContextStore
// satisfies it structurally.
type ContextAccessor interface {
    ContextGetter
    Set(key string, value interface{})
    Delete(key string)
}

// PropertyRef identifies a single mutable location a node can read, write,
// or delete: a dot-path into the message payload, or a flow/global context
// key. Used by nodes like Change (set/change/delete/move) and Switch (the
// property under test) wherever Node-RED's editor lets a user target
// "msg."/"flow."/"global." rather than only a literal.
type PropertyRef struct {
    Type Type   // TypeMsg, TypeFlow, or TypeGlobal - any other Type is invalid here
    Path string // dot-separated message path ("payload.count"), or a flat context key
}

// PropertyResolver supplies the data a PropertyRef reads from or writes to.
type PropertyResolver struct {
    Message       map[string]interface{}
    FlowContext   ContextAccessor
    GlobalContext ContextAccessor
}

// Get reads the value at ref. A missing message path or context key
// resolves to (nil, false), not an error, mirroring Value.Resolve's msg/
// flow/global handling.
func (ref PropertyRef) Get(r PropertyResolver) (interface{}, bool) {
    switch ref.Type {
    case TypeMsg:
        return lookupPath(r.Message, ref.Path)
    case TypeFlow:
        if r.FlowContext == nil {
            return nil, false
        }
        return r.FlowContext.Get(ref.Path)
    case TypeGlobal:
        if r.GlobalContext == nil {
            return nil, false
        }
        return r.GlobalContext.Get(ref.Path)
    default:
        return nil, false
    }
}

// Set writes value at ref, creating intermediate maps along Path as needed
// for TypeMsg (mirroring Node-RED's Change node "set" behavior, which does
// the same rather than erroring on a missing intermediate object).
func (ref PropertyRef) Set(r PropertyResolver, value interface{}) error {
    switch ref.Type {
    case TypeMsg:
        if r.Message == nil {
            return fmt.Errorf("typedvalue: message is nil")
        }
        setPath(r.Message, ref.Path, value)
        return nil
    case TypeFlow:
        if r.FlowContext == nil {
            return fmt.Errorf("typedvalue: flow context is not available")
        }
        r.FlowContext.Set(ref.Path, value)
        return nil
    case TypeGlobal:
        if r.GlobalContext == nil {
            return fmt.Errorf("typedvalue: global context is not available")
        }
        r.GlobalContext.Set(ref.Path, value)
        return nil
    default:
        return fmt.Errorf("typedvalue: unsupported property ref type %q", ref.Type)
    }
}

// Delete removes the value at ref. A missing path/key is a no-op, not an
// error.
func (ref PropertyRef) Delete(r PropertyResolver) error {
    switch ref.Type {
    case TypeMsg:
        if r.Message != nil {
            deletePath(r.Message, ref.Path)
        }
        return nil
    case TypeFlow:
        if r.FlowContext == nil {
            return fmt.Errorf("typedvalue: flow context is not available")
        }
        r.FlowContext.Delete(ref.Path)
        return nil
    case TypeGlobal:
        if r.GlobalContext == nil {
            return fmt.Errorf("typedvalue: global context is not available")
        }
        r.GlobalContext.Delete(ref.Path)
        return nil
    default:
        return fmt.Errorf("typedvalue: unsupported property ref type %q", ref.Type)
    }
}

// setPath writes value at a dot-separated path within msg, creating
// map[string]interface{} values for any intermediate segment that doesn't
// exist yet or isn't itself a map (overwriting it - matching Node-RED's
// Change node, which does the same rather than erroring).
func setPath(msg map[string]interface{}, path string, value interface{}) {
    segments := strings.Split(path, ".")
    current := msg
    for i, segment := range segments {
        if i == len(segments)-1 {
            current[segment] = value
            return
        }
        next, ok := current[segment].(map[string]interface{})
        if !ok {
            next = make(map[string]interface{})
            current[segment] = next
        }
        current = next
    }
}

// deletePath removes the value at a dot-separated path within msg. A
// missing intermediate segment is a no-op.
func deletePath(msg map[string]interface{}, path string) {
    segments := strings.Split(path, ".")
    current := msg
    for i, segment := range segments {
        if i == len(segments)-1 {
            delete(current, segment)
            return
        }
        next, ok := current[segment].(map[string]interface{})
        if !ok {
            return
        }
        current = next
    }
}
