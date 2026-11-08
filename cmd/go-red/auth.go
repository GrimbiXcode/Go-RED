package main

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
)

// The security baseline (docs/NEXT_LEVEL_PLAN.md, Phase 6):
//
//   - an optional bearer token and/or OIDC sign-in (oidc.go) guard /api/*,
//     /ws and /metrics;
//   - browser requests from another origin are rejected unless the origin
//     is allowed, and allowed origins get CORS headers;
//   - import and deploy are rate limited per client address.

// tokenPattern is what an auth token may look like: long enough to be
// worth having, and only characters that are valid in an HTTP header, a
// WebSocket subprotocol and a URL, so the same token works on every path.
var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{16,}$`)

// validateToken reports whether token is usable (empty means no auth).
func validateToken(token string) error {
	if token == "" {
		return nil
	}
	if !tokenPattern.MatchString(token) {
		return errors.New("auth token must be at least 16 characters, using only A-Z a-z 0-9 . _ ~ -")
	}
	return nil
}

const (
	// websocketSubprotocol is the subprotocol the editor offers; the server
	// selects it so browsers accept the handshake.
	websocketSubprotocol = "gored"
	// tokenSubprotocolPrefix carries the bearer token in the WebSocket
	// handshake, where browsers cannot set an Authorization header.
	tokenSubprotocolPrefix = "gored.token."
)

// tokenFromRequest extracts the presented token: the Authorization bearer
// header, or for the WebSocket handshake a "gored.token.<token>"
// subprotocol or the access_token query parameter.
func tokenFromRequest(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		scheme, rest, ok := strings.Cut(auth, " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			return strings.TrimSpace(rest)
		}
		return ""
	}
	if r.URL.Path != "/ws" {
		return ""
	}
	for _, proto := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		if proto = strings.TrimSpace(proto); strings.HasPrefix(proto, tokenSubprotocolPrefix) {
			return strings.TrimPrefix(proto, tokenSubprotocolPrefix)
		}
	}
	return r.URL.Query().Get("access_token")
}

// isPublicAPIPath lists the API paths that never need authentication:
// probes, and what a signed-out editor needs to sign in or out.
func isPublicAPIPath(p string) bool {
	switch p {
	case "/api/health", "/api/version", "/api/auth/info", "/api/auth/logout":
		return true
	}
	return false
}

// isProtectedPath says which paths authentication guards. The public API
// paths stay open, and the editor itself is static content.
func isProtectedPath(p string) bool {
	switch {
	case isPublicAPIPath(p):
		return false
	case strings.HasPrefix(p, "/api/"), p == "/ws", p == "/metrics":
		return true
	}
	return false
}

// requireAuth answers protected requests that carry neither the token nor
// (with sign-in on) a usable session cookie with 401. Authenticated
// requests get their principal in the context (principalFrom). With neither
// a token nor sessions configured it returns next unchanged.
func requireAuth(token string, sessions *oidcAuth, next http.Handler) http.Handler {
	if token == "" && sessions == nil {
		return next
	}
	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := authenticate(r, expected, sessions); ok {
			r = withPrincipal(r, p)
		} else if isProtectedPath(r.URL.Path) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="go-red"`)
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate checks the bearer token first, then the session cookie.
func authenticate(r *http.Request, token []byte, sessions *oidcAuth) (principal, bool) {
	if len(token) > 0 {
		if presented := tokenFromRequest(r); presented != "" && subtle.ConstantTimeCompare([]byte(presented), token) == 1 {
			return principal{Method: dto.AuthMethodToken}, true
		}
	}
	if sessions != nil && sessionCookieUsable(r) {
		return sessions.session(r)
	}
	return principal{}, false
}

// originAllowed reports whether a request may be served for the browser
// origin it came from: requests without an Origin header (non-browser
// clients, same-origin GETs), the server's own origin, and every entry of
// allowed ("*" allows all).
func originAllowed(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(strings.TrimRight(a, "/"), origin) {
			return true
		}
	}
	return false
}

// crossOrigin reports whether r carries an Origin that is not the server's.
func crossOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || !strings.EqualFold(u.Host, r.Host)
}

// corsMiddleware is the browser origin policy for the API: a cross-origin
// request from an origin that is not allowed is refused (403, so a page on
// another site cannot deploy or import anything with a simple request), an
// allowed one gets CORS headers and its preflight answered.
func corsMiddleware(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !crossOrigin(r) || (!isProtectedPath(r.URL.Path) && !isPublicAPIPath(r.URL.Path)) {
			// Same-origin requests and the editor's own static files need
			// no policy; the API (public endpoints included) does.
			next.ServeHTTP(w, r)
			return
		}
		if !originAllowed(r, allowed) {
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		h.Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a token bucket per client address.
type rateLimiter struct {
	mu      sync.Mutex
	rate    float64 // tokens per second
	burst   float64
	buckets map[string]*bucket
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// newRateLimiter allows perMinute requests per client in steady state and a
// burst of a quarter of that (at least 3). nil means unlimited.
func newRateLimiter(perMinute int) *rateLimiter {
	if perMinute <= 0 {
		return nil
	}
	burst := perMinute / 4
	if burst < 3 {
		burst = 3
	}
	return &rateLimiter{rate: float64(perMinute) / 60, burst: float64(burst), buckets: make(map[string]*bucket), now: time.Now}
}

// allow takes one token for key and reports whether one was available.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= 10000 {
			l.pruneLocked(now)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// pruneLocked forgets clients that have been quiet for ten minutes.
func (l *rateLimiter) pruneLocked(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.buckets, key)
		}
	}
}

// limit wraps a handler so each client address gets at most the configured
// rate; over the limit the request is answered with 429.
func (l *rateLimiter) limit(next http.HandlerFunc) http.HandlerFunc {
	if l == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientKey(r)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "too many requests, try again in a moment")
			return
		}
		next(w, r)
	}
}

// clientKey identifies the client of a request by its address.
func clientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// handleAuthInfo answers GET /api/auth/info (public): which methods the
// editor can use to authenticate.
func (s *server) handleAuthInfo(w http.ResponseWriter, r *http.Request) {
	info := dto.AuthInfo{Methods: []string{}}
	if s.authToken {
		info.Methods = append(info.Methods, dto.AuthMethodToken)
	}
	if s.sessions != nil {
		info.Methods = append(info.Methods, dto.AuthMethodOIDC)
		info.LoginURL = oidcLoginPath
	}
	writeJSON(w, http.StatusOK, info)
}

// handleAuthMe answers GET /api/auth/me: who the request is authenticated
// as ("none" when the server runs without authentication).
func (s *server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	p, ok := principalFrom(r)
	if !ok {
		writeJSON(w, http.StatusOK, dto.AuthUser{Method: dto.AuthMethodNone})
		return
	}
	writeJSON(w, http.StatusOK, dto.AuthUser{Method: p.Method, Subject: p.Subject, Email: p.Email, Name: p.Name})
}

// handleLogout answers POST /api/auth/logout (public, so an expired session
// can still sign out): it clears the session cookies and, unless logout is
// local, names the provider's end-session URL for the browser to visit next.
func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var resp dto.LogoutResponse
	if s.sessions != nil {
		resp.LogoutURL = s.sessions.providerLogoutURL(r)
		s.sessions.clearSession(w)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, resp)
}
