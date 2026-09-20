package execnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cancelling the per-message context (undeploy, or the engine's node
// timeout) must kill the running command and surface as an error wrapping
// context.Canceled - not as a "successful" run with exit code -1, and not
// as this node's own TimeoutMs message.
func TestNode_Execute_CancelledContext_KillsCommand(t *testing.T) {
	t.Setenv(enableEnvVar, "1")
	n := &Node{Command: "sleep", Args: []string{"5"}, TimeoutMs: 10000}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	out, err := n.Execute(ctx, map[string]interface{}{})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Nil(t, out)
	assert.Less(t, elapsed, 2*time.Second, "the command should be killed on cancel, not run to completion")
}
