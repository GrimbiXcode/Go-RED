package httprequest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/typedvalue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cancelling the per-message context while the server is still stalling
// must abort the request promptly (well before TimeoutMs) with an error
// wrapping context.Canceled.
func TestNode_Execute_CancelledContext_ReturnsPromptly(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := &Node{Url: typedvalue.Value{Type: typedvalue.TypeString, Value: srv.URL}, TimeoutMs: 10000}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := n.Execute(ctx, map[string]interface{}{})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should wrap context.Canceled, got: %v", err)
	assert.Less(t, elapsed, time.Second)
}
