package join

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Close (called by the engine on undeploy) must drop every incomplete
// group, and a part arriving afterwards starts a fresh group rather than
// completing a stale one.
func TestNode_Close_ReleasesPendingGroups(t *testing.T) {
	n := &Node{}
	_, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 0, 2, "array", "a", nil))
	require.NoError(t, err)
	n.mu.Lock()
	require.Len(t, n.groups, 1)
	n.mu.Unlock()

	require.NoError(t, n.Close())

	n.mu.Lock()
	assert.Empty(t, n.groups, "Close should release the buffered group")
	n.mu.Unlock()

	outputs, err := n.ExecuteMulti(context.Background(), partsMsg("g1", 1, 2, "array", "b", nil))
	require.NoError(t, err)
	assert.Empty(t, outputs, "the stale part must not complete the group after Close")
}
