package linkout

import (
    "context"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"links": []interface{}{"link-in-1", "link-in-2"}}))
    assert.Equal(t, []string{"link-in-1", "link-in-2"}, n.Links)
    assert.Equal(t, map[string]interface{}{"links": []interface{}{"link-in-1", "link-in-2"}}, n.GetConfig())
}

func TestNode_ExecuteMulti(t *testing.T) {
    t.Run("sends on no output port", func(t *testing.T) {
        n := &Node{Links: []string{"target"}}
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": 1})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })

    t.Run("delivers to every configured target via NodeRuntime.SubmitToNode", func(t *testing.T) {
        n := &Node{Links: []string{"target-a", "target-b"}}

        var delivered []string
        rt := registry.NewNodeRuntime("flow-1", "link-out-1", "link out", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
            delivered = append(delivered, nodeID)
            assert.Equal(t, "hello", payload["payload"])
        }, nil)
        ctx := registry.WithRuntime(context.Background(), rt)

        _, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": "hello"})
        require.NoError(t, err)

        assert.ElementsMatch(t, []string{"target-a", "target-b"}, delivered)
    })

    t.Run("is a no-op when no NodeRuntime is available", func(t *testing.T) {
        n := &Node{Links: []string{"target"}}
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })
}

func TestNode_Execute(t *testing.T) {
    t.Run("delegates to ExecuteMulti and returns an empty payload", func(t *testing.T) {
        n := &Node{}
        output, err := n.Execute(context.Background(), map[string]interface{}{})
        require.NoError(t, err)
        assert.Empty(t, output)
    })
}
