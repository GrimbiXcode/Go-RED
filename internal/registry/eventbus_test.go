package registry

import (
    "errors"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestEventBus(t *testing.T) {
    t.Run("PublishError calls every registered error handler", func(t *testing.T) {
        bus := NewEventBus()
        var calls []NodeErrorEvent
        bus.OnError(func(evt NodeErrorEvent) { calls = append(calls, evt) })
        bus.OnError(func(evt NodeErrorEvent) { calls = append(calls, evt) })

        bus.PublishError(NodeErrorEvent{NodeID: "n1", Err: errors.New("boom")})

        assert.Len(t, calls, 2)
        assert.Equal(t, "n1", calls[0].NodeID)
    })

    t.Run("PublishStatus calls every registered status handler", func(t *testing.T) {
        bus := NewEventBus()
        var calls []NodeStatusEvent
        bus.OnStatus(func(evt NodeStatusEvent) { calls = append(calls, evt) })

        bus.PublishStatus(NodeStatusEvent{NodeID: "n1", Status: "connected"})

        assert.Len(t, calls, 1)
        assert.Equal(t, "connected", calls[0].Status)
    })

    t.Run("PublishComplete calls every registered complete handler", func(t *testing.T) {
        bus := NewEventBus()
        var calls []NodeCompleteEvent
        bus.OnComplete(func(evt NodeCompleteEvent) { calls = append(calls, evt) })

        bus.PublishComplete(NodeCompleteEvent{NodeID: "n1", Payload: map[string]interface{}{"a": 1}})

        assert.Len(t, calls, 1)
        assert.Equal(t, map[string]interface{}{"a": 1}, calls[0].Payload)
    })

    t.Run("PublishError with no handlers registered does not panic", func(t *testing.T) {
        bus := NewEventBus()
        assert.NotPanics(t, func() { bus.PublishError(NodeErrorEvent{}) })
    })

    t.Run("error/status/complete handlers are independent", func(t *testing.T) {
        bus := NewEventBus()
        errorCalls, statusCalls, completeCalls := 0, 0, 0
        bus.OnError(func(evt NodeErrorEvent) { errorCalls++ })
        bus.OnStatus(func(evt NodeStatusEvent) { statusCalls++ })
        bus.OnComplete(func(evt NodeCompleteEvent) { completeCalls++ })

        bus.PublishError(NodeErrorEvent{})
        assert.Equal(t, 1, errorCalls)
        assert.Equal(t, 0, statusCalls)
        assert.Equal(t, 0, completeCalls)

        bus.PublishStatus(NodeStatusEvent{})
        assert.Equal(t, 1, errorCalls)
        assert.Equal(t, 1, statusCalls)
        assert.Equal(t, 0, completeCalls)

        bus.PublishComplete(NodeCompleteEvent{})
        assert.Equal(t, 1, errorCalls)
        assert.Equal(t, 1, statusCalls)
        assert.Equal(t, 1, completeCalls)
    })
}
