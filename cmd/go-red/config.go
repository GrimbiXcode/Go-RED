package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the server configuration. Values come from, in rising
// precedence, the built-in defaults, an optional YAML file (-config or
// GORED_CONFIG), GORED_* environment variables and command-line flags; see
// loadConfig. The yaml tags are the keys of the config file.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int `yaml:"port"`
	// DataDir holds the persisted flows (and their backups and quarantine).
	DataDir string `yaml:"dataDir"`
	// WebUIDir, when set, serves the editor from that directory instead of
	// the build embedded in the binary (a development convenience).
	WebUIDir string `yaml:"webDir"`
	// MaxInflight bounds concurrent node executions per flow; a flow's own
	// maxConcurrency overrides it.
	MaxInflight int `yaml:"maxInflight"`
	// MaxMessages is the message queue size per flow.
	MaxMessages int `yaml:"maxMessages"`
	// MessageLog is how many routed messages are kept for GET /api/messages
	// (0 disables the log).
	MessageLog int `yaml:"messageLog"`
	// LogLevel is debug, info, warn or error.
	LogLevel string `yaml:"logLevel"`
	// AuthToken, when set, is required as a bearer token on /api/*, /ws and
	// /metrics (health and version stay public).
	AuthToken string `yaml:"authToken"`
	// AllowedOrigins lists browser origins (scheme://host[:port], or "*")
	// that may call the API and open the WebSocket from another site; the
	// editor's own origin is always allowed.
	AllowedOrigins []string `yaml:"allowedOrigins"`
	// RateLimit caps import and deploy requests per client address and
	// minute; 0 disables the limit.
	RateLimit int `yaml:"rateLimit"`
	// BackupKeep is how many backups of a flow file are kept (0 disables).
	BackupKeep int `yaml:"backupKeep"`
	// BackupInterval is the minimum age of the newest backup before a save
	// makes another one.
	BackupInterval time.Duration `yaml:"backupInterval"`

	// OIDCIssuer, when set, turns on editor sign-in through an OpenID
	// Connect provider (authorization code flow with PKCE); the other
	// OIDC* fields configure it. See docs/PROTOCOL.md.
	OIDCIssuer string `yaml:"oidcIssuer"`
	// OIDCClientID is the client registered at the provider.
	OIDCClientID string `yaml:"oidcClientId"`
	// OIDCClientSecret is the client's secret (empty for a public client).
	OIDCClientSecret string `yaml:"oidcClientSecret"`
	// OIDCRedirectURL is the callback the provider redirects to, as the
	// browser sees it: scheme://host[:port]/auth/callback.
	OIDCRedirectURL string `yaml:"oidcRedirectUrl"`
	// OIDCScopes are requested in addition to "openid".
	OIDCScopes []string `yaml:"oidcScopes"`
	// OIDCAllowedEmails lists who may sign in: verified e-mail addresses, or
	// "@domain" for a whole domain.
	OIDCAllowedEmails []string `yaml:"oidcAllowedEmails"`
	// OIDCAllowedGroups lets members of these groups (read from the ID
	// token claim OIDCGroupsClaim) sign in.
	OIDCAllowedGroups []string `yaml:"oidcAllowedGroups"`
	// OIDCGroupsClaim is the ID token claim holding the user's groups.
	OIDCGroupsClaim string `yaml:"oidcGroupsClaim"`
	// OIDCAllowAnyUser lets every account the provider authenticates sign
	// in; without it an allowlist (emails or groups) is required.
	OIDCAllowAnyUser bool `yaml:"oidcAllowAnyUser"`
	// OIDCLogout is what signing out does: "provider" also ends the session
	// at the provider when it has an end-session endpoint (RP-initiated
	// logout), "local" only ends Go-RED's.
	OIDCLogout string `yaml:"oidcLogout"`
	// SessionSecret signs editor session cookies (at least 32 characters).
	// Empty means a random key per process: sessions end on restart.
	SessionSecret string `yaml:"sessionSecret"`
	// SessionTTL is how long a sign-in lasts.
	SessionTTL time.Duration `yaml:"sessionTtl"`
}

// flagOutput receives flag errors and the -help text (tests replace it).
var flagOutput io.Writer = os.Stderr

// secretFlag registers a flag for a secret. Unlike the other flags it does
// not show the value so far (from the config file or environment) as its
// default, so -help and the usage text printed on a mistyped flag, which
// often end up in service logs, never contain the secret.
func secretFlag(fs *flag.FlagSet, dst *string, name, usage string) {
	fs.Func(name, usage, func(v string) error {
		*dst = v
		return nil
	})
}

// defaultConfig is the configuration without any file, env or flag.
func defaultConfig() Config {
	return Config{
		Port:            8080,
		DataDir:         "data",
		MaxInflight:     1024,
		MaxMessages:     1000,
		MessageLog:      0,
		LogLevel:        "info",
		RateLimit:       60,
		BackupKeep:      5,
		BackupInterval:  10 * time.Minute,
		OIDCScopes:      []string{"profile", "email"},
		OIDCGroupsClaim: "groups",
		OIDCLogout:      oidcLogoutProvider,
		SessionTTL:      12 * time.Hour,
	}
}

// loadConfig builds the configuration from args (the command line without
// the program name) and getenv (usually os.Getenv). It returns the config,
// whether -version was asked for, and any parse or validation error.
func loadConfig(args []string, getenv func(string) string) (Config, bool, error) {
	cfg := defaultConfig()

	// 1. Config file: -config on the command line wins over GORED_CONFIG.
	file := getenv("GORED_CONFIG")
	if fromArgs := configFileFromArgs(args); fromArgs != "" {
		file = fromArgs
	}
	if file != "" {
		if err := loadConfigFile(file, &cfg); err != nil {
			return cfg, false, err
		}
	}

	// 2. Environment.
	if err := applyEnv(&cfg, getenv); err != nil {
		return cfg, false, err
	}

	// 3. Flags, whose defaults are the values so far, so an unset flag
	// keeps what file and environment said.
	fs := flag.NewFlagSet("go-red", flag.ContinueOnError)
	fs.SetOutput(flagOutput)
	fs.String("config", file, "YAML config file (keys: port, dataDir, webDir, maxInflight, maxMessages, messageLog, logLevel, authToken, allowedOrigins, rateLimit, backupKeep, backupInterval, oidcIssuer, oidcClientId, oidcClientSecret, oidcRedirectUrl, oidcScopes, oidcAllowedEmails, oidcAllowedGroups, oidcGroupsClaim, oidcAllowAnyUser, oidcLogout, sessionSecret, sessionTtl)")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Directory for flow data")
	fs.StringVar(&cfg.WebUIDir, "web-dir", cfg.WebUIDir, "Serve the editor from this directory instead of the embedded build (development)")
	fs.IntVar(&cfg.MaxInflight, "max-inflight", cfg.MaxInflight, "Maximum concurrent node executions per flow (a flow's maxConcurrency overrides it)")
	fs.IntVar(&cfg.MaxMessages, "max-messages", cfg.MaxMessages, "Message queue size per flow")
	fs.IntVar(&cfg.MessageLog, "message-log", cfg.MessageLog, "Number of routed messages to keep for GET /api/messages (0 disables)")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: debug, info, warn, error")
	secretFlag(fs, &cfg.AuthToken, "auth-token", "Bearer token required on /api/*, /ws and /metrics (prefer GORED_AUTH_TOKEN or the config file; empty: no authentication)")
	origins := fs.String("allowed-origins", strings.Join(cfg.AllowedOrigins, ","), "Comma-separated browser origins allowed to use the API cross-site, or * (the editor's own origin is always allowed)")
	fs.IntVar(&cfg.RateLimit, "rate-limit", cfg.RateLimit, "Import and deploy requests allowed per client and minute (0 disables)")
	fs.IntVar(&cfg.BackupKeep, "backup-keep", cfg.BackupKeep, "Backups to keep per flow file (0 disables)")
	fs.DurationVar(&cfg.BackupInterval, "backup-interval", cfg.BackupInterval, "Minimum time between two backups of the same flow")
	fs.StringVar(&cfg.OIDCIssuer, "oidc-issuer", cfg.OIDCIssuer, "OpenID Connect issuer URL; turns on editor sign-in (empty: off)")
	fs.StringVar(&cfg.OIDCClientID, "oidc-client-id", cfg.OIDCClientID, "OpenID Connect client ID")
	secretFlag(fs, &cfg.OIDCClientSecret, "oidc-client-secret", "OpenID Connect client secret (prefer GORED_OIDC_CLIENT_SECRET or the config file)")
	fs.StringVar(&cfg.OIDCRedirectURL, "oidc-redirect-url", cfg.OIDCRedirectURL, "Callback URL as the browser sees it: scheme://host[:port]/auth/callback")
	scopes := fs.String("oidc-scopes", strings.Join(cfg.OIDCScopes, ","), "Comma-separated scopes requested besides openid")
	emails := fs.String("oidc-allowed-emails", strings.Join(cfg.OIDCAllowedEmails, ","), "Comma-separated verified e-mail addresses (or @domain) allowed to sign in")
	groups := fs.String("oidc-allowed-groups", strings.Join(cfg.OIDCAllowedGroups, ","), "Comma-separated groups allowed to sign in")
	fs.StringVar(&cfg.OIDCGroupsClaim, "oidc-groups-claim", cfg.OIDCGroupsClaim, "ID token claim that holds the user's groups")
	fs.BoolVar(&cfg.OIDCAllowAnyUser, "oidc-allow-any-user", cfg.OIDCAllowAnyUser, "Let every account the provider authenticates sign in (no allowlist)")
	fs.StringVar(&cfg.OIDCLogout, "oidc-logout", cfg.OIDCLogout, "What signing out ends: provider (also the provider's session, when it supports that) or local (only Go-RED's)")
	secretFlag(fs, &cfg.SessionSecret, "session-secret", "Key that signs session cookies, at least 32 characters (prefer GORED_SESSION_SECRET or the config file; empty: random, sessions end on restart)")
	fs.DurationVar(&cfg.SessionTTL, "session-ttl", cfg.SessionTTL, "How long an editor sign-in lasts")
	showVersion := fs.Bool("version", false, "Print the version and exit")
	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}
	cfg.AllowedOrigins = splitList(*origins)
	cfg.OIDCScopes = splitList(*scopes)
	cfg.OIDCAllowedEmails = splitList(*emails)
	cfg.OIDCAllowedGroups = splitList(*groups)

	if err := cfg.validate(); err != nil {
		return cfg, false, err
	}
	return cfg, *showVersion, nil
}

// configFileFromArgs finds -config in args without parsing the rest, so the
// file can be read before the other flags are given their defaults.
func configFileFromArgs(args []string) string {
	for i, arg := range args {
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || name != "config" {
			continue
		}
		if hasValue {
			return value
		}
		if i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// loadConfigFile decodes a YAML file into cfg; keys it does not know are
// an error, so a typo never passes silently as "default".
func loadConfigFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config file: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("config file %s: %w", path, err)
	}
	return nil
}

// applyEnv overrides cfg with every GORED_* variable that is set.
func applyEnv(cfg *Config, getenv func(string) string) error {
	setInt := func(dst *int) func(string) error {
		return func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return err
			}
			*dst = n
			return nil
		}
	}
	setString := func(dst *string) func(string) error {
		return func(v string) error {
			*dst = v
			return nil
		}
	}
	setList := func(dst *[]string) func(string) error {
		return func(v string) error {
			*dst = splitList(v)
			return nil
		}
	}
	setDuration := func(dst *time.Duration) func(string) error {
		return func(v string) error {
			d, err := time.ParseDuration(v)
			if err != nil {
				return err
			}
			*dst = d
			return nil
		}
	}
	vars := []struct {
		key   string
		apply func(string) error
	}{
		{"GORED_PORT", setInt(&cfg.Port)},
		{"GORED_DATA_DIR", setString(&cfg.DataDir)},
		{"GORED_WEB_DIR", setString(&cfg.WebUIDir)},
		{"GORED_MAX_INFLIGHT", setInt(&cfg.MaxInflight)},
		{"GORED_MAX_MESSAGES", setInt(&cfg.MaxMessages)},
		{"GORED_MESSAGE_LOG", setInt(&cfg.MessageLog)},
		{"GORED_LOG_LEVEL", setString(&cfg.LogLevel)},
		{"GORED_AUTH_TOKEN", setString(&cfg.AuthToken)},
		{"GORED_ALLOWED_ORIGINS", setList(&cfg.AllowedOrigins)},
		{"GORED_RATE_LIMIT", setInt(&cfg.RateLimit)},
		{"GORED_BACKUP_KEEP", setInt(&cfg.BackupKeep)},
		{"GORED_BACKUP_INTERVAL", setDuration(&cfg.BackupInterval)},
		{"GORED_OIDC_ISSUER", setString(&cfg.OIDCIssuer)},
		{"GORED_OIDC_CLIENT_ID", setString(&cfg.OIDCClientID)},
		{"GORED_OIDC_CLIENT_SECRET", setString(&cfg.OIDCClientSecret)},
		{"GORED_OIDC_REDIRECT_URL", setString(&cfg.OIDCRedirectURL)},
		{"GORED_OIDC_SCOPES", setList(&cfg.OIDCScopes)},
		{"GORED_OIDC_ALLOWED_EMAILS", setList(&cfg.OIDCAllowedEmails)},
		{"GORED_OIDC_ALLOWED_GROUPS", setList(&cfg.OIDCAllowedGroups)},
		{"GORED_OIDC_GROUPS_CLAIM", setString(&cfg.OIDCGroupsClaim)},
		{"GORED_OIDC_LOGOUT", setString(&cfg.OIDCLogout)},
		{"GORED_OIDC_ALLOW_ANY_USER", func(v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return err
			}
			cfg.OIDCAllowAnyUser = b
			return nil
		}},
		{"GORED_SESSION_SECRET", setString(&cfg.SessionSecret)},
		{"GORED_SESSION_TTL", setDuration(&cfg.SessionTTL)},
	}
	for _, v := range vars {
		value := getenv(v.key)
		if value == "" {
			continue
		}
		if err := v.apply(value); err != nil {
			return fmt.Errorf("%s: %w", v.key, err)
		}
	}
	return nil
}

// splitList turns a comma-separated list into its trimmed, non-empty items.
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// validate rejects values the server could not run with.
func (c Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.MaxInflight < 1 {
		return fmt.Errorf("max-inflight must be at least 1, got %d", c.MaxInflight)
	}
	if c.MaxMessages < 1 {
		return fmt.Errorf("max-messages must be at least 1, got %d", c.MaxMessages)
	}
	if c.MessageLog < 0 || c.RateLimit < 0 || c.BackupKeep < 0 || c.BackupInterval < 0 {
		return errors.New("message-log, rate-limit, backup-keep and backup-interval must not be negative")
	}
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "warning", "error":
	default:
		return fmt.Errorf("log-level must be debug, info, warn or error, got %q", c.LogLevel)
	}
	if err := validateToken(c.AuthToken); err != nil {
		return err
	}
	for _, origin := range c.AllowedOrigins {
		if origin == "*" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" || u.Path != "" {
			return fmt.Errorf("allowed origin %q must look like scheme://host[:port]", origin)
		}
	}
	return c.validateOIDC()
}

// validateOIDC checks the sign-in settings. Everything is optional while
// OIDCIssuer is empty, except that a session secret must still be usable.
func (c Config) validateOIDC() error {
	if c.SessionSecret != "" && len(c.SessionSecret) < 32 {
		return errors.New("session-secret must be at least 32 characters")
	}
	if c.OIDCLogout != oidcLogoutProvider && c.OIDCLogout != oidcLogoutLocal {
		return fmt.Errorf("oidc-logout must be %s or %s, got %q", oidcLogoutProvider, oidcLogoutLocal, c.OIDCLogout)
	}
	if c.OIDCIssuer == "" {
		if c.OIDCClientID != "" || c.OIDCRedirectURL != "" {
			return errors.New("oidc-client-id and oidc-redirect-url need oidc-issuer")
		}
		return nil
	}
	if u, err := url.Parse(c.OIDCIssuer); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("oidc-issuer %q must be an http(s) URL", c.OIDCIssuer)
	}
	if c.OIDCClientID == "" {
		return errors.New("oidc-issuer needs oidc-client-id")
	}
	u, err := url.Parse(c.OIDCRedirectURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != oidcCallbackPath || u.RawQuery != "" {
		return fmt.Errorf("oidc-redirect-url must look like scheme://host[:port]%s, got %q", oidcCallbackPath, c.OIDCRedirectURL)
	}
	if !c.OIDCAllowAnyUser && len(c.OIDCAllowedEmails) == 0 && len(c.OIDCAllowedGroups) == 0 {
		return errors.New("oidc-issuer needs oidc-allowed-emails or oidc-allowed-groups (or oidc-allow-any-user to let every account of the provider in)")
	}
	if len(c.OIDCAllowedGroups) > 0 && c.OIDCGroupsClaim == "" {
		return errors.New("oidc-allowed-groups needs oidc-groups-claim")
	}
	for _, e := range c.OIDCAllowedEmails {
		if !strings.Contains(e, "@") {
			return fmt.Errorf("oidc-allowed-emails entry %q must be an address or @domain", e)
		}
	}
	if c.SessionTTL < time.Minute {
		return errors.New("session-ttl must be at least 1m")
	}
	return nil
}
