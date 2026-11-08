package dto

// Authentication methods reported by GET /api/auth/info and /api/auth/me.
const (
	// AuthMethodToken is the static bearer token (-auth-token).
	AuthMethodToken = "token"
	// AuthMethodOIDC is editor sign-in through an OpenID Connect provider.
	AuthMethodOIDC = "oidc"
	// AuthMethodNone means the server runs without authentication.
	AuthMethodNone = "none"
)

// AuthInfo is the public answer of GET /api/auth/info: how a client can
// authenticate against this server.
type AuthInfo struct {
	// Methods lists the accepted methods ("token", "oidc"); empty when the
	// server needs no authentication.
	Methods []string `json:"methods"`
	// LoginURL starts the OIDC sign-in (append ?next=/path to come back
	// there); set only when "oidc" is among the methods.
	LoginURL string `json:"loginUrl,omitempty"`
}

// LogoutResponse is the answer of POST /api/auth/logout.
type LogoutResponse struct {
	// LogoutURL, when set, is where the browser goes next to end the
	// session at the OIDC provider too (RP-initiated logout); the provider
	// sends it back to the editor afterwards.
	LogoutURL string `json:"logoutUrl,omitempty"`
}

// AuthUser is the answer of GET /api/auth/me: who the request was
// authenticated as.
type AuthUser struct {
	// Method is "token", "oidc" or "none".
	Method string `json:"method"`
	// Subject is the provider's stable user ID (OIDC only).
	Subject string `json:"subject,omitempty"`
	// Email is the user's e-mail address, when the provider sent one.
	Email string `json:"email,omitempty"`
	// Name is a display name (name, preferred_username or email).
	Name string `json:"name,omitempty"`
}
