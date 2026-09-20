package base

import (
	"context"
	"testing"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/typedvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext(t *testing.T) {
	t.Run("returns the context as is", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		assert.Equal(t, ctx, Context(ctx))
	})
	t.Run("nil yields Background", func(t *testing.T) {
		assert.Equal(t, context.Background(), Context(nil))
	})
	t.Run("a typed nil context yields Background", func(t *testing.T) {
		var c context.Context
		assert.Equal(t, context.Background(), Context(c))
	})
	t.Run("a non-context yields Background", func(t *testing.T) {
		assert.Equal(t, context.Background(), Context("not a context"))
		assert.Equal(t, context.Background(), Context(42))
	})
}

func TestRuntime(t *testing.T) {
	rt := registry.NewNodeRuntime("f1", "n1", "test", nil, nil, nil, nil, nil)

	t.Run("finds the embedded runtime", func(t *testing.T) {
		got, ok := Runtime(registry.WithRuntime(context.Background(), rt))
		require.True(t, ok)
		assert.Same(t, rt, got)
	})
	t.Run("plain context has no runtime", func(t *testing.T) {
		got, ok := Runtime(context.Background())
		assert.False(t, ok)
		assert.Nil(t, got)
	})
	t.Run("nil has no runtime", func(t *testing.T) {
		got, ok := Runtime(nil)
		assert.False(t, ok)
		assert.Nil(t, got)
	})
	t.Run("non-context has no runtime", func(t *testing.T) {
		got, ok := Runtime("ctx")
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestResolvers(t *testing.T) {
	msg := map[string]interface{}{"payload": "hi"}

	t.Run("without a runtime only the message is available", func(t *testing.T) {
		vr, pr := Resolvers(context.Background(), msg)
		assert.Equal(t, msg, vr.Message)
		assert.Nil(t, vr.FlowContext)
		assert.Nil(t, vr.GlobalContext)
		assert.Equal(t, msg, pr.Message)
		assert.Nil(t, pr.FlowContext)
		assert.Nil(t, pr.GlobalContext)

		_, err := typedvalue.Value{Type: typedvalue.TypeFlow, Value: "k"}.Resolve(vr)
		assert.Error(t, err, "flow context must be reported as unavailable")
	})

	t.Run("with a runtime the stores are wired in", func(t *testing.T) {
		flow := registry.NewContextStore()
		global := registry.NewContextStore()
		flow.Set("k", "flow-value")
		global.Set("k", "global-value")
		rt := registry.NewNodeRuntime("f1", "n1", "test", flow, global, nil, nil, nil)
		ctx := registry.WithRuntime(context.Background(), rt)

		vr, pr := Resolvers(ctx, msg)
		got, err := typedvalue.Value{Type: typedvalue.TypeFlow, Value: "k"}.Resolve(vr)
		require.NoError(t, err)
		assert.Equal(t, "flow-value", got)
		got, err = typedvalue.Value{Type: typedvalue.TypeGlobal, Value: "k"}.Resolve(vr)
		require.NoError(t, err)
		assert.Equal(t, "global-value", got)

		require.NoError(t, typedvalue.PropertyRef{Type: typedvalue.TypeFlow, Path: "w"}.Set(pr, 1))
		v, ok := flow.Get("w")
		require.True(t, ok)
		assert.Equal(t, 1, v)
	})

	t.Run("nil stores on the runtime stay nil interfaces", func(t *testing.T) {
		rt := registry.NewNodeRuntime("f1", "n1", "test", nil, nil, nil, nil, nil)
		vr, pr := Resolvers(registry.WithRuntime(context.Background(), rt), msg)
		assert.Nil(t, vr.FlowContext)
		assert.Nil(t, vr.GlobalContext)
		assert.Nil(t, pr.FlowContext)
		assert.Nil(t, pr.GlobalContext)
		assert.Error(t, typedvalue.PropertyRef{Type: typedvalue.TypeGlobal, Path: "w"}.Set(pr, 1))
	})

	t.Run("Resolver returns the value resolver alone", func(t *testing.T) {
		flow := registry.NewContextStore()
		flow.Set("k", "v")
		rt := registry.NewNodeRuntime("f1", "n1", "test", flow, nil, nil, nil, nil)
		r := Resolver(registry.WithRuntime(context.Background(), rt), msg)
		got, err := typedvalue.Value{Type: typedvalue.TypeFlow, Value: "k"}.Resolve(r)
		require.NoError(t, err)
		assert.Equal(t, "v", got)
		assert.Nil(t, r.GlobalContext)
	})
}
