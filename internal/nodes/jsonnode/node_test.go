package jsonnode

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{"pretty": true}))
    assert.True(t, n.Pretty)
    assert.Equal(t, map[string]interface{}{"pretty": true}, n.GetConfig())
}

func TestNode_Execute_ParsesStringPayload(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": `{"a":1,"b":"x"}`})
    require.NoError(t, err)
    parsed, ok := output["payload"].(map[string]interface{})
    require.True(t, ok)
    assert.Equal(t, float64(1), parsed["a"])
    assert.Equal(t, "x", parsed["b"])
}

func TestNode_Execute_InvalidJSONErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "not json"})
    assert.Error(t, err)
}

func TestNode_Execute_StringifiesObjectPayload(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)
    assert.Equal(t, `{"a":1}`, output["payload"])
}

func TestNode_Execute_PrettyStringify(t *testing.T) {
    n := &Node{Pretty: true}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": map[string]interface{}{"a": float64(1)}})
    require.NoError(t, err)
    assert.Equal(t, "{\n  \"a\": 1\n}", output["payload"])
}

func TestNode_Execute_RoundTrip(t *testing.T) {
    n := &Node{}
    original := map[string]interface{}{"a": float64(1), "b": []interface{}{"x", "y"}, "c": true}

    stringified, err := n.Execute(context.Background(), map[string]interface{}{"payload": original})
    require.NoError(t, err)

    parsed, err := n.Execute(context.Background(), map[string]interface{}{"payload": stringified["payload"]})
    require.NoError(t, err)

    assert.Equal(t, original, parsed["payload"])
}
