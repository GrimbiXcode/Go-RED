package csvnode

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "hasHeaderRow": true, "columns": []interface{}{"a", "b"}, "delimiter": ";",
    }))
    assert.True(t, n.HasHeaderRow)
    assert.Equal(t, []string{"a", "b"}, n.Columns)
    assert.Equal(t, ";", n.Delimiter)
}

func TestNode_Execute_ParseWithoutHeaderRow(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a,b\n1,2\n"})
    require.NoError(t, err)
    rows, ok := output["payload"].([]interface{})
    require.True(t, ok)
    require.Len(t, rows, 2)
    assert.Equal(t, []interface{}{"a", "b"}, rows[0])
    assert.Equal(t, []interface{}{"1", "2"}, rows[1])
}

func TestNode_Execute_ParseWithHeaderRow(t *testing.T) {
    n := &Node{HasHeaderRow: true}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "name,age\nAda,30\nGrace,85\n"})
    require.NoError(t, err)
    rows, ok := output["payload"].([]interface{})
    require.True(t, ok)
    require.Len(t, rows, 2)

    row0, ok := rows[0].(map[string]interface{})
    require.True(t, ok)
    assert.Equal(t, "Ada", row0["name"])
    assert.Equal(t, "30", row0["age"])
}

func TestNode_Execute_ParseWithExplicitColumns(t *testing.T) {
    n := &Node{Columns: []string{"x", "y"}}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "1,2\n3,4\n"})
    require.NoError(t, err)
    rows := output["payload"].([]interface{})
    require.Len(t, rows, 2)
    row0 := rows[0].(map[string]interface{})
    assert.Equal(t, "1", row0["x"])
    assert.Equal(t, "2", row0["y"])
}

func TestNode_Execute_SerializeArrayOfArrays(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": []interface{}{
            []interface{}{"a", "b"},
            []interface{}{"1", "2"},
        },
    })
    require.NoError(t, err)
    assert.Equal(t, "a,b\n1,2\n", output["payload"])
}

func TestNode_Execute_SerializeArrayOfObjectsWithColumns(t *testing.T) {
    n := &Node{Columns: []string{"name", "age"}, HasHeaderRow: true}
    output, err := n.Execute(context.Background(), map[string]interface{}{
        "payload": []interface{}{
            map[string]interface{}{"name": "Ada", "age": "30"},
        },
    })
    require.NoError(t, err)
    assert.Equal(t, "name,age\nAda,30\n", output["payload"])
}

func TestNode_Execute_CustomDelimiter(t *testing.T) {
    n := &Node{Delimiter: ";"}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a;b\n1;2\n"})
    require.NoError(t, err)
    rows := output["payload"].([]interface{})
    assert.Equal(t, []interface{}{"a", "b"}, rows[0])
}

func TestNode_Execute_UnsupportedPayloadTypeErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": float64(1)})
    assert.Error(t, err)
}

func TestNode_Execute_InvalidCSVErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a,\"b\n"})
    assert.Error(t, err)
}

func TestNode_Execute_RoundTrip(t *testing.T) {
    n := &Node{HasHeaderRow: true, Columns: []string{"name", "age"}}
    original := "name,age\nAda,30\nGrace,85\n"

    parsed, err := n.Execute(context.Background(), map[string]interface{}{"payload": original})
    require.NoError(t, err)

    serialized, err := n.Execute(context.Background(), map[string]interface{}{"payload": parsed["payload"]})
    require.NoError(t, err)
    assert.Equal(t, original, serialized["payload"])
}
