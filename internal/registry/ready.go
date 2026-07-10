package registry

import "context"

type readyContextKey struct{}

// WithReady returns a copy of ctx carrying ready, retrievable via
// SignalReady. The engine embeds this into the context passed to an
// EagerlyReadyNode's Start, alongside the NodeRuntime from WithRuntime -
// see EagerlyReadyNode's doc comment in registry.go for why this exists.
func WithReady(ctx context.Context, ready func()) context.Context {
    return context.WithValue(ctx, readyContextKey{}, ready)
}

// SignalReady invokes the ready callback embedded by WithReady, if any.
// Safe to call even if none was embedded (e.g. a unit test calling Start
// directly with a plain context.Background()) and safe to call more than
// once - the engine's own callback is idempotent, and a node need not
// track whether it already called this itself.
func SignalReady(ctx context.Context) {
    if ready, ok := ctx.Value(readyContextKey{}).(func()); ok && ready != nil {
        ready()
    }
}
