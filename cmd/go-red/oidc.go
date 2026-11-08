package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/GrimbiXcode/Go-RED/internal/dto"
)

// Editor sign-in through an OpenID Connect provider.
//
// GET /auth/login sends the browser to the provider (authorization code
// flow with PKCE, state and nonce kept in a short-lived signed cookie);
// GET /auth/callback exchanges the code, verifies the ID token, checks the
// allowlist and sets a signed session cookie. requireAuth accepts that
// cookie only on same-origin requests (see sessionCookieUsable), so another
// site can never ride on it. Sessions are stateless: logout clears the
// cookie, and a user removed at the provider keeps access until the
// session expires (-session-ttl).

const (
	oidcLoginPath    = "/auth/login"
	oidcCallbackPath = "/auth/callback"

	// loginCookieName holds state, nonce, PKCE verifier and the page to
	// return to while the browser is at the provider.
	loginCookieName = "gored_login"
	loginCookieTTL  = 10 * time.Minute

	// oidcHTTPTimeout bounds discovery, key and token requests to the
	// provider.
	oidcHTTPTimeout = 15 * time.Second
)

// oidcSettings is the sign-in part of Config.
type oidcSettings struct {
	Issuer        string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	AllowedEmails []string
	AllowedGroups []string
	GroupsClaim   string
	AllowAnyUser  bool
	SessionSecret string
	SessionTTL    time.Duration
}

// oidcSettings extracts the sign-in settings; ok is false when sign-in is off.
func (c Config) oidcSettings() (oidcSettings, bool) {
	if c.OIDCIssuer == "" {
		return oidcSettings{}, false
	}
	return oidcSettings{
		Issuer:        c.OIDCIssuer,
		ClientID:      c.OIDCClientID,
		ClientSecret:  c.OIDCClientSecret,
		RedirectURL:   c.OIDCRedirectURL,
		Scopes:        c.OIDCScopes,
		AllowedEmails: c.OIDCAllowedEmails,
		AllowedGroups: c.OIDCAllowedGroups,
		GroupsClaim:   c.OIDCGroupsClaim,
		AllowAnyUser:  c.OIDCAllowAnyUser,
		SessionSecret: c.SessionSecret,
		SessionTTL:    c.SessionTTL,
	}, true
}

// principal is who a request was authenticated as; requireAuth puts it into
// the request context.
type principal struct {
	Method  string
	Subject string
	Email   string
	Name    string
}

type principalKey struct{}

func withPrincipal(r *http.Request, p principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, p))
}

// principalFrom returns the authenticated principal of r, if any.
func principalFrom(r *http.Request) (principal, bool) {
	p, ok := r.Context().Value(principalKey{}).(principal)
	return p, ok
}

// oidcAuth runs the sign-in flow and checks session cookies.
type oidcAuth struct {
	cfg         oidcSettings
	secure      bool   // the redirect URL is https: cookies get Secure and the __Host- prefix
	sessionName string // session cookie name
	sessionKey  []byte
	loginKey    []byte
	client      *http.Client
	now         func() time.Time

	mu       sync.Mutex
	provider *oidc.Provider // discovered lazily, kept once it worked
}

// newOIDCAuth prepares sign-in. The provider is discovered on first use (and
// again after a failure), so a provider that is down at startup does not
// keep the server from starting.
func newOIDCAuth(cfg oidcSettings, client *http.Client) (*oidcAuth, error) {
	if client == nil {
		client = &http.Client{Timeout: oidcHTTPTimeout}
	}
	secret := []byte(cfg.SessionSecret)
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("session key: %w", err)
		}
		slog.Warn("no session secret configured: editor sessions end when the server restarts")
	}
	redirect, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("oidc redirect url: %w", err)
	}
	a := &oidcAuth{
		cfg:         cfg,
		secure:      redirect.Scheme == "https",
		sessionName: "gored_session",
		sessionKey:  deriveKey(secret, "go-red session"),
		loginKey:    deriveKey(secret, "go-red login"),
		client:      client,
		now:         time.Now,
	}
	if a.secure {
		a.sessionName = "__Host-gored_session"
	}
	return a, nil
}

// deriveKey gives every cookie kind its own HMAC key.
func deriveKey(secret []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

// clientContext carries the HTTP client for the provider. It is not
// cancelled: go-oidc keeps the context of NewProvider for later key fetches.
func (a *oidcAuth) clientContext() context.Context {
	return oidc.ClientContext(context.Background(), a.client)
}

// discover returns the provider, running discovery if it has not worked yet.
func (a *oidcAuth) discover() (*oidc.Provider, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.provider != nil {
		return a.provider, nil
	}
	p, err := oidc.NewProvider(a.clientContext(), a.cfg.Issuer)
	if err != nil {
		return nil, err
	}
	a.provider = p
	return p, nil
}

func (a *oidcAuth) oauthConfig(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.cfg.ClientID,
		ClientSecret: a.cfg.ClientSecret,
		RedirectURL:  a.cfg.RedirectURL,
		Endpoint:     p.Endpoint(),
		Scopes:       append([]string{oidc.ScopeOpenID}, a.cfg.Scopes...),
	}
}

// loginState is the content of the login cookie.
type loginState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Next     string `json:"r"`
	Expires  int64  `json:"e"`
}

// sessionClaims is the content of the session cookie.
type sessionClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email,omitempty"`
	Name    string `json:"name,omitempty"`
	Expires int64  `json:"exp"`
}

// handleLogin starts the sign-in: GET /auth/login[?next=/path].
func (a *oidcAuth) handleLogin(w http.ResponseWriter, r *http.Request) {
	p, err := a.discover()
	if err != nil {
		slog.Error("oidc discovery failed", "issuer", a.cfg.Issuer, "err", err)
		authErrorPage(w, http.StatusBadGateway, "The sign-in provider is not reachable. Try again in a moment.")
		return
	}
	st := loginState{
		State:    randomString(),
		Nonce:    randomString(),
		Verifier: oauth2.GenerateVerifier(),
		Next:     safeNext(r.URL.Query().Get("next")),
		Expires:  a.now().Add(loginCookieTTL).Unix(),
	}
	value, err := signCookie(a.loginKey, st)
	if err != nil {
		slog.Error("oidc login cookie", "err", err)
		authErrorPage(w, http.StatusInternalServerError, "Sign-in could not be started.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     loginCookieName,
		Value:    value,
		Path:     "/auth/",
		MaxAge:   int(loginCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
	target := a.oauthConfig(p).AuthCodeURL(st.State, oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier))
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
}

// handleCallback finishes the sign-in: GET /auth/callback?code=&state=.
func (a *oidcAuth) handleCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	// The login cookie is single use, whatever happens next.
	http.SetCookie(w, &http.Cookie{Name: loginCookieName, Path: "/auth/", MaxAge: -1, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode})

	var st loginState
	c, err := r.Cookie(loginCookieName)
	if err != nil || !verifyCookie(a.loginKey, c.Value, &st) || a.now().Unix() > st.Expires {
		reason := "expired"
		switch {
		case err != nil:
			reason = "missing (blocked, or sent to another host than the redirect URL)"
		case st.Expires == 0:
			reason = "invalid signature (session secret changed, or tampered)"
		}
		slog.Info("oidc callback without a usable sign-in cookie", "cookie", reason, "host", r.Host)
		authErrorPage(w, http.StatusBadRequest, "The sign-in expired or was started in another browser. Please sign in again.")
		return
	}
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		slog.Info("oidc provider refused sign-in", "error", sanitizeString(e), "description", sanitizeString(q.Get("error_description")))
		authErrorPage(w, http.StatusForbidden, "The sign-in provider did not sign you in.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(st.State)) != 1 || q.Get("code") == "" {
		slog.Info("oidc callback does not match the sign-in it answers", "stateMatches", q.Get("state") == st.State, "hasCode", q.Get("code") != "")
		authErrorPage(w, http.StatusBadRequest, "The sign-in response did not match. Please sign in again.")
		return
	}

	p, err := a.discover()
	if err != nil {
		slog.Error("oidc discovery failed", "issuer", a.cfg.Issuer, "err", err)
		authErrorPage(w, http.StatusBadGateway, "The sign-in provider is not reachable. Try again in a moment.")
		return
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(r.Context(), a.client), oidcHTTPTimeout)
	defer cancel()
	token, err := a.oauthConfig(p).Exchange(ctx, q.Get("code"), oauth2.VerifierOption(st.Verifier))
	if err != nil {
		slog.Error("oidc code exchange failed", "err", err)
		authErrorPage(w, http.StatusBadGateway, "Sign-in failed. Please try again.")
		return
	}
	rawID, _ := token.Extra("id_token").(string)
	if rawID == "" {
		slog.Error("oidc token response without id_token")
		authErrorPage(w, http.StatusBadGateway, "Sign-in failed. Please try again.")
		return
	}
	idToken, err := p.VerifierContext(a.clientContext(), &oidc.Config{ClientID: a.cfg.ClientID, Now: a.now}).Verify(ctx, rawID)
	if err != nil {
		slog.Error("oidc id token rejected", "err", err)
		authErrorPage(w, http.StatusBadGateway, "Sign-in failed. Please try again.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(st.Nonce)) != 1 {
		slog.Error("oidc id token nonce mismatch")
		authErrorPage(w, http.StatusBadRequest, "The sign-in response did not match. Please sign in again.")
		return
	}

	user, err := a.userFromToken(idToken)
	if err != nil {
		slog.Error("oidc id token claims", "err", err)
		authErrorPage(w, http.StatusBadGateway, "Sign-in failed. Please try again.")
		return
	}
	if !user.allowed {
		slog.Warn("oidc sign-in refused: not on the allowlist", "subject", user.Subject, "email", user.Email)
		authErrorPage(w, http.StatusForbidden, "Your account is not allowed to use this Go-RED server.")
		return
	}

	value, err := signCookie(a.sessionKey, sessionClaims{
		Subject: user.Subject,
		Email:   user.Email,
		Name:    user.Name,
		Expires: a.now().Add(a.cfg.SessionTTL).Unix(),
	})
	if err != nil {
		slog.Error("oidc session cookie", "err", err)
		authErrorPage(w, http.StatusInternalServerError, "Sign-in failed. Please try again.")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     a.sessionName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(a.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	})
	slog.Info("oidc sign-in", "subject", user.Subject, "email", user.Email)
	http.Redirect(w, r, st.Next, http.StatusFound)
}

// oidcUser is what the ID token says about the user.
type oidcUser struct {
	principal
	allowed bool
}

// userFromToken reads the claims and applies the allowlist: a verified
// e-mail address (exact or @domain) or one of the allowed groups.
func (a *oidcAuth) userFromToken(t *oidc.IDToken) (oidcUser, error) {
	var claims map[string]any
	if err := t.Claims(&claims); err != nil {
		return oidcUser{}, err
	}
	str := func(key string) string {
		s, _ := claims[key].(string)
		return sanitizeString(s)
	}
	u := oidcUser{principal: principal{Method: dto.AuthMethodOIDC, Subject: sanitizeString(t.Subject), Email: str("email")}}
	for _, key := range []string{"name", "preferred_username", "email"} {
		if u.Name = str(key); u.Name != "" {
			break
		}
	}
	if u.Name == "" {
		u.Name = u.Subject
	}

	if a.cfg.AllowAnyUser {
		u.allowed = true
		return u, nil
	}
	// A missing email_verified claim does not count as verified.
	if verified, _ := claims["email_verified"].(bool); verified && u.Email != "" {
		email := strings.ToLower(u.Email)
		for _, allowed := range a.cfg.AllowedEmails {
			allowed = strings.ToLower(allowed)
			if email == allowed || (strings.HasPrefix(allowed, "@") && strings.HasSuffix(email, allowed)) {
				u.allowed = true
				return u, nil
			}
		}
	}
	if len(a.cfg.AllowedGroups) > 0 {
		for _, g := range groupsClaim(claims[a.cfg.GroupsClaim]) {
			for _, allowed := range a.cfg.AllowedGroups {
				if g == allowed {
					u.allowed = true
					return u, nil
				}
			}
		}
	}
	return u, nil
}

// groupsClaim accepts a list of strings or a single string.
func groupsClaim(v any) []string {
	switch g := v.(type) {
	case string:
		return []string{g}
	case []any:
		out := make([]string, 0, len(g))
		for _, item := range g {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// session returns the principal of a valid session cookie on r.
func (a *oidcAuth) session(r *http.Request) (principal, bool) {
	c, err := r.Cookie(a.sessionName)
	if err != nil {
		return principal{}, false
	}
	var s sessionClaims
	if !verifyCookie(a.sessionKey, c.Value, &s) || a.now().Unix() > s.Expires || s.Subject == "" {
		return principal{}, false
	}
	return principal{Method: dto.AuthMethodOIDC, Subject: s.Subject, Email: s.Email, Name: s.Name}, true
}

// clearSession expires the session cookie.
func (a *oidcAuth) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: a.sessionName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode})
}

// sessionCookieUsable says whether a session cookie may authenticate r. It
// never may for a cross-origin request (even an allowed origin: the API
// sends no Access-Control-Allow-Credentials), and a state-changing request
// must prove it is same-origin with an Origin header or Sec-Fetch-Site,
// which browsers always send on such requests and forms cannot forge.
func sessionCookieUsable(r *http.Request) bool {
	if crossOrigin(r) {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	if r.Header.Get("Origin") != "" {
		return true // crossOrigin already compared it with Host
	}
	return r.Header.Get("Sec-Fetch-Site") == "same-origin"
}

// safeNext keeps the post-login redirect on this server: a local absolute
// path, never "//host" or "/\host", which browsers treat as another site.
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") ||
		strings.ContainsAny(next, "\r\n\x00") || strings.HasPrefix(next, "/auth/") {
		return "/"
	}
	if u, err := url.Parse(next); err != nil || u.Host != "" || u.Scheme != "" {
		return "/"
	}
	return next
}

// randomString is 32 random bytes, base64url encoded.
func randomString() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error()) // never happens on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// signCookie encodes v as base64url(JSON) + "." + base64url(HMAC-SHA256).
func signCookie(key []byte, v any) (string, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// verifyCookie checks the signature of a signCookie value and decodes it.
func verifyCookie(key []byte, value string, v any) bool {
	body, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(body))
	if !hmac.Equal(got, mac.Sum(nil)) {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return false
	}
	return json.Unmarshal(payload, v) == nil
}

// authErrorPage is the small HTML page a failed sign-in ends on. The message
// is one of the fixed texts above; details only go to the log.
func authErrorPage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Go-RED sign-in</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:15vh auto;padding:0 1rem;line-height:1.5}a{color:inherit}</style></head>
<body><h1>Sign-in failed</h1><p>%s</p><p><a href="%s">Try again</a> &middot; <a href="/">Back to the editor</a></p></body></html>
`, html.EscapeString(message), oidcLoginPath)
}
