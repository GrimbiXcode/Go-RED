package trigger

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/GrimbiXcode/Go-RED/internal/typedvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An already-cancelled per-message context must be honored: Execute
// returns an error wrapping context.Canceled, sends nothing, and leaves no
// second-send timer behind.
func TestNode_Execute_CancelledContext_Errors(t *testing.T) {
	submitted := make(chan struct{}, 1)
	rt := registry.NewNodeRuntime("f1", "trigger-1", "trigger", nil, nil, nil, func(nodeID string, payload map[string]interface{}) {
		submitted <- struct{}{}
	}, nil)
	ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
	cancel()

	n := &Node{
		FirstPayload:  typedvalue.Value{Type: typedvalue.TypeMsg, Value: "payload"},
		SecondPayload: typedvalue.Value{Type: typedvalue.TypeBool, Value: "false"},
		DelayMs:       5,
	}
	out, err := n.Execute(ctx, map[string]interface{}{"payload": "x"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Nil(t, out)

	n.mu.Lock()
	assert.Empty(t, n.timers, "no second send should have been scheduled")
	n.mu.Unlock()

	select {
	case <-submitted:
		t.Fatal("a second send was delivered for a cancelled message")
	case <-time.After(30 * time.Millisecond):
	}
	require.NoError(t, n.Close())
}
