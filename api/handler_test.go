package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestHandlerEndToEnd exercises the serverless entrypoint against a real
// Postgres database. It is skipped unless DATABASE_URL is set.
func TestHandlerEndToEnd(t *testing.T) {
	if os.Getenv("DATABASE_URL") == "" {
		t.Skip("DATABASE_URL not set")
	}
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "test-password-123")
	t.Setenv("AUTH_SESSION_SECRET", "test-session-secret-0123456789abcdef")
	t.Setenv("SECURITY_CREDENTIAL_KEY", "test-credential-key-0123456789abcdef")
	t.Setenv("CRON_SECRET", "test-cron-secret")

	// healthz
	rec := httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz: got %d, body %s", rec.Code, rec.Body.String())
	}

	// login with wrong password
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	Handler(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong login: got %d", rec.Code)
	}

	// login with correct password
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"test-password-123"}`))
	req.Header.Set("Content-Type", "application/json")
	Handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: got %d, body %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("login: no session cookie set")
	}

	// cron endpoint rejects missing secret
	rec = httptest.NewRecorder()
	Handler(rec, httptest.NewRequest(http.MethodGet, "/api/internal/check-all", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("cron without secret: got %d", rec.Code)
	}

	// cron endpoint runs a check pass with the secret (no accounts -> ok)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/internal/check-all", nil)
	req.Header.Set("Authorization", "Bearer test-cron-secret")
	Handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cron with secret: got %d, body %s", rec.Code, rec.Body.String())
	}
}
