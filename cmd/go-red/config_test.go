package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadConfigPrecedence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "go-red.yaml")
	require.NoError(t, os.WriteFile(file, []byte(`
port: 9000
authToken: file-token-1234567890
allowedOrigins: [http://a.example, http://b.example]
backupInterval: 5m
rateLimit: 5
`), 0o644))

	cfg, showVersion, err := loadConfig([]string{"-config", file, "-port", "9002", "-log-level", "debug"}, envOf(map[string]string{
		"GORED_PORT":         "9001",
		"GORED_RATE_LIMIT":   "7",
		"GORED_MAX_MESSAGES": "50",
	}))
	require.NoError(t, err)
	assert.False(t, showVersion)
	assert.Equal(t, 9002, cfg.Port, "flag beats env and file")
	assert.Equal(t, 7, cfg.RateLimit, "env beats file")
	assert.Equal(t, 50, cfg.MaxMessages, "env beats default")
	assert.Equal(t, "file-token-1234567890", cfg.AuthToken, "file beats default")
	assert.Equal(t, []string{"http://a.example", "http://b.example"}, cfg.AllowedOrigins)
	assert.Equal(t, 5*time.Minute, cfg.BackupInterval)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, defaultConfig().DataDir, cfg.DataDir, "untouched values keep their default")

	cfg, _, err = loadConfig(nil, envOf(map[string]string{"GORED_CONFIG": file, "GORED_ALLOWED_ORIGINS": " http://c.example ,*"}))
	require.NoError(t, err)
	assert.Equal(t, 9000, cfg.Port, "GORED_CONFIG names the file")
	assert.Equal(t, []string{"http://c.example", "*"}, cfg.AllowedOrigins)

	cfg, _, err = loadConfig([]string{"-config=" + file, "-allowed-origins", "http://d.example"}, envOf(nil))
	require.NoError(t, err)
	assert.Equal(t, []string{"http://d.example"}, cfg.AllowedOrigins, "the flag replaces the file's list")

	_, showVersion, err = loadConfig([]string{"-version"}, envOf(nil))
	require.NoError(t, err)
	assert.True(t, showVersion)
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	cases := map[string]struct {
		args []string
		env  map[string]string
		yaml string
	}{
		"unknown yaml key":         {yaml: "prot: 1\n"},
		"bad env int":              {env: map[string]string{"GORED_PORT": "eighty"}},
		"bad env duration":         {env: map[string]string{"GORED_BACKUP_INTERVAL": "soon"}},
		"port out of range":        {args: []string{"-port", "70000"}},
		"short token":              {args: []string{"-auth-token", "abc"}},
		"token with slashes":       {args: []string{"-auth-token", "abc/def/ghi/jkl/mno"}},
		"origin without scheme":    {args: []string{"-allowed-origins", "app.example"}},
		"origin with path":         {args: []string{"-allowed-origins", "http://app.example/api"}},
		"bad log level":            {args: []string{"-log-level", "loud"}},
		"negative backups":         {args: []string{"-backup-keep", "-1"}},
		"unknown flag":             {args: []string{"-max-workers", "3"}},
		"short session secret":     {args: []string{"-session-secret", "too-short"}},
		"oidc without client":      {args: []string{"-oidc-issuer", "https://idp.example", "-oidc-redirect-url", "https://red.example/auth/callback", "-oidc-allow-any-user"}},
		"oidc without redirect":    {args: []string{"-oidc-issuer", "https://idp.example", "-oidc-client-id", "c", "-oidc-allow-any-user"}},
		"oidc redirect path":       {args: []string{"-oidc-issuer", "https://idp.example", "-oidc-client-id", "c", "-oidc-redirect-url", "https://red.example/callback", "-oidc-allow-any-user"}},
		"oidc issuer scheme":       {args: []string{"-oidc-issuer", "idp.example", "-oidc-client-id", "c", "-oidc-redirect-url", "https://red.example/auth/callback", "-oidc-allow-any-user"}},
		"oidc without allowlist":   {args: []string{"-oidc-issuer", "https://idp.example", "-oidc-client-id", "c", "-oidc-redirect-url", "https://red.example/auth/callback"}},
		"oidc bad email entry":     {args: []string{"-oidc-issuer", "https://idp.example", "-oidc-client-id", "c", "-oidc-redirect-url", "https://red.example/auth/callback", "-oidc-allowed-emails", "example.com"}},
		"client id without issuer": {args: []string{"-oidc-client-id", "c"}},
		"bad allow-any env":        {env: map[string]string{"GORED_OIDC_ALLOW_ANY_USER": "maybe"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			args := tc.args
			if tc.yaml != "" {
				file := filepath.Join(t.TempDir(), "c.yaml")
				require.NoError(t, os.WriteFile(file, []byte(tc.yaml), 0o644))
				args = append([]string{"-config", file}, args...)
			}
			_, _, err := loadConfig(args, envOf(tc.env))
			assert.Error(t, err)
		})
	}
}

func TestDefaultConfigIsValid(t *testing.T) {
	cfg, _, err := loadConfig(nil, envOf(nil))
	require.NoError(t, err)
	assert.Equal(t, defaultConfig(), cfg)
	assert.NoError(t, cfg.validate())
}

func TestLoadConfigOIDC(t *testing.T) {
	file := filepath.Join(t.TempDir(), "go-red.yaml")
	require.NoError(t, os.WriteFile(file, []byte(`
oidcIssuer: https://idp.example/realms/red
oidcClientId: go-red
oidcRedirectUrl: https://red.example/auth/callback
oidcAllowedEmails: [ada@example.com]
sessionTtl: 2h
`), 0o644))
	cfg, _, err := loadConfig([]string{"-config", file, "-oidc-allowed-groups", "admins,ops"}, envOf(map[string]string{
		"GORED_OIDC_CLIENT_SECRET": "from-env",
		"GORED_SESSION_SECRET":     "0123456789abcdef0123456789abcdef",
		"GORED_OIDC_SCOPES":        "email",
	}))
	require.NoError(t, err)
	settings, ok := cfg.oidcSettings()
	require.True(t, ok)
	assert.Equal(t, oidcSettings{
		Issuer:        "https://idp.example/realms/red",
		ClientID:      "go-red",
		ClientSecret:  "from-env",
		RedirectURL:   "https://red.example/auth/callback",
		Scopes:        []string{"email"},
		AllowedEmails: []string{"ada@example.com"},
		AllowedGroups: []string{"admins", "ops"},
		GroupsClaim:   "groups",
		SessionSecret: "0123456789abcdef0123456789abcdef",
		SessionTTL:    2 * time.Hour,
	}, settings)

	_, ok = defaultConfig().oidcSettings()
	assert.False(t, ok, "sign-in is off by default")
}

func TestSecretsStayOutOfUsageText(t *testing.T) {
	var out bytes.Buffer
	flagOutput = &out
	t.Cleanup(func() { flagOutput = os.Stderr })
	env := envOf(map[string]string{
		"GORED_AUTH_TOKEN":         "env-auth-token-0123456789",
		"GORED_OIDC_CLIENT_SECRET": "env-client-secret",
		"GORED_SESSION_SECRET":     "env-session-secret-0123456789abcdef0123",
	})

	for _, args := range [][]string{{"-help"}, {"-no-such-flag"}} {
		out.Reset()
		_, _, err := loadConfig(args, env)
		require.Error(t, err)
		usage := out.String()
		assert.Contains(t, usage, "-oidc-client-secret", "the flags are still listed")
		for _, secret := range []string{"env-auth-token", "env-client-secret", "env-session-secret"} {
			assert.NotContains(t, usage, secret, "%v prints %s", args, secret)
		}
	}

	// The secret flags still work, and beat the environment.
	cfg, _, err := loadConfig([]string{"-auth-token", "flag-auth-token-0123456789", "-session-secret", "flag-session-secret-0123456789abcdef01"}, env)
	require.NoError(t, err)
	assert.Equal(t, "flag-auth-token-0123456789", cfg.AuthToken)
	assert.Equal(t, "flag-session-secret-0123456789abcdef01", cfg.SessionSecret)
	assert.Equal(t, "env-client-secret", cfg.OIDCClientSecret, "an unset secret flag keeps the environment's value")
}
