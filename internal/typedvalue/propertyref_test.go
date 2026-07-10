package typedvalue

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// mapContextAccessor is a minimal ContextAccessor backed by a plain map,
// used to test PropertyRef without depending on registry.ContextStore.
type mapContextAccessor map[string]interface{}

func (m mapContextAccessor) Get(key string) (interface{}, bool) {
    v, ok := m[key]
    return v, ok
}
func (m mapContextAccessor) Set(key string, value interface{}) { m[key] = value }
func (m mapContextAccessor) Delete(key string)                 { delete(m, key) }

func TestPropertyRef_Get(t *testing.T) {
    msg := map[string]interface{}{"payload": map[string]interface{}{"count": float64(3)}}
    flow := mapContextAccessor{"counter": 7}

    t.Run("msg: resolves a nested path", func(t *testing.T) {
        v, ok := PropertyRef{Type: TypeMsg, Path: "payload.count"}.Get(PropertyResolver{Message: msg})
        assert.True(t, ok)
        assert.Equal(t, float64(3), v)
    })

    t.Run("msg: missing path resolves to (nil, false)", func(t *testing.T) {
        v, ok := PropertyRef{Type: TypeMsg, Path: "payload.missing"}.Get(PropertyResolver{Message: msg})
        assert.False(t, ok)
        assert.Nil(t, v)
    })

    t.Run("flow: resolves an existing key", func(t *testing.T) {
        v, ok := PropertyRef{Type: TypeFlow, Path: "counter"}.Get(PropertyResolver{FlowContext: flow})
        assert.True(t, ok)
        assert.Equal(t, 7, v)
    })

    t.Run("flow: nil FlowContext resolves to (nil, false)", func(t *testing.T) {
        v, ok := PropertyRef{Type: TypeFlow, Path: "counter"}.Get(PropertyResolver{})
        assert.False(t, ok)
        assert.Nil(t, v)
    })
}

func TestPropertyRef_Set(t *testing.T) {
    t.Run("msg: overwrites an existing top-level key", func(t *testing.T) {
        msg := map[string]interface{}{"payload": "old"}
        require.NoError(t, (PropertyRef{Type: TypeMsg, Path: "payload"}).Set(PropertyResolver{Message: msg}, "new"))
        assert.Equal(t, "new", msg["payload"])
    })

    t.Run("msg: creates intermediate maps for a nested path", func(t *testing.T) {
        msg := map[string]interface{}{}
        require.NoError(t, (PropertyRef{Type: TypeMsg, Path: "a.b.c"}).Set(PropertyResolver{Message: msg}, 42))

        a, ok := msg["a"].(map[string]interface{})
        require.True(t, ok)
        b, ok := a["b"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, 42, b["c"])
    })

    t.Run("msg: overwrites a non-map intermediate segment instead of erroring", func(t *testing.T) {
        msg := map[string]interface{}{"a": "not a map"}
        require.NoError(t, (PropertyRef{Type: TypeMsg, Path: "a.b"}).Set(PropertyResolver{Message: msg}, 1))

        a, ok := msg["a"].(map[string]interface{})
        require.True(t, ok)
        assert.Equal(t, 1, a["b"])
    })

    t.Run("flow: writes through to the ContextAccessor", func(t *testing.T) {
        flow := mapContextAccessor{}
        require.NoError(t, (PropertyRef{Type: TypeFlow, Path: "counter"}).Set(PropertyResolver{FlowContext: flow}, 5))
        assert.Equal(t, 5, flow["counter"])
    })

    t.Run("flow: nil FlowContext errors", func(t *testing.T) {
        err := (PropertyRef{Type: TypeFlow, Path: "counter"}).Set(PropertyResolver{}, 5)
        assert.Error(t, err)
    })

    t.Run("msg: nil message errors", func(t *testing.T) {
        err := (PropertyRef{Type: TypeMsg, Path: "payload"}).Set(PropertyResolver{}, 5)
        assert.Error(t, err)
    })
}

func TestPropertyRef_Delete(t *testing.T) {
    t.Run("msg: removes a top-level key", func(t *testing.T) {
        msg := map[string]interface{}{"payload": "x", "topic": "y"}
        require.NoError(t, (PropertyRef{Type: TypeMsg, Path: "payload"}).Delete(PropertyResolver{Message: msg}))
        _, exists := msg["payload"]
        assert.False(t, exists)
        assert.Equal(t, "y", msg["topic"])
    })

    t.Run("msg: removes a nested key", func(t *testing.T) {
        msg := map[string]interface{}{"a": map[string]interface{}{"b": 1, "c": 2}}
        require.NoError(t, (PropertyRef{Type: TypeMsg, Path: "a.b"}).Delete(PropertyResolver{Message: msg}))

        a := msg["a"].(map[string]interface{})
        _, exists := a["b"]
        assert.False(t, exists)
        assert.Equal(t, 2, a["c"])
    })

    t.Run("msg: missing path is a no-op, not an error", func(t *testing.T) {
        msg := map[string]interface{}{}
        assert.NoError(t, (PropertyRef{Type: TypeMsg, Path: "a.b.c"}).Delete(PropertyResolver{Message: msg}))
    })

    t.Run("flow: deletes through to the ContextAccessor", func(t *testing.T) {
        flow := mapContextAccessor{"counter": 1}
        require.NoError(t, (PropertyRef{Type: TypeFlow, Path: "counter"}).Delete(PropertyResolver{FlowContext: flow}))
        _, ok := flow.Get("counter")
        assert.False(t, ok)
    })
}
