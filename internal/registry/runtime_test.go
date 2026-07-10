package registry

import (
    "context"
    "errors"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRuntimeContextRoundTrip(t *testing.T) {
    t.Run("RuntimeFromContext returns false when nothing was embedded", func(t *testing.T) {
        _, ok := RuntimeFromContext(context.Background())
        assert.False(t, ok)
    })

    t.Run("WithRuntime/RuntimeFromContext round-trips the same *NodeRuntime", func(t *testing.T) {
        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, nil, nil, nil)
        ctx := WithRuntime(context.Background(), rt)

        got, ok := RuntimeFromContext(ctx)
        require.True(t, ok)
        assert.Same(t, rt, got)
    })
}

func TestNodeRuntime_ReportingIsNilSafe(t *testing.T) {
    t.Run("a nil *NodeRuntime does not panic when reporting or subscribing", func(t *testing.T) {
        var rt *NodeRuntime
        assert.NotPanics(t, func() {
            rt.ReportStatus("ok", "detail")
            rt.ReportError(errors.New("boom"))
            rt.OnError(func(NodeErrorEvent) {})
            rt.OnStatus(func(NodeStatusEvent) {})
            rt.OnComplete(func(NodeCompleteEvent) {})
            rt.SubmitToNode("other", nil)
        })
    })

    t.Run("a NodeRuntime with no EventBus does not panic when reporting or subscribing", func(t *testing.T) {
        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, nil, nil, nil)
        assert.NotPanics(t, func() {
            rt.ReportStatus("ok", "detail")
            rt.ReportError(errors.New("boom"))
            rt.OnError(func(NodeErrorEvent) {})
            rt.SubmitToNode("other", nil)
        })
    })

    t.Run("ReportError with a nil error does not publish", func(t *testing.T) {
        bus := NewEventBus()
        called := false
        bus.OnError(func(evt NodeErrorEvent) { called = true })

        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, bus, nil, nil)
        rt.ReportError(nil)

        assert.False(t, called)
    })

    t.Run("ReportStatus/ReportError publish through to a real EventBus", func(t *testing.T) {
        bus := NewEventBus()
        var gotStatus NodeStatusEvent
        var gotErr NodeErrorEvent
        bus.OnStatus(func(evt NodeStatusEvent) { gotStatus = evt })
        bus.OnError(func(evt NodeErrorEvent) { gotErr = evt })

        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, bus, nil, nil)
        rt.ReportStatus("connecting", "attempt 1")
        rt.ReportError(errors.New("boom"))

        assert.Equal(t, "connecting", gotStatus.Status)
        assert.Equal(t, "attempt 1", gotStatus.Detail)
        assert.Equal(t, "n1", gotStatus.NodeID)

        assert.EqualError(t, gotErr.Err, "boom")
        assert.Equal(t, "t1", gotErr.NodeType)
    })

    t.Run("OnError/OnStatus/OnComplete subscribe through to a real EventBus", func(t *testing.T) {
        bus := NewEventBus()
        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, bus, nil, nil)

        var errCalled, statusCalled, completeCalled bool
        rt.OnError(func(NodeErrorEvent) { errCalled = true })
        rt.OnStatus(func(NodeStatusEvent) { statusCalled = true })
        rt.OnComplete(func(NodeCompleteEvent) { completeCalled = true })

        bus.PublishError(NodeErrorEvent{})
        bus.PublishStatus(NodeStatusEvent{})
        bus.PublishComplete(NodeCompleteEvent{})

        assert.True(t, errCalled)
        assert.True(t, statusCalled)
        assert.True(t, completeCalled)
    })

    t.Run("SubmitToNode calls through to the submit function", func(t *testing.T) {
        var gotNodeID string
        var gotPayload map[string]interface{}
        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
            gotNodeID = nodeID
            gotPayload = payload
        }, nil)

        rt.SubmitToNode("target", map[string]interface{}{"a": 1})

        assert.Equal(t, "target", gotNodeID)
        assert.Equal(t, map[string]interface{}{"a": 1}, gotPayload)
    })

    t.Run("GetNode calls through to the getNode function", func(t *testing.T) {
        sentinel := passthroughExecutor{}
        rt := NewNodeRuntime("f1", "n1", "t1", nil, nil, nil, nil, func(nodeID string) (NodeExecutor, bool) {
            if nodeID == "config-1" {
                return sentinel, true
            }
            return nil, false
        })

        got, ok := rt.GetNode("config-1")
        assert.True(t, ok)
        assert.Equal(t, sentinel, got)

        _, ok = rt.GetNode("missing")
        assert.False(t, ok)
    })

    t.Run("a nil *NodeRuntime's GetNode does not panic", func(t *testing.T) {
        var rt *NodeRuntime
        _, ok := rt.GetNode("anything")
        assert.False(t, ok)
    })
}

type passthroughExecutor struct{}

func (passthroughExecutor) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return input, nil
}
func (passthroughExecutor) Validate() error                             { return nil }
func (passthroughExecutor) GetConfig() map[string]interface{}           { return nil }
func (passthroughExecutor) SetConfig(config map[string]interface{}) error { return nil }
