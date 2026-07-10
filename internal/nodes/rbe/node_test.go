package rbe

import (
    "context"
    "sync"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func newRBENode() *Node {
    return &Node{Mode: "rbe", Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "mode": "deadband", "gap": float64(5), "separateTopics": true,
    }))
    assert.Equal(t, "deadband", n.Mode)
    assert.Equal(t, float64(5), n.Gap)
    assert.True(t, n.SeparateTopics)

    cfg := n.GetConfig()
    assert.Equal(t, "deadband", cfg["mode"])
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "rbe", n.Mode)
    assert.Equal(t, typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, n.Property)
}

func TestNode_Validate(t *testing.T) {
    n := &Node{Mode: "bogus"}
    assert.Error(t, n.Validate())
    n.Mode = "deadband"
    assert.NoError(t, n.Validate())
}

func TestNode_ExecuteMulti_RBEMode(t *testing.T) {
    n := newRBENode()

    t.Run("first message always passes", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(1)})
        require.NoError(t, err)
        assert.Contains(t, outputs, "output")
    })

    t.Run("same value again is suppressed", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(1)})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })

    t.Run("a changed value passes again", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(2)})
        require.NoError(t, err)
        assert.Contains(t, outputs, "output")
    })

    t.Run("missing property is suppressed", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })
}

func TestNode_ExecuteMulti_DeadbandMode(t *testing.T) {
    n := &Node{Mode: "deadband", Gap: 5, Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}

    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(10)})
    require.NoError(t, err)
    assert.Contains(t, outputs, "output")

    t.Run("a small change under the gap is suppressed", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(12)})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })

    t.Run("a change meeting the gap passes", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(15)})
        require.NoError(t, err)
        assert.Contains(t, outputs, "output")
    })

    t.Run("non-numeric property errors", func(t *testing.T) {
        _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "not a number"})
        assert.Error(t, err)
    })
}

func TestNode_ExecuteMulti_SeparateTopics(t *testing.T) {
    n := &Node{Mode: "rbe", SeparateTopics: true, Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}

    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(1), "topic": "a"})
    require.NoError(t, err)
    assert.Contains(t, outputs, "output")

    t.Run("a different topic with the same value still passes", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(1), "topic": "b"})
        require.NoError(t, err)
        assert.Contains(t, outputs, "output")
    })

    t.Run("repeating the same topic and value is suppressed", func(t *testing.T) {
        outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(1), "topic": "a"})
        require.NoError(t, err)
        assert.Empty(t, outputs)
    })
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    n := newRBENode()
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(1)})
    require.NoError(t, err)
    assert.Equal(t, float64(1), output["payload"])
}

func TestNode_ExecuteMulti_ConcurrentAccessIsSafe(t *testing.T) {
    n := newRBENode()
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            _, _ = n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(i % 5)})
        }(i)
    }
    wg.Wait()
}
