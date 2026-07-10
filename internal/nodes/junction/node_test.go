package junction

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNode_Execute(t *testing.T) {
    t.Run("returns input unchanged", func(t *testing.T) {
        n := &Node{}
        input := map[string]interface{}{"payload": "hello"}

        output, err := n.Execute(nil, input)

        require.NoError(t, err)
        assert.Equal(t, input, output)
    })
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    assert.NoError(t, n.Validate())
    assert.NoError(t, n.SetConfig(map[string]interface{}{}))
    assert.Equal(t, map[string]interface{}{}, n.GetConfig())
}
