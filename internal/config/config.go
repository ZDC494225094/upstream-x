package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Admin    AdminConfig    `yaml:"admin"`
	Auth     AuthConfig     `yaml:"auth"`
	Security SecurityConfig `yaml:"security"`
	App      AppConfig      `yaml:"app"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
}

type DatabaseConfig struct {
	SQLitePath string `yaml:"sqlite_path"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type AuthConfig struct {
	SessionSecret        string        `yaml:"session_secret"`
	SessionMaxAge        time.Duration `yaml:"session_max_age"`
	SessionSameSite      string        `yaml:"session_same_site"`
	SessionSecure        *bool         `yaml:"session_secure"`
	AllowIframeEmbedding *bool         `yaml:"allow_iframe_embedding"`
	IframeAllowedOrigins []string      `yaml:"iframe_allowed_origins"`
}

type SecurityConfig struct {
	CredentialKey string `yaml:"credential_key"`
}

type SMTPConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Host       string   `yaml:"host"`
	Port       int      `yaml:"port"`
	UserName   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	From       string   `yaml:"from"`
	Recipients []string `yaml:"recipients"`
	UseTLS     bool     `yaml:"use_tls"`
}

type AppConfig struct {
	TimeZone string `yaml:"time_zone"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if strings.TrimSpace(path) != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		} else if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	if err := applyEnv(&cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if strings.TrimSpace(cfg.Database.SQLitePath) == "" {
		cfg.Database.SQLitePath = filepath.Join("data", "app.db")
	}
	if cfg.Auth.SessionMaxAge == 0 {
		cfg.Auth.SessionMaxAge = 7 * 24 * time.Hour
	}
	if strings.TrimSpace(cfg.Auth.SessionSameSite) == "" {
		cfg.Auth.SessionSameSite = "none"
	}
	if cfg.App.TimeZone == "" {
		cfg.App.TimeZone = "Asia/Shanghai"
	}
}

func (c *Config) Validate() error {
	sameSite := strings.ToLower(strings.TrimSpace(c.Auth.SessionSameSite))
	secure := c.Auth.SessionSecureEnabled()
	switch {
	case c.Admin.Username == "":
		return errors.New("admin.username is required")
	case c.Admin.Password == "":
		return errors.New("admin.password is required")
	case len(c.Auth.SessionSecret) < 16:
		return errors.New("auth.session_secret must be at least 16 characters")
	case sameSite != "lax" && sameSite != "strict" && sameSite != "none":
		return errors.New("auth.session_same_site must be one of: lax, strict, none")
	case sameSite == "none" && c.Auth.SessionSecure != nil && !secure:
		return errors.New("auth.session_secure must be true when auth.session_same_site is none")
	case len(c.Security.CredentialKey) < 32:
		return errors.New("security.credential_key must be at least 32 characters")
	}

	return nil
}

func applyEnv(cfg *Config) error {
	if value, ok := lookupEnv("SERVER_PORT"); ok {
		cfg.Server.Port = value
	}
	if value, ok := lookupEnv("DATABASE_SQLITE_PATH"); ok {
		cfg.Database.SQLitePath = value
	}
	if value, ok := lookupEnv("ADMIN_USERNAME"); ok {
		cfg.Admin.Username = value
	}
	if value, ok := lookupEnv("ADMIN_PASSWORD"); ok {
		cfg.Admin.Password = value
	}
	if value, ok := lookupEnv("AUTH_SESSION_SECRET"); ok {
		cfg.Auth.SessionSecret = value
	}
	if value, ok := lookupEnv("AUTH_SESSION_MAX_AGE"); ok {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse AUTH_SESSION_MAX_AGE: %w", err)
		}
		cfg.Auth.SessionMaxAge = duration
	}
	if value, ok := lookupEnv("AUTH_SESSION_SAME_SITE"); ok {
		cfg.Auth.SessionSameSite = value
	}
	if value, ok := lookupEnv("AUTH_SESSION_SECURE"); ok {
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("parse AUTH_SESSION_SECURE: %w", err)
		}
		cfg.Auth.SessionSecure = &parsed
	}
	if value, ok := lookupEnv("AUTH_ALLOW_IFRAME_EMBEDDING"); ok {
		parsed, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("parse AUTH_ALLOW_IFRAME_EMBEDDING: %w", err)
		}
		cfg.Auth.AllowIframeEmbedding = &parsed
	}
	if value, ok := lookupEnv("AUTH_IFRAME_ALLOWED_ORIGINS"); ok {
		cfg.Auth.IframeAllowedOrigins = splitCSV(value)
	}
	if value, ok := lookupEnv("SECURITY_CREDENTIAL_KEY"); ok {
		cfg.Security.CredentialKey = value
	}
	if value, ok := lookupEnv("APP_TIME_ZONE"); ok {
		cfg.App.TimeZone = value
	} else if value, ok := lookupEnv("TZ"); ok {
		cfg.App.TimeZone = value
	}

	return nil
}

func lookupEnv(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func parseBool(value string) (bool, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func (c AuthConfig) SessionSecureEnabled() bool {
	return c.SessionSecure != nil && *c.SessionSecure
}

func (c AuthConfig) IframeEmbeddingEnabled() bool {
	return c.AllowIframeEmbedding == nil || *c.AllowIframeEmbedding
}
