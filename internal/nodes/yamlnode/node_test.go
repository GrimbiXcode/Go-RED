package yamlnode

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, map[string]interface{}{}, n.GetConfig())
}

func TestNode_Execute_ParsesStringPayload(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a: 1\nb: x\n"})
    require.NoError(t, err)
    parsed, ok := output["payload"].(map[string]interface{})
    require.True(t, ok)
    assert.Equal(t, 1, parsed["a"])
    assert.Equal(t, "x", parsed["b"])
}

func TestNode_Execute_ParsesNestedMapping(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a:\n  b: 1\n  c: 2\n"})
    require.NoError(t, err)
    parsed, ok := output["payload"].(map[string]interface{})
    require.True(t, ok)
    nested, ok := parsed["a"].(map[string]interface{})
    require.True(t, ok, "nested mapping should be map[string]interface{}, not map[interface{}]interface{}")
    assert.Equal(t, 1, nested["b"])
}

func TestNode_Execute_InvalidYAMLErrors(t *testing.T) {
    n := &Node{}
    _, err := n.Execute(context.Background(), map[string]interface{}{"payload": "a: [1, 2\n"})
    assert.Error(t, err)
}

func TestNode_Execute_StringifiesObjectPayload(t *testing.T) {
    n := &Node{}
    output, err := n.Execute(context.Background(), map[string]interface{}{"payload": map[string]interface{}{"a": 1}})
    require.NoError(t, err)
    str, ok := output["payload"].(string)
    require.True(t, ok)
    assert.Contains(t, str, "a: 1")
}

func TestNode_Execute_RoundTrip(t *testing.T) {
    n := &Node{}
    original := map[string]interface{}{"a": 1, "b": []interface{}{"x", "y"}, "c": true}

    stringified, err := n.Execute(context.Background(), map[string]interface{}{"payload": original})
    require.NoError(t, err)

    parsed, err := n.Execute(context.Background(), map[string]interface{}{"payload": stringified["payload"]})
    require.NoError(t, err)

    assert.Equal(t, original, parsed["payload"])
}
