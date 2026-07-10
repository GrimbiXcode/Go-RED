package split

import (
    "context"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"mode": "array", "separator": ";"}))
    assert.Equal(t, "array", n.Mode)
    assert.Equal(t, ";", n.Separator)
}

func TestNode_SetConfig_Defaults(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, n.Property)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{Mode: "bogus"}).Validate())
    assert.NoError(t, (&Node{Mode: "array"}).Validate())
}

// collectSubmits wraps a NodeRuntime whose SubmitToNode calls are recorded,
// used to verify parts 2..N of a split are dispatched via SubmitToNode.
func collectSubmits() (context.Context, *[]map[string]interface{}) {
    var submitted []map[string]interface{}
    rt := registry.NewNodeRuntime("f1", "split-1", "split", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
        submitted = append(submitted, payload)
    }, nil)
    return registry.WithRuntime(context.Background(), rt), &submitted
}

func TestNode_ExecuteMulti_ArrayMode(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "array"}
    ctx, submitted := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": []interface{}{"a", "b", "c"}})
    require.NoError(t, err)

    require.Contains(t, outputs, "output")
    first := outputs["output"]
    assert.Equal(t, "a", first["payload"])
    firstParts := first["parts"].(map[string]interface{})
    assert.Equal(t, float64(0), firstParts["index"])
    assert.Equal(t, float64(3), firstParts["count"])
    assert.Equal(t, "array", firstParts["type"])

    require.Len(t, *submitted, 2)
    assert.Equal(t, "b", (*submitted)[0]["payload"])
    assert.Equal(t, "c", (*submitted)[1]["payload"])
    secondParts := (*submitted)[0]["parts"].(map[string]interface{})
    assert.Equal(t, float64(1), secondParts["index"])
    assert.Equal(t, firstParts["id"], secondParts["id"], "all parts share the same group id")
}

func TestNode_ExecuteMulti_StringMode(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "string", Separator: ","}
    ctx, submitted := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": "a,b,c"})
    require.NoError(t, err)
    assert.Equal(t, "a", outputs["output"]["payload"])
    require.Len(t, *submitted, 2)
    assert.Equal(t, "b", (*submitted)[0]["payload"])
    assert.Equal(t, "c", (*submitted)[1]["payload"])
    parts := outputs["output"]["parts"].(map[string]interface{})
    assert.Equal(t, ",", parts["ch"])
}

func TestNode_ExecuteMulti_StringMode_DefaultSeparator(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "string"}
    ctx, submitted := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": "a\nb"})
    require.NoError(t, err)
    assert.Equal(t, "a", outputs["output"]["payload"])
    require.Len(t, *submitted, 1)
    assert.Equal(t, "b", (*submitted)[0]["payload"])
}

func TestNode_ExecuteMulti_ObjectMode(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "object"}
    ctx, submitted := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": map[string]interface{}{"a": 1, "b": 2}})
    require.NoError(t, err)

    all := append([]map[string]interface{}{outputs["output"]}, *submitted...)
    require.Len(t, all, 2)
    assert.Equal(t, "a", all[0]["topic"])
    assert.Equal(t, 1, all[0]["payload"])
    assert.Equal(t, "b", all[1]["topic"])
    assert.Equal(t, 2, all[1]["payload"])
    parts0 := all[0]["parts"].(map[string]interface{})
    assert.Equal(t, "a", parts0["key"])
}

func TestNode_ExecuteMulti_AutoDetectMode(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    ctx, _ := collectSubmits()

    outputs, err := n.ExecuteMulti(ctx, map[string]interface{}{"payload": []interface{}{"x"}})
    require.NoError(t, err)
    parts := outputs["output"]["parts"].(map[string]interface{})
    assert.Equal(t, "array", parts["type"])
}

func TestNode_ExecuteMulti_AutoDetectFailsForUnsupportedType(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": float64(1)})
    assert.Error(t, err)
}

func TestNode_ExecuteMulti_EmptyArraySendsNothing(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "array"}
    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": []interface{}{}})
    require.NoError(t, err)
    assert.Empty(t, outputs)
}

func TestNode_ExecuteMulti_MissingPropertyErrors(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "array"}
    _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{})
    assert.Error(t, err)
}

func TestNode_ExecuteMulti_TypeMismatchErrors(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "array"}
    _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "not an array"})
    assert.Error(t, err)
}

func TestNode_ExecuteMulti_NoRuntimeStillReturnsFirstPart(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "array"}
    outputs, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": []interface{}{"only-first-without-runtime", "b"}})
    require.NoError(t, err)
    assert.Equal(t, "only-first-without-runtime", outputs["output"]["payload"])
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Mode: "array"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": []interface{}{"a"}})
    require.NoError(t, err)
    assert.Equal(t, "a", output["payload"])
}
