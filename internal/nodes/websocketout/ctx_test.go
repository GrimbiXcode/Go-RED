package websocketout

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxServer implements both Provider and ContextProvider; SendContext
// blocks until ctx ends, standing in for a peer that never drains its
// socket.
type ctxServer struct {
	mu           sync.Mutex
	sendCalls    int
	contextCalls int
}

func (s *ctxServer) Send(data []byte, isText bool) error {
	s.mu.Lock()
	s.sendCalls++
	s.mu.Unlock()
	return nil
}

func (s *ctxServer) SendContext(ctx context.Context, data []byte, isText bool) error {
	s.mu.Lock()
	s.contextCalls++
	s.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (s *ctxServer) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (s *ctxServer) Validate() error                               { return nil }
func (s *ctxServer) GetConfig() map[string]interface{}             { return nil }
func (s *ctxServer) SetConfig(config map[string]interface{}) error { return nil }

// Execute must route the per-message context into the provider's
// SendContext (never the unbounded Send when both exist), and a send
// that is still blocked when that context is cancelled must return
// promptly with an error wrapping context.Canceled.
func TestNode_Execute_CancelledContext_ReturnsPromptly(t *testing.T) {
	s := &ctxServer{}
	rt := registry.NewNodeRuntime("f1", "wsout-1", "websocket out", nil, nil, nil, nil, func(nodeID string) (registry.NodeExecutor, bool) {
		if nodeID == "s1" {
			return s, true
		}
		return nil, false
	})
	ctx, cancel := context.WithCancel(registry.WithRuntime(context.Background(), rt))
	defer cancel()

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	n := &Node{Server: "s1"}
	start := time.Now()
	_, err := n.Execute(ctx, map[string]interface{}{"payload": "hello"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Less(t, elapsed, time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Equal(t, 1, s.contextCalls, "SendContext should be preferred")
	assert.Equal(t, 0, s.sendCalls, "the unbounded Send must not be used when SendContext exists")
}
