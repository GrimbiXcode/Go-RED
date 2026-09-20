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
	// WebUIDir is the directory with the built editor (web/dist).
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
}

// defaultConfig is the configuration without any file, env or flag.
func defaultConfig() Config {
	return Config{
		Port:           8080,
		DataDir:        "data",
		WebUIDir:       "web/dist",
		MaxInflight:    1024,
		MaxMessages:    1000,
		MessageLog:     0,
		LogLevel:       "info",
		RateLimit:      60,
		BackupKeep:     5,
		BackupInterval: 10 * time.Minute,
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
	fs.SetOutput(os.Stderr)
	fs.String("config", file, "YAML config file (keys: port, dataDir, webDir, maxInflight, maxMessages, messageLog, logLevel, authToken, allowedOrigins, rateLimit, backupKeep, backupInterval)")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "Port to listen on")
	fs.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Directory for flow data")
	fs.StringVar(&cfg.WebUIDir, "web-dir", cfg.WebUIDir, "Directory for the built WebUI")
	fs.IntVar(&cfg.MaxInflight, "max-inflight", cfg.MaxInflight, "Maximum concurrent node executions per flow (a flow's maxConcurrency overrides it)")
	fs.IntVar(&cfg.MaxMessages, "max-messages", cfg.MaxMessages, "Message queue size per flow")
	fs.IntVar(&cfg.MessageLog, "message-log", cfg.MessageLog, "Number of routed messages to keep for GET /api/messages (0 disables)")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: debug, info, warn, error")
	fs.StringVar(&cfg.AuthToken, "auth-token", cfg.AuthToken, "Bearer token required on /api/*, /ws and /metrics (empty: no authentication)")
	origins := fs.String("allowed-origins", strings.Join(cfg.AllowedOrigins, ","), "Comma-separated browser origins allowed to use the API cross-site, or * (the editor's own origin is always allowed)")
	fs.IntVar(&cfg.RateLimit, "rate-limit", cfg.RateLimit, "Import and deploy requests allowed per client and minute (0 disables)")
	fs.IntVar(&cfg.BackupKeep, "backup-keep", cfg.BackupKeep, "Backups to keep per flow file (0 disables)")
	fs.DurationVar(&cfg.BackupInterval, "backup-interval", cfg.BackupInterval, "Minimum time between two backups of the same flow")
	showVersion := fs.Bool("version", false, "Print the version and exit")
	if err := fs.Parse(args); err != nil {
		return cfg, false, err
	}
	cfg.AllowedOrigins = splitList(*origins)

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
		{"GORED_ALLOWED_ORIGINS", func(v string) error { cfg.AllowedOrigins = splitList(v); return nil }},
		{"GORED_RATE_LIMIT", setInt(&cfg.RateLimit)},
		{"GORED_BACKUP_KEEP", setInt(&cfg.BackupKeep)},
		{"GORED_BACKUP_INTERVAL", func(v string) error {
			d, err := time.ParseDuration(v)
			if err != nil {
				return err
			}
			cfg.BackupInterval = d
			return nil
		}},
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
	return nil
}
