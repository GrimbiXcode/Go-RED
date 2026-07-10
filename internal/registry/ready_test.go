package registry

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestSignalReady_CallsTheEmbeddedCallback(t *testing.T) {
    called := false
    ctx := WithReady(context.Background(), func() { called = true })

    SignalReady(ctx)

    assert.True(t, called)
}

func TestSignalReady_NoCallbackEmbedded_IsANoOp(t *testing.T) {
    assert.NotPanics(t, func() {
        SignalReady(context.Background())
    })
}

func TestSignalReady_SafeToCallMultipleTimes(t *testing.T) {
    calls := 0
    ctx := WithReady(context.Background(), func() { calls++ })

    SignalReady(ctx)
    SignalReady(ctx)
    SignalReady(ctx)

    assert.Equal(t, 3, calls, "SignalReady itself doesn't dedupe - callers that need idempotency (like the engine) wrap their own callback in sync.Once")
}
