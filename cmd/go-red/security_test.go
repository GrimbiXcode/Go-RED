package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "correct-horse-battery-staple"

// request builds a request against the router with a fixed Host so origin
// checks are deterministic.
func request(method, target string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "localhost:8080"
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAuthTokenGuardsAPIWebSocketAndMetrics(t *testing.T) {
	_, h := newTestServerWith(t, routerOptions{AuthToken: testToken})

	w := serve(h, request(http.MethodGet, "/api/flows", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Bearer")

	w = serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Authorization": "Bearer wrong-token-wrong-token"}))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Authorization": "Bearer " + testToken}))
	assert.Equal(t, http.StatusOK, w.Code)

	w = serve(h, request(http.MethodGet, "/metrics", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	w = serve(h, request(http.MethodGet, "/metrics", map[string]string{"Authorization": "Bearer " + testToken}))
	assert.Equal(t, http.StatusOK, w.Code)

	// Probes and the editor itself stay public.
	assert.Equal(t, http.StatusOK, serve(h, request(http.MethodGet, "/api/health", nil)).Code)
	assert.Equal(t, http.StatusOK, serve(h, request(http.MethodGet, "/api/version", nil)).Code)
	assert.NotEqual(t, http.StatusUnauthorized, serve(h, request(http.MethodGet, "/", nil)).Code)

	// The WebSocket handshake carries the token as a subprotocol or query
	// parameter (browsers cannot set Authorization there). Without the
	// WebSocket handler wired up, /ws falls through to the SPA handler, so
	// "not 401" is the signal that the token was accepted.
	assert.Equal(t, http.StatusUnauthorized, serve(h, request(http.MethodGet, "/ws", nil)).Code)
	w = serve(h, request(http.MethodGet, "/ws", map[string]string{"Sec-WebSocket-Protocol": "gored, gored.token." + testToken}))
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	w = serve(h, request(http.MethodGet, "/ws?access_token="+testToken, nil))
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	w = serve(h, request(http.MethodGet, "/ws?access_token=nope", map[string]string{"Sec-WebSocket-Protocol": "gored"}))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestValidateToken(t *testing.T) {
	assert.NoError(t, validateToken(""))
	assert.NoError(t, validateToken(testToken))
	assert.Error(t, validateToken("short"))
	assert.Error(t, validateToken("has spaces in it and more"))
	assert.Error(t, validateToken("has/slashes/and+plus=="))
}

func TestOriginPolicy(t *testing.T) {
	t.Run("cross-site requests are refused unless the origin is allowed", func(t *testing.T) {
		_, h := newTestServerWith(t, routerOptions{})

		w := serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Origin": "http://evil.example"}))
		assert.Equal(t, http.StatusForbidden, w.Code)

		w = serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Origin": "http://localhost:8080"}))
		assert.Equal(t, http.StatusOK, w.Code, "the editor's own origin")
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "same-origin needs no CORS headers")

		w = serve(h, request(http.MethodGet, "/api/flows", nil))
		assert.Equal(t, http.StatusOK, w.Code, "non-browser clients send no Origin")

		w = serve(h, request(http.MethodGet, "/api/health", map[string]string{"Origin": "http://evil.example"}))
		assert.Equal(t, http.StatusForbidden, w.Code, "the policy covers the public endpoints too")
	})

	t.Run("allowed origins get CORS headers and preflights", func(t *testing.T) {
		_, h := newTestServerWith(t, routerOptions{AllowedOrigins: []string{"http://app.example"}})

		w := serve(h, request(http.MethodOptions, "/api/flows", map[string]string{"Origin": "http://app.example", "Access-Control-Request-Method": "POST"}))
		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "http://app.example", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")

		w = serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Origin": "http://app.example"}))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "http://app.example", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, w.Header().Values("Vary"), "Origin")

		w = serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Origin": "http://other.example"}))
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("a wildcard allows every origin", func(t *testing.T) {
		_, h := newTestServerWith(t, routerOptions{AllowedOrigins: []string{"*"}})
		w := serve(h, request(http.MethodGet, "/api/flows", map[string]string{"Origin": "http://anything.example"}))
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "http://anything.example", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("originAllowed", func(t *testing.T) {
		req := request(http.MethodGet, "/ws", map[string]string{"Origin": "http://LOCALHOST:8080"})
		assert.True(t, originAllowed(req, nil), "host comparison ignores case")
		req = request(http.MethodGet, "/ws", map[string]string{"Origin": "http://localhost:5173"})
		assert.False(t, originAllowed(req, nil))
		assert.True(t, originAllowed(req, []string{"http://localhost:5173/"}))
		req = request(http.MethodGet, "/ws", map[string]string{"Origin": "not a url"})
		assert.False(t, originAllowed(req, []string{"*"}), "a malformed origin never passes")
	})
}

func TestRateLimitOnImportAndDeploy(t *testing.T) {
	_, h := newTestServerWith(t, routerOptions{RateLimitPerMinute: 12}) // burst of 3

	flow := dto.Flow{ID: "rl", Name: "Rate", Nodes: map[string]dto.Node{}, Connections: []dto.Connection{}}
	for i := 0; i < 3; i++ {
		w := do(t, h, http.MethodPost, "/api/flows/import", flow)
		require.Equal(t, http.StatusCreated, w.Code, "request %d: %s", i, w.Body.String())
	}
	w := do(t, h, http.MethodPost, "/api/flows/import", flow)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "1", w.Header().Get("Retry-After"))

	// Deploy shares the client's bucket; other endpoints are not limited.
	w = do(t, h, http.MethodPost, "/api/flows/rl/deploy", nil)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, http.StatusOK, do(t, h, http.MethodGet, "/api/flows", nil).Code)
}

func TestRateLimiterRefills(t *testing.T) {
	l := newRateLimiter(12) // 0.2 tokens/s, burst 3
	now := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		assert.True(t, l.allow("a"), "burst %d", i)
	}
	assert.False(t, l.allow("a"))
	assert.True(t, l.allow("b"), "other clients have their own bucket")

	now = now.Add(5 * time.Second)
	assert.True(t, l.allow("a"), "one token every five seconds")
	assert.False(t, l.allow("a"))

	assert.Nil(t, newRateLimiter(0), "0 disables the limit")
}
