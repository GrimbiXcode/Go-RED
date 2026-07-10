package join

import (
    "context"
    "sync"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func partsMsg(id string, index, count int, partType string, payload interface{}, extra map[string]interface{}) map[string]interface{} {
    parts := map[string]interface{}{"id": id, "index": float64(index), "count": float64(count), "type": partType}
    for k, v := range extra {
        parts[k] = v
    }
    return map[string]interface{}{"payload": payload, "parts": parts}
}

func TestNode_ExecuteMulti_MissingPartsErrors(t *testing.T) {
    n := &Node{}
    _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

func TestNode_ExecuteMulti_IncompleteGroupSendsNothing(t *testing.T) {
    n := &Node{}
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 3, "array", "a", nil))
    require.NoError(t, err)
    assert.Empty(t, outputs)
}

func TestNode_ExecuteMulti_ArrayReassembly(t *testing.T) {
    n := &Node{}
    _, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 3, "array", "a", nil))
    require.NoError(t, err)
    _, err = n.ExecuteMulti(context.Background(), partsMsg("g1", 1, 3, "array", "b", nil))
    require.NoError(t, err)
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 2, 3, "array", "c", nil))
    require.NoError(t, err)

    require.Contains(t, outputs, "output")
    assert.Equal(t, []interface{}{"a", "b", "c"}, outputs["output"]["payload"])
    _, hasParts := outputs["output"]["parts"]
    assert.False(t, hasParts, "the reassembled message should not carry parts metadata")
}

func TestNode_ExecuteMulti_ArrayReassembly_OutOfOrderArrival(t *testing.T) {
    n := &Node{}
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 2, 3, "array", "c", nil))
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 3, "array", "a", nil))
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 1, 3, "array", "b", nil))
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"a", "b", "c"}, outputs["output"]["payload"])
}

func TestNode_ExecuteMulti_StringReassembly(t *testing.T) {
    n := &Node{}
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 2, "string", "a", map[string]interface{}{"ch": ","}))
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 1, 2, "string", "b", map[string]interface{}{"ch": ","}))
    require.NoError(t, err)
    assert.Equal(t, "a,b", outputs["output"]["payload"])
}

func TestNode_ExecuteMulti_ObjectReassembly(t *testing.T) {
    n := &Node{}
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 2, "object", 1, map[string]interface{}{"key": "a"}))
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 1, 2, "object", 2, map[string]interface{}{"key": "b"}))
    require.NoError(t, err)
    assert.Equal(t, map[string]interface{}{"a": 1, "b": 2}, outputs["output"]["payload"])
}

func TestNode_ExecuteMulti_IndependentGroupsDoNotInterfere(t *testing.T) {
    n := &Node{}
    _, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 1, "array", "g1-a", nil))
    require.NoError(t, err)
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g2", 0, 1, "array", "g2-a", nil))
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"g2-a"}, outputs["output"]["payload"])
}

func TestNode_ExecuteMulti_GroupIsClearedAfterCompletion(t *testing.T) {
    n := &Node{}
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 1, "array", "a", nil))
    n.mu.Lock()
    _, exists := n.groups["g1"]
    n.mu.Unlock()
    assert.False(t, exists)
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), partsMsg("g1", 0, 1, "array", "a", nil))
    require.NoError(t, err)
    assert.Equal(t, []interface{}{"a"}, output["payload"])
}

func TestNode_ExecuteMulti_ConcurrentAccessIsSafe(t *testing.T) {
    n := &Node{}
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        for p := 0; p < 5; p++ {
            wg.Add(1)
            go func(group string, index int) {
                defer wg.Done()
                _, _ = n.ExecuteMulti(context.Background(), partsMsg(group, index, 5, "array", index, nil))
            }(groupName(i), p)
        }
    }
    wg.Wait()
}

func groupName(i int) string {
    return "concurrent-group-" + string(rune('a'+i))
}
