package sortnode

import (
    "context"
    "sync"
    "testing"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
    "github.com/GrimbiXcode/Go-RED/internal/typedvalue"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func partsMsg(id string, index, count int, payload interface{}) map[string]interface{} {
    return map[string]interface{}{
        "payload": payload,
        "parts":   map[string]interface{}{"id": id, "index": float64(index), "count": float64(count), "type": "array"},
    }
}

func collectSubmits() (context.Context, *[]map[string]interface{}) {
    var submitted []map[string]interface{}
    rt := registry.NewNodeRuntime("f1", "sort-1", "sort", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
        submitted = append(submitted, payload)
    }, nil)
    return registry.WithRuntime(context.Background(), rt), &submitted
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"descending": true}))
    assert.True(t, n.Descending)
}

func TestNode_ExecuteMulti_MissingPartsErrors(t *testing.T) {
    n := &Node{}
    _, err := n.ExecuteMulti(context.Background(), map[string]interface{}{"payload": "x"})
    assert.Error(t, err)
}

func TestNode_ExecuteMulti_IncompleteGroupSendsNothing(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 3, float64(5)))
    require.NoError(t, err)
    assert.Empty(t, outputs)
}

func TestNode_ExecuteMulti_SortsAscendingByDefault(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    ctx, submitted := collectSubmits()

    // Arrival order 3, 1, 2 (by index 0,1,2) but payload values are
    // unsorted (30, 10, 20) - sort node reorders by payload value, not
    // arrival/index order.
    _, err := n.ExecuteMulti(ctx, partsMsg("g1", 0, 3, float64(30)))
    require.NoError(t, err)
    _, err = n.ExecuteMulti(ctx, partsMsg("g1", 1, 3, float64(10)))
    require.NoError(t, err)
    outputs, err := n.ExecuteMulti(ctx, partsMsg("g1", 2, 3, float64(20)))
    require.NoError(t, err)

    require.Contains(t, outputs, "output")
    all := append([]map[string]interface{}{outputs["output"]}, *submitted...)
    require.Len(t, all, 3)
    assert.Equal(t, float64(10), all[0]["payload"])
    assert.Equal(t, float64(20), all[1]["payload"])
    assert.Equal(t, float64(30), all[2]["payload"])

    // parts.index should reflect the new sorted position, not the
    // original arrival index.
    for i, msg := range all {
        parts := msg["parts"].(map[string]interface{})
        assert.Equal(t, float64(i), parts["index"])
        assert.Equal(t, "g1", parts["id"], "group id is preserved so a downstream Join still works")
    }
}

func TestNode_ExecuteMulti_Descending(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}, Descending: true}
    ctx, submitted := collectSubmits()

    _, _ = n.ExecuteMulti(ctx, partsMsg("g1", 0, 2, float64(1)))
    outputs, err := n.ExecuteMulti(ctx, partsMsg("g1", 1, 2, float64(2)))
    require.NoError(t, err)

    assert.Equal(t, float64(2), outputs["output"]["payload"])
    require.Len(t, *submitted, 1)
    assert.Equal(t, float64(1), (*submitted)[0]["payload"])
}

func TestNode_ExecuteMulti_StringComparison(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    ctx, submitted := collectSubmits()

    _, _ = n.ExecuteMulti(ctx, partsMsg("g1", 0, 2, "banana"))
    outputs, err := n.ExecuteMulti(ctx, partsMsg("g1", 1, 2, "apple"))
    require.NoError(t, err)

    assert.Equal(t, "apple", outputs["output"]["payload"])
    require.Len(t, *submitted, 1)
    assert.Equal(t, "banana", (*submitted)[0]["payload"])
}

func TestNode_ExecuteMulti_GroupIsClearedAfterCompletion(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 1, float64(1)))
    n.mu.Lock()
    _, exists := n.groups["g1"]
    n.mu.Unlock()
    assert.False(t, exists)
}

func TestNode_ExecuteMulti_NoRuntimeStillReturnsFirst(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    _, _ = n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 2, float64(2)))
    outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 1, 2, float64(1)))
    require.NoError(t, err)
    assert.Equal(t, float64(1), outputs["output"]["payload"])
}

func TestNode_Execute_DelegatesToExecuteMulti(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    output, err := n.Execute(context.Background(), partsMsg("g1", 0, 1, float64(1)))
    require.NoError(t, err)
    assert.Equal(t, float64(1), output["payload"])
}

func TestNode_ExecuteMulti_ConcurrentAccessIsSafe(t *testing.T) {
    n := &Node{Property: typedvalue.PropertyRef{Type: typedvalue.TypeMsg, Path: "payload"}}
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        for p := 0; p < 5; p++ {
            wg.Add(1)
            go func(group string, index int) {
                defer wg.Done()
                _, _ = n.ExecuteMulti(context.Background(), partsMsg(group, index, 5, float64(index)))
            }(groupName(i), p)
        }
    }
    wg.Wait()
}

func groupName(i int) string {
    return "concurrent-group-" + string(rune('a'+i))
}
