package base

import (
	"context"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/typedvalue"
)

// Context returns the context.Context behind the interface{} ctx the
// engine passes to Execute/ExecuteMulti, or context.Background() when ctx
// is nil or not a context (a direct unit-test call). Use it where the
// node only needs cancellation/deadline; a node that must know whether a
// real context was supplied keeps its own type assertion.
func Context(ctx interface{}) context.Context {
	if c, ok := ctx.(context.Context); ok && c != nil {
		return c
	}
	return context.Background()
}

// Runtime returns the registry.NodeRuntime the engine embedded into ctx,
// with ok == false when ctx is not a context.Context or carries no
// runtime (a node executed outside the engine).
func Runtime(ctx interface{}) (*registry.NodeRuntime, bool) {
	c, ok := ctx.(context.Context)
	if !ok || c == nil {
		return nil, false
	}
	return registry.RuntimeFromContext(c)
}

// Resolvers builds the typedvalue resolvers for msg: both read the
// message directly and, when ctx carries a NodeRuntime, this flow's and
// the global context store. Without a runtime (or with nil stores) the
// flow/global fields stay nil, and resolving a "flow"/"global" typed
// value reports that the context is unavailable.
func Resolvers(ctx interface{}, msg map[string]interface{}) (typedvalue.Resolver, typedvalue.PropertyResolver) {
	valueResolver := typedvalue.Resolver{Message: msg}
	propResolver := typedvalue.PropertyResolver{Message: msg}

	rt, ok := Runtime(ctx)
	if !ok {
		return valueResolver, propResolver
	}
	// Assign only non-nil stores: a nil *ContextStore stored in the
	// interface field would be non-nil and get dereferenced.
	if rt.FlowContext != nil {
		valueResolver.FlowContext = rt.FlowContext
		propResolver.FlowContext = rt.FlowContext
	}
	if rt.GlobalContext != nil {
		valueResolver.GlobalContext = rt.GlobalContext
		propResolver.GlobalContext = rt.GlobalContext
	}
	return valueResolver, propResolver
}

// Resolver is Resolvers for a node that only reads typed values and
// never writes a PropertyRef.
func Resolver(ctx interface{}, msg map[string]interface{}) typedvalue.Resolver {
	r, _ := Resolvers(ctx, msg)
	return r
}
