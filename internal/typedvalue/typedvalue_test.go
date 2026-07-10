package typedvalue

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// mapContext is a minimal ContextGetter backed by a plain map, used to test
// Resolve without depending on internal/engine.ContextStore.
type mapContext map[string]interface{}

func (m mapContext) Get(key string) (interface{}, bool) {
    v, ok := m[key]
    return v, ok
}

func TestValueResolve_Literals(t *testing.T) {
    t.Run("str returns the literal string", func(t *testing.T) {
        v, err := Value{Type: TypeString, Value: "hello"}.Resolve(Resolver{})
        require.NoError(t, err)
        assert.Equal(t, "hello", v)
    })

    t.Run("num parses a valid number", func(t *testing.T) {
        v, err := Value{Type: TypeNumber, Value: "42.5"}.Resolve(Resolver{})
        require.NoError(t, err)
        assert.Equal(t, 42.5, v)
    })

    t.Run("num errors on an invalid number", func(t *testing.T) {
        _, err := Value{Type: TypeNumber, Value: "not-a-number"}.Resolve(Resolver{})
        assert.Error(t, err)
    })

    t.Run("bool parses a valid bool", func(t *testing.T) {
        v, err := Value{Type: TypeBool, Value: "true"}.Resolve(Resolver{})
        require.NoError(t, err)
        assert.Equal(t, true, v)
    })

    t.Run("bool errors on an invalid bool", func(t *testing.T) {
        _, err := Value{Type: TypeBool, Value: "not-a-bool"}.Resolve(Resolver{})
        assert.Error(t, err)
    })

    t.Run("json parses a JSON object", func(t *testing.T) {
        v, err := Value{Type: TypeJSON, Value: `{"a":1}`}.Resolve(Resolver{})
        require.NoError(t, err)
        assert.Equal(t, map[string]interface{}{"a": float64(1)}, v)
    })

    t.Run("json errors on invalid JSON", func(t *testing.T) {
        _, err := Value{Type: TypeJSON, Value: `{not json`}.Resolve(Resolver{})
        assert.Error(t, err)
    })

    t.Run("env reads an environment variable", func(t *testing.T) {
        t.Setenv("TYPEDVALUE_TEST_VAR", "from-env")
        v, err := Value{Type: TypeEnv, Value: "TYPEDVALUE_TEST_VAR"}.Resolve(Resolver{})
        require.NoError(t, err)
        assert.Equal(t, "from-env", v)
    })

    t.Run("unsupported type errors", func(t *testing.T) {
        _, err := Value{Type: "jsonata", Value: "$now()"}.Resolve(Resolver{})
        assert.Error(t, err)
    })
}

func TestValueResolve_Msg(t *testing.T) {
    msg := map[string]interface{}{
        "payload": map[string]interface{}{
            "count": float64(3),
        },
        "topic": "sensors/temp",
    }

    t.Run("resolves a top-level field", func(t *testing.T) {
        v, err := Value{Type: TypeMsg, Value: "topic"}.Resolve(Resolver{Message: msg})
        require.NoError(t, err)
        assert.Equal(t, "sensors/temp", v)
    })

    t.Run("resolves a nested field via dot path", func(t *testing.T) {
        v, err := Value{Type: TypeMsg, Value: "payload.count"}.Resolve(Resolver{Message: msg})
        require.NoError(t, err)
        assert.Equal(t, float64(3), v)
    })

    t.Run("missing path resolves to nil, not an error", func(t *testing.T) {
        v, err := Value{Type: TypeMsg, Value: "payload.missing"}.Resolve(Resolver{Message: msg})
        require.NoError(t, err)
        assert.Nil(t, v)
    })

    t.Run("path through a non-map value resolves to nil", func(t *testing.T) {
        v, err := Value{Type: TypeMsg, Value: "topic.nested"}.Resolve(Resolver{Message: msg})
        require.NoError(t, err)
        assert.Nil(t, v)
    })
}

func TestValueResolve_FlowAndGlobal(t *testing.T) {
    t.Run("flow reads from the flow context", func(t *testing.T) {
        ctx := mapContext{"counter": 7}
        v, err := Value{Type: TypeFlow, Value: "counter"}.Resolve(Resolver{FlowContext: ctx})
        require.NoError(t, err)
        assert.Equal(t, 7, v)
    })

    t.Run("flow errors when no flow context is available", func(t *testing.T) {
        _, err := Value{Type: TypeFlow, Value: "counter"}.Resolve(Resolver{})
        assert.Error(t, err)
    })

    t.Run("global reads from the global context", func(t *testing.T) {
        ctx := mapContext{"apiKey": "secret"}
        v, err := Value{Type: TypeGlobal, Value: "apiKey"}.Resolve(Resolver{GlobalContext: ctx})
        require.NoError(t, err)
        assert.Equal(t, "secret", v)
    })

    t.Run("global errors when no global context is available", func(t *testing.T) {
        _, err := Value{Type: TypeGlobal, Value: "apiKey"}.Resolve(Resolver{})
        assert.Error(t, err)
    })

    t.Run("missing key resolves to nil, not an error", func(t *testing.T) {
        v, err := Value{Type: TypeFlow, Value: "missing"}.Resolve(Resolver{FlowContext: mapContext{}})
        require.NoError(t, err)
        assert.Nil(t, v)
    })
}
