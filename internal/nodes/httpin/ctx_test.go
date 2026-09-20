package httpin

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A request still waiting for its "http response" when the flow context
// is cancelled (undeploy) must be completed right away - with a 503 - not
// held open until ResponseTimeoutMs; and Start itself returns promptly.
func TestNode_Start_CancelCompletesPendingRequest(t *testing.T) {
	n := &Node{Method: "GET", Path: "/api/pending-on-cancel", ResponseTimeoutMs: 10000}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitted := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- n.Start(ctx, func(payload map[string]interface{}) {
			emitted <- struct{}{} // deliberately never completes the response
		})
	}()

	require.Eventually(t, func() bool { return SharedAddr() != "" }, 2*time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond) // let the route registration in Start run

	type result struct {
		resp *http.Response
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + testAddr() + "/api/pending-on-cancel")
		resultCh <- result{resp, err}
	}()

	select {
	case <-emitted:
	case <-time.After(2 * time.Second):
		t.Fatal("emit was never called")
	}

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Start did not return within 1s of cancellation")
	}

	select {
	case r := <-resultCh:
		require.NoError(t, r.err)
		defer r.resp.Body.Close()
		assert.Equal(t, http.StatusServiceUnavailable, r.resp.StatusCode)
	case <-time.After(time.Second):
		t.Fatal("the pending request was not completed within 1s of cancellation")
	}
}
