package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
)

const (
	testClientID    = "go-red-test"
	testRedirectURL = "http://localhost:8080/auth/callback"
	testCode        = "the-authorization-code"
)

// fakeProvider is an OIDC provider for tests: discovery and keys come from
// oidctest, the token endpoint checks the code and the PKCE verifier and
// answers with an ID token carrying the claims the test set.
type fakeProvider struct {
	t      *testing.T
	srv    *httptest.Server
	key    *rsa.PrivateKey
	mu     sync.Mutex
	claims map[string]any // set per sign-in; iss, aud, exp and nonce are filled in
	nonce  string         // overrides the nonce from the login request when set
	// challenge and nonce of the last login, recorded by the test from the
	// authorization URL.
	lastChallenge string
	lastNonce     string
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	p := &fakeProvider{t: t, key: key}
	discovery := &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: "k1", Algorithm: oidc.RS256}}}
	mux := http.NewServeMux()
	mux.Handle("/", discovery)
	mux.HandleFunc("POST /token", p.token)
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	discovery.SetIssuer(p.srv.URL)
	return p
}

// set changes the provider's state under its lock (the token endpoint runs
// on the server's goroutine).
func (p *fakeProvider) set(change func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	change()
}

func (p *fakeProvider) token(w http.ResponseWriter, r *http.Request) {
	require.NoError(p.t, r.ParseForm())
	p.mu.Lock()
	defer p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if r.PostForm.Get("code") != testCode || r.PostForm.Get("grant_type") != "authorization_code" ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != p.lastChallenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	claims := map[string]any{"iss": p.srv.URL, "aud": testClientID, "exp": time.Now().Add(time.Hour).Unix(), "nonce": p.lastNonce}
	if p.nonce != "" {
		claims["nonce"] = p.nonce
	}
	for k, v := range p.claims {
		claims[k] = v
	}
	raw, err := json.Marshal(claims)
	require.NoError(p.t, err)
	idToken := oidctest.SignIDToken(p.key, "k1", oidc.RS256, string(raw))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken})
}

func (p *fakeProvider) settings() oidcSettings {
	return oidcSettings{
		Issuer:        p.srv.URL,
		ClientID:      testClientID,
		RedirectURL:   testRedirectURL,
		Scopes:        []string{"profile", "email"},
		AllowedEmails: []string{"ada@example.com", "@team.example"},
		AllowedGroups: []string{"go-red-admins"},
		GroupsClaim:   "groups",
		SessionSecret: strings.Repeat("s", 32),
		SessionTTL:    time.Hour,
	}
}

// oidcServer is a router with OIDC sign-in against p (and the test token).
func oidcServer(t *testing.T, p *fakeProvider, opts routerOptions, edit func(*oidcSettings)) (http.Handler, *oidcAuth) {
	t.Helper()
	settings := p.settings()
	if edit != nil {
		edit(&settings)
	}
	auth, err := newOIDCAuth(settings, p.srv.Client())
	require.NoError(t, err)
	opts.OIDC = auth
	_, h := newTestServerWith(t, opts)
	return h, auth
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// startLogin runs GET /auth/login and records what the provider would see.
func startLogin(t *testing.T, h http.Handler, p *fakeProvider, next string) (state string, login *http.Cookie) {
	t.Helper()
	target := "/auth/login"
	if next != "" {
		target += "?next=" + url.QueryEscape(next)
	}
	w := serve(h, request(http.MethodGet, target, nil))
	require.Equal(t, http.StatusFound, w.Code, w.Body.String())
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(loc.String(), p.srv.URL+"/auth?"), loc.String())
	q := loc.Query()
	assert.Equal(t, testClientID, q.Get("client_id"))
	assert.Equal(t, testRedirectURL, q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "openid profile email", q.Get("scope"))
	require.NotEmpty(t, q.Get("state"))
	require.NotEmpty(t, q.Get("nonce"))
	p.set(func() { p.lastChallenge, p.lastNonce = q.Get("code_challenge"), q.Get("nonce") })
	login = cookieNamed(w, loginCookieName)
	require.NotNil(t, login)
	assert.True(t, login.HttpOnly)
	return q.Get("state"), login
}

// callback simulates the provider redirecting the browser back.
func callback(h http.Handler, query string, login *http.Cookie) *httptest.ResponseRecorder {
	req := request(http.MethodGet, "/auth/callback?"+query, nil)
	if login != nil {
		req.AddCookie(login)
	}
	return serve(h, req)
}

// signIn runs the whole flow with claims and returns the session cookie.
func signIn(t *testing.T, h http.Handler, p *fakeProvider, claims map[string]any) *http.Cookie {
	t.Helper()
	p.set(func() { p.claims = claims })
	state, login := startLogin(t, h, p, "/flows/demo?x=1")
	w := callback(h, "code="+testCode+"&state="+url.QueryEscape(state), login)
	require.Equal(t, http.StatusFound, w.Code, w.Body.String())
	assert.Equal(t, "/flows/demo?x=1", w.Header().Get("Location"))
	session := cookieNamed(w, "gored_session")
	require.NotNil(t, session)
	assert.True(t, session.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, session.SameSite)
	return session
}

func withCookie(req *http.Request, c *http.Cookie) *http.Request {
	req.AddCookie(c)
	return req
}

var ada = map[string]any{"sub": "user-1", "email": "ada@example.com", "email_verified": true, "name": "Ada"}

func TestOIDCSignInFlow(t *testing.T) {
	p := newFakeProvider(t)
	h, _ := oidcServer(t, p, routerOptions{}, nil)

	assert.Equal(t, http.StatusUnauthorized, serve(h, request(http.MethodGet, "/api/flows", nil)).Code)

	session := signIn(t, h, p, ada)

	assert.Equal(t, http.StatusOK, serve(h, withCookie(request(http.MethodGet, "/api/flows", nil), session)).Code)
	assert.NotEqual(t, http.StatusUnauthorized, serve(h, withCookie(request(http.MethodGet, "/ws", map[string]string{"Origin": "http://localhost:8080"}), session)).Code,
		"the same-origin WebSocket handshake carries the cookie")

	w := serve(h, withCookie(request(http.MethodGet, "/api/auth/me", nil), session))
	require.Equal(t, http.StatusOK, w.Code)
	var me dto.AuthUser
	decodeBody(t, w, &me)
	assert.Equal(t, dto.AuthUser{Method: dto.AuthMethodOIDC, Subject: "user-1", Email: "ada@example.com", Name: "Ada"}, me)

	t.Run("state-changing requests need proof of same origin", func(t *testing.T) {
		create := func(headers map[string]string) int {
			req := httptest.NewRequest(http.MethodPost, "/api/flows", strings.NewReader(`{"name":"From cookie"}`))
			req.Host = "localhost:8080"
			req.Header.Set("Content-Type", "application/json")
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			return serve(h, withCookie(req, session)).Code
		}
		assert.Equal(t, http.StatusUnauthorized, create(nil), "no Origin, no Sec-Fetch-Site")
		assert.Equal(t, http.StatusUnauthorized, create(map[string]string{"Sec-Fetch-Site": "same-site"}))
		assert.Equal(t, http.StatusCreated, create(map[string]string{"Origin": "http://localhost:8080"}))
		assert.Equal(t, http.StatusCreated, create(map[string]string{"Sec-Fetch-Site": "same-origin"}))
	})

	t.Run("logout clears the cookie and needs no auth", func(t *testing.T) {
		w := serve(h, request(http.MethodPost, "/api/auth/logout", map[string]string{"Origin": "http://localhost:8080"}))
		assert.Equal(t, http.StatusNoContent, w.Code)
		cleared := cookieNamed(w, "gored_session")
		require.NotNil(t, cleared)
		assert.Less(t, cleared.MaxAge, 0)
	})
}

func TestOIDCSessionCookieIsNotUsableCrossOrigin(t *testing.T) {
	p := newFakeProvider(t)
	h, _ := oidcServer(t, p, routerOptions{AllowedOrigins: []string{"http://app.example"}}, nil)
	session := signIn(t, h, p, ada)

	w := serve(h, withCookie(request(http.MethodGet, "/api/flows", map[string]string{"Origin": "http://app.example"}), session))
	assert.Equal(t, http.StatusUnauthorized, w.Code, "an allowed origin still has to bring its own token")
	w = serve(h, withCookie(request(http.MethodGet, "/ws", map[string]string{"Origin": "http://app.example"}), session))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOIDCSessionCookieTamperingAndExpiry(t *testing.T) {
	p := newFakeProvider(t)
	h, auth := oidcServer(t, p, routerOptions{}, nil)
	session := signIn(t, h, p, ada)

	body, sig, _ := strings.Cut(session.Value, ".")
	raw, err := base64.RawURLEncoding.DecodeString(sig)
	require.NoError(t, err)
	raw[0] ^= 0xff
	flipped := base64.RawURLEncoding.EncodeToString(raw)
	forged, err := signCookie([]byte("another key"), sessionClaims{Subject: "admin", Expires: time.Now().Add(time.Hour).Unix()})
	require.NoError(t, err)
	for name, value := range map[string]string{
		"flipped signature": body + "." + flipped,
		"no signature":      body,
		"other key":         forged,
		"garbage":           "not-a-cookie",
	} {
		c := &http.Cookie{Name: session.Name, Value: value}
		assert.Equal(t, http.StatusUnauthorized, serve(h, withCookie(request(http.MethodGet, "/api/flows", nil), c)).Code, name)
	}

	auth.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	assert.Equal(t, http.StatusUnauthorized, serve(h, withCookie(request(http.MethodGet, "/api/flows", nil), session)).Code, "expired")
}

func TestOIDCCallbackRejectsBadResponses(t *testing.T) {
	p := newFakeProvider(t)
	h, _ := oidcServer(t, p, routerOptions{}, nil)
	p.set(func() { p.claims = ada })

	t.Run("missing login cookie", func(t *testing.T) {
		state, _ := startLogin(t, h, p, "")
		w := callback(h, "code="+testCode+"&state="+state, nil)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Nil(t, cookieNamed(w, "gored_session"))
	})
	t.Run("state mismatch", func(t *testing.T) {
		_, login := startLogin(t, h, p, "")
		w := callback(h, "code="+testCode+"&state=other", login)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("wrong PKCE verifier", func(t *testing.T) {
		state, login := startLogin(t, h, p, "")
		p.set(func() { p.lastChallenge = "something-else" })
		w := callback(h, "code="+testCode+"&state="+state, login)
		assert.Equal(t, http.StatusBadGateway, w.Code)
		assert.NotContains(t, w.Body.String(), "invalid_grant", "provider details stay in the log")
	})
	t.Run("nonce mismatch", func(t *testing.T) {
		p.set(func() { p.nonce = "replayed" })
		defer p.set(func() { p.nonce = "" })
		state, login := startLogin(t, h, p, "")
		w := callback(h, "code="+testCode+"&state="+state, login)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Nil(t, cookieNamed(w, "gored_session"))
	})
	t.Run("provider error", func(t *testing.T) {
		_, login := startLogin(t, h, p, "")
		w := callback(h, "error=access_denied&error_description=%3Cscript%3E", login)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, w.Body.String(), "<script>")
	})
	t.Run("login cookie is single use", func(t *testing.T) {
		state, login := startLogin(t, h, p, "")
		w := callback(h, "code="+testCode+"&state="+state, login)
		require.Equal(t, http.StatusFound, w.Code)
		cleared := cookieNamed(w, loginCookieName)
		require.NotNil(t, cleared)
		assert.Less(t, cleared.MaxAge, 0)
	})
}

func TestOIDCAllowlist(t *testing.T) {
	p := newFakeProvider(t)
	h, _ := oidcServer(t, p, routerOptions{}, nil)

	attempt := func(claims map[string]any) int {
		p.set(func() { p.claims = claims })
		state, login := startLogin(t, h, p, "")
		return callback(h, "code="+testCode+"&state="+url.QueryEscape(state), login).Code
	}
	assert.Equal(t, http.StatusFound, attempt(map[string]any{"sub": "a", "email": "ADA@example.com", "email_verified": true}), "listed address, any case")
	assert.Equal(t, http.StatusFound, attempt(map[string]any{"sub": "b", "email": "bob@team.example", "email_verified": true}), "listed domain")
	assert.Equal(t, http.StatusFound, attempt(map[string]any{"sub": "c", "groups": []string{"x", "go-red-admins"}}), "listed group")
	assert.Equal(t, http.StatusFound, attempt(map[string]any{"sub": "c", "groups": "go-red-admins"}), "single group string")
	assert.Equal(t, http.StatusForbidden, attempt(map[string]any{"sub": "d", "email": "ada@example.com"}), "email_verified missing")
	assert.Equal(t, http.StatusForbidden, attempt(map[string]any{"sub": "d", "email": "ada@example.com", "email_verified": false}), "not verified")
	assert.Equal(t, http.StatusForbidden, attempt(map[string]any{"sub": "e", "email": "eve@evilteam.example", "email_verified": true}), "suffix without @ is not the domain")
	assert.Equal(t, http.StatusForbidden, attempt(map[string]any{"sub": "f", "groups": []string{"users"}}))

	open, _ := oidcServer(t, p, routerOptions{}, func(s *oidcSettings) {
		s.AllowedEmails, s.AllowedGroups, s.AllowAnyUser = nil, nil, true
	})
	p.set(func() { p.claims = map[string]any{"sub": "anyone"} })
	state, login := startLogin(t, open, p, "")
	assert.Equal(t, http.StatusFound, callback(open, "code="+testCode+"&state="+url.QueryEscape(state), login).Code)
}

func TestOIDCWithTokenAndAuthInfo(t *testing.T) {
	p := newFakeProvider(t)
	h, _ := oidcServer(t, p, routerOptions{AuthToken: testToken}, nil)

	w := serve(h, request(http.MethodGet, "/api/auth/info", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var info dto.AuthInfo
	decodeBody(t, w, &info)
	assert.Equal(t, dto.AuthInfo{Methods: []string{"token", "oidc"}, LoginURL: "/auth/login"}, info)

	w = serve(h, request(http.MethodGet, "/api/auth/me", map[string]string{"Authorization": "Bearer " + testToken}))
	require.Equal(t, http.StatusOK, w.Code)
	var me dto.AuthUser
	decodeBody(t, w, &me)
	assert.Equal(t, dto.AuthMethodToken, me.Method)

	assert.Equal(t, http.StatusUnauthorized, serve(h, request(http.MethodGet, "/api/auth/me", nil)).Code)

	_, plain := newTestServer(t)
	var plainInfo dto.AuthInfo
	decodeBody(t, serve(plain, request(http.MethodGet, "/api/auth/info", nil)), &plainInfo)
	assert.Equal(t, dto.AuthInfo{Methods: []string{}}, plainInfo)
	var nobody dto.AuthUser
	decodeBody(t, serve(plain, request(http.MethodGet, "/api/auth/me", nil)), &nobody)
	assert.Equal(t, dto.AuthUser{Method: dto.AuthMethodNone}, nobody)
}

func TestOIDCProviderDown(t *testing.T) {
	settings := oidcSettings{Issuer: "http://127.0.0.1:1", ClientID: "c", RedirectURL: testRedirectURL, AllowAnyUser: true, SessionTTL: time.Hour}
	auth, err := newOIDCAuth(settings, &http.Client{Timeout: time.Second})
	require.NoError(t, err)
	_, h := newTestServerWith(t, routerOptions{OIDC: auth})
	w := serve(h, request(http.MethodGet, "/auth/login", nil))
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.NotContains(t, w.Body.String(), "127.0.0.1")
}

func TestOIDCSecureCookies(t *testing.T) {
	p := newFakeProvider(t)
	auth, err := newOIDCAuth(oidcSettings{Issuer: p.srv.URL, ClientID: "c", RedirectURL: "https://red.example/auth/callback", AllowAnyUser: true, SessionTTL: time.Hour}, p.srv.Client())
	require.NoError(t, err)
	assert.True(t, auth.secure)
	assert.Equal(t, "__Host-gored_session", auth.sessionName)
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":                     "/",
		"/":                    "/",
		"/flows/a?b=c":         "/flows/a?b=c",
		"//evil.example":       "/",
		"/\\evil.example":      "/",
		"https://evil.example": "/",
		"flows":                "/",
		"/auth/login":          "/",
		"/x\r\nSet-Cookie: a":  "/",
	} {
		assert.Equal(t, want, safeNext(in), "%q", in)
	}
}

func TestOIDCCallbackRefusalsAreLogged(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	p := newFakeProvider(t)
	h, _ := oidcServer(t, p, routerOptions{}, nil)
	p.set(func() { p.claims = ada })

	state, login := startLogin(t, h, p, "")
	assert.Equal(t, http.StatusBadRequest, callback(h, "code="+testCode+"&state="+state, nil).Code)
	assert.Contains(t, logs.String(), "oidc callback without a usable sign-in cookie")
	assert.Contains(t, logs.String(), "cookie=\"missing")

	logs.Reset()
	assert.Equal(t, http.StatusBadRequest, callback(h, "code="+testCode+"&state=other", login).Code)
	assert.Contains(t, logs.String(), "oidc callback does not match the sign-in it answers")
	assert.Contains(t, logs.String(), "stateMatches=false")
	assert.NotContains(t, logs.String(), state, "the state value itself is not logged")
}
