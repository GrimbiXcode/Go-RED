package main

import (
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
		"unknown yaml key":      {yaml: "prot: 1\n"},
		"bad env int":           {env: map[string]string{"GORED_PORT": "eighty"}},
		"bad env duration":      {env: map[string]string{"GORED_BACKUP_INTERVAL": "soon"}},
		"port out of range":     {args: []string{"-port", "70000"}},
		"short token":           {args: []string{"-auth-token", "abc"}},
		"token with slashes":    {args: []string{"-auth-token", "abc/def/ghi/jkl/mno"}},
		"origin without scheme": {args: []string{"-allowed-origins", "app.example"}},
		"origin with path":      {args: []string{"-allowed-origins", "http://app.example/api"}},
		"bad log level":         {args: []string{"-log-level", "loud"}},
		"negative backups":      {args: []string{"-backup-keep", "-1"}},
		"unknown flag":          {args: []string{"-max-workers", "3"}},
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
