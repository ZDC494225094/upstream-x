package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"upstream-x/server/models"
	"upstream-x/server/upstream"
)

// Simulates the current Sub2API protocol: refresh tokens are single-use and
// rotate on every refresh; presenting a burned token yields
// 401 REFRESH_TOKEN_INVALID.
func TestSub2APIRefreshRotationReturnsNewCredentials(t *testing.T) {
	t.Parallel()

	var currentRefresh atomic.Value
	currentRefresh.Store("rt_old")
	var refreshCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.RefreshToken != currentRefresh.Load().(string) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":401,"message":"invalid refresh token","reason":"REFRESH_TOKEN_INVALID"}`))
				return
			}
			refreshCalls.Add(1)
			next := "rt_new"
			currentRefresh.Store(next)
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"fresh-access","refresh_token":"rt_new"}}`))
		case "/api/v1/auth/me":
			if r.Header.Get("Authorization") != "Bearer fresh-access" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":42}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[],"pages":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	account := &models.UpstreamAccount{
		Type:            models.UpstreamTypeSub2API,
		Sub2APIAuthMode: models.Sub2APIAuthModeRefreshToken,
	}
	creds := upstream.AccountCredentials{
		BaseURL:         server.URL,
		Type:            models.UpstreamTypeSub2API,
		Sub2APIAuthMode: models.Sub2APIAuthModeRefreshToken,
		RefreshToken:    "rt_old",
	}
	detector := &DetectorService{client: upstream.NewClient()}

	snapshot, used, err := detector.fetchSnapshot(context.Background(), account, creds)
	if err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.BalanceUSD != 42 {
		t.Fatalf("balance = %v, want 42", snapshot.BalanceUSD)
	}
	if used == nil || used.RefreshToken != "rt_new" || used.BearerToken != "fresh-access" {
		t.Fatalf("used credentials = %+v, want rotated rt_new/fresh-access", used)
	}

	// The burned token must now fail, with the upstream error returned
	// verbatim (raw body, no reinterpretation).
	stale := creds
	_, _, err = detector.fetchSnapshot(context.Background(), account, stale)
	if err == nil || !strings.Contains(err.Error(), "REFRESH_TOKEN_INVALID") {
		t.Fatalf("stale fetch error = %v, want the raw REFRESH_TOKEN_INVALID body", err)
	}
	if !strings.Contains(err.Error(), `"message":"invalid refresh token"`) {
		t.Fatalf("stale fetch error should carry the upstream body verbatim, got: %v", err)
	}
}

func TestSub2APIRefreshRejectsAPIKey(t *testing.T) {
	t.Parallel()

	creds := upstream.AccountCredentials{
		BaseURL:         "http://127.0.0.1:1",
		Type:            models.UpstreamTypeSub2API,
		Sub2APIAuthMode: models.Sub2APIAuthModeRefreshToken,
		RefreshToken:    "sk-not-a-refresh-token",
	}
	client := upstream.NewClient()
	_, err := client.RefreshSub2APIToken(context.Background(), creds)
	if err == nil || !strings.Contains(err.Error(), "API Key") {
		t.Fatalf("refresh error = %v, want API key guidance", err)
	}
}
