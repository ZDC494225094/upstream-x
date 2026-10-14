package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"upstream-x/internal/models"
	"upstream-x/internal/upstream"
)

func TestSub2APIPasswordModeDoesNotReloginAfterNonAuthFailure(t *testing.T) {
	t.Parallel()

	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			loginCalls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"temporary failure"}`))
	}))
	defer server.Close()

	account, creds := passwordAccountForAuthTest(server.URL, "stale-access")
	detector := &DetectorService{client: upstream.NewClient()}
	_, _, err := detector.fetchSnapshot(context.Background(), account, creds)
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("fetch error = %v, want upstream status 500", err)
	}
	if got := loginCalls.Load(); got != 0 {
		t.Fatalf("login calls = %d, want 0 for a non-authentication failure", got)
	}
}

func TestSub2APIPasswordModeReloginsAfterUnauthorized(t *testing.T) {
	t.Parallel()

	var loginCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/settings/public":
			_, _ = w.Write([]byte(`{"code":0,"data":{"turnstile_enabled":false}}`))
		case "/api/v1/auth/login":
			loginCalls.Add(1)
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"fresh-access","refresh_token":"fresh-refresh"}}`))
		case "/api/v1/auth/me":
			if r.Header.Get("Authorization") != "Bearer fresh-access" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":12.5}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[],"pages":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	account, creds := passwordAccountForAuthTest(server.URL, "stale-access")
	detector := &DetectorService{client: upstream.NewClient()}
	snapshot, _, err := detector.fetchSnapshot(context.Background(), account, creds)
	if err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.BalanceUSD != 12.5 {
		t.Fatalf("balance = %v, want 12.5", snapshot.BalanceUSD)
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("login calls = %d, want 1", got)
	}
}

func passwordAccountForAuthTest(baseURL string, accessToken string) (*models.UpstreamAccount, upstream.AccountCredentials) {
	account := &models.UpstreamAccount{
		Type:            models.UpstreamTypeSub2API,
		Sub2APIAuthMode: models.Sub2APIAuthModePassword,
	}
	creds := upstream.AccountCredentials{
		BaseURL:         baseURL,
		Type:            models.UpstreamTypeSub2API,
		Sub2APIAuthMode: models.Sub2APIAuthModePassword,
		Sub2APIEmail:    "demo@example.com",
		Sub2APIPassword: "secret",
		BearerToken:     accessToken,
		RefreshToken:    "old-refresh",
	}
	return account, creds
}
