package rangenode

import (
    "context"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func defaultNode() *Node {
    return &Node{
        Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"},
        MinIn:    0, MaxIn: 100, MinOut: 0, MaxOut: 10,
        Action: "scale",
    }
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "minin": float64(0), "maxin": float64(255), "minout": float64(0), "maxout": float64(1),
        "action": "clamp", "round": true,
    }))
    assert.Equal(t, float64(255), n.MaxIn)
    assert.Equal(t, "clamp", n.Action)
    assert.True(t, n.Round)

    cfg := n.GetConfig()
    assert.Equal(t, "clamp", cfg["action"])
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, "scale", n.Action)
    assert.Equal(t, float64(100), n.MaxIn)
}

func TestNode_Validate(t *testing.T) {
    n := &Node{Action: "bogus"}
    assert.Error(t, n.Validate())

    n.Action = "roll"
    assert.NoError(t, n.Validate())
}

func TestNode_Execute_Scale(t *testing.T) {
    n := defaultNode()
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(50)})
    require.NoError(t, err)
    assert.InDelta(t, 5.0, output["payload"], 0.0001)
}

func TestNode_Execute_ScaleOutOfRangeIsUnclamped(t *testing.T) {
    n := defaultNode()
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(200)})
    require.NoError(t, err)
    assert.InDelta(t, 20.0, output["payload"], 0.0001)
}

func TestNode_Execute_Clamp(t *testing.T) {
    n := defaultNode()
    n.Action = "clamp"
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(200)})
    require.NoError(t, err)
    assert.InDelta(t, 10.0, output["payload"], 0.0001)

    output, err = n.Execute(context.Background(), map[string]interface{}{"payload": float64(-50)})
    require.NoError(t, err)
    assert.InDelta(t, 0.0, output["payload"], 0.0001)
}

func TestNode_Execute_Roll(t *testing.T) {
    n := defaultNode()
    n.Action = "roll"
    // 110 -> scaled to 11 -> wraps into [0,10) -> 1
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(110)})
    require.NoError(t, err)
    assert.InDelta(t, 1.0, output["payload"], 0.0001)

    t.Run("negative input wraps into range too", func(t *testing.T) {
        output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(-10)})
        require.NoError(t, err)
        v := output["payload"].(float64)
        assert.GreaterOrEqual(t, v, 0.0)
        assert.Less(t, v, 10.0)
    })
}

func TestNode_Execute_Round(t *testing.T) {
    n := defaultNode()
    n.Round = true
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(53)})
    require.NoError(t, err)
    assert.Equal(t, 5.0, output["payload"])
}

func TestNode_Execute_MissingPropertyIsNoop(t *testing.T) {
    n := defaultNode()
    output, err := n.Execute(context.Background(), map[string]interface{}{})
    require.NoError(t, err)
    _, exists := output["payload"]
    assert.False(t, exists)
}

func TestNode_Execute_NonNumericPropertyErrors(t *testing.T) {
    n := defaultNode()
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "not a number"})
    assert.Error(t, err)
}

func TestScaleClampRoll_Helpers(t *testing.T) {
    t.Run("scale with equal min/max input avoids divide by zero", func(t *testing.T) {
        assert.Equal(t, 5.0, scale(3, 10, 10, 5, 5))
    })

    t.Run("clamp handles inverted bounds", func(t *testing.T) {
        // bounds (10, 5) normalize to [5,10]; 20 exceeds the upper bound.
        assert.Equal(t, 10.0, clamp(20, 10, 5))
    })

    t.Run("roll with zero span returns minOut", func(t *testing.T) {
        assert.Equal(t, 5.0, roll(99, 5, 5))
    })
}
