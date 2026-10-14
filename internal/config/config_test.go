package config

import "testing"

func TestValidateAllowsIframeCookieConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Admin: AdminConfig{
			Username: "admin",
			Password: "secret",
		},
		Auth: AuthConfig{
			SessionSecret:   "1234567890abcdef",
			SessionSameSite: "none",
			SessionSecure:   boolPtr(true),
		},
		Security: SecurityConfig{
			CredentialKey: "12345678901234567890123456789012",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsSameSiteNoneWithoutSecure(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Admin: AdminConfig{
			Username: "admin",
			Password: "secret",
		},
		Auth: AuthConfig{
			SessionSecret:   "1234567890abcdef",
			SessionSameSite: "none",
			SessionSecure:   boolPtr(false),
		},
		Security: SecurityConfig{
			CredentialKey: "12345678901234567890123456789012",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestValidateAllowsDefaultSecureInference(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Admin: AdminConfig{
			Username: "admin",
			Password: "secret",
		},
		Auth: AuthConfig{
			SessionSecret:   "1234567890abcdef",
			SessionSameSite: "none",
		},
		Security: SecurityConfig{
			CredentialKey: "12345678901234567890123456789012",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
