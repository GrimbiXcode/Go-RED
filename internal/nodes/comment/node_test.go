package comment

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_Execute(t *testing.T) {
    t.Run("returns input unchanged", func(t *testing.T) {
        n := &Node{Text: "a note"}
        input := map[string]interface{}{"payload": "hello"}

        output, err := n.Execute(nil, input)

        require.NoError(t, err)
        assert.Equal(t, input, output)
    })
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.Validate())

    require.NoError(t, n.SetConfig(map[string]interface{}{"text": "hello world"}))
    assert.Equal(t, "hello world", n.Text)
    assert.Equal(t, map[string]interface{}{"text": "hello world"}, n.GetConfig())

    t.Run("ignores a non-string text value instead of erroring", func(t *testing.T) {
        require.NoError(t, n.SetConfig(map[string]interface{}{"text": 42}))
        assert.Equal(t, "hello world", n.Text, "previous value should be left untouched")
    })
}
