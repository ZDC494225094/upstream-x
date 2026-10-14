package upstream

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"upstream-x/internal/models"
)

func TestFetchSub2APIUsesModelPlazaCatalogWhenAvailable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":12.5}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":7,"name":"OpenAI","description":"default","platform":"openai","rate_multiplier":1.2}]}`))
		case "/api/v1/model-plaza":
			if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
				t.Fatalf("authorization = %q", got)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"description":"catalog","groups":[{"id":7,"name":"OpenAI","description":"plaza","platform":"openai","rate_multiplier":1.2,"models":[{"name":"gpt-5","platform":"openai"},{"name":"gpt-5-mini","platform":"openai"}]}]}}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[],"pages":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := NewClient().Fetch(context.Background(), AccountCredentials{
		BaseURL:     server.URL,
		BearerToken: "access-token",
		Type:        models.UpstreamTypeSub2API,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(snapshot.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(snapshot.Groups))
	}
	group := snapshot.Groups[0]
	if group.CatalogSource != "model_plaza" {
		t.Fatalf("catalog source = %q", group.CatalogSource)
	}
	if len(group.Models) != 2 || group.Models[0].Name != "gpt-5" {
		t.Fatalf("models = %#v", group.Models)
	}
	if len(snapshot.Tokens) != 0 {
		t.Fatalf("tokens = %d, want none", len(snapshot.Tokens))
	}
}

func TestFetchSub2APICanUsePlazaAndTrendWithoutKeyListing(t *testing.T) {
	t.Parallel()

	today := time.Now().In(time.Local).Format("2006-01-02")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":4}}`))
		case "/api/v1/model-plaza":
			_, _ = w.Write([]byte(`{"code":0,"data":{"groups":[{"id":9,"name":"Plaza","platform":"openai","rate_multiplier":1,"models":[{"name":"gpt-5","platform":"openai"}]}]}}`))
		case "/api/v1/groups/available":
			http.NotFound(w, r)
		case "/api/v1/keys":
			t.Fatalf("/api/v1/keys should not be requested when the daily trend is available")
		case "/api/v1/usage/dashboard/trend":
			_, _ = fmt.Fprintf(w, `{"code":0,"data":{"trend":[{"date":%q,"actual_cost":1.25}]}}`, today)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := NewClient().Fetch(context.Background(), AccountCredentials{
		BaseURL:     server.URL,
		BearerToken: "plaza-token",
		Type:        models.UpstreamTypeSub2API,
	})
	if err != nil {
		t.Fatalf("fetch sub2api: %v", err)
	}
	if !snapshot.DailyUsageEnabled || len(snapshot.DailyUsage) != 90 {
		t.Fatalf("daily usage = enabled:%t points:%d, want enabled and 90 points", snapshot.DailyUsageEnabled, len(snapshot.DailyUsage))
	}
	if len(snapshot.Tokens) != 0 {
		t.Fatalf("tokens = %d, want none", len(snapshot.Tokens))
	}
	if snapshot.DailyUsage[len(snapshot.DailyUsage)-1].UsedQuotaUSD != 1.25 {
		t.Fatalf("today usage = %v, want 1.25", snapshot.DailyUsage[len(snapshot.DailyUsage)-1].UsedQuotaUSD)
	}
}

func TestFetchSub2APIFallsBackWhenModelPlazaUnavailable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":8}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":3,"name":"Legacy","description":"legacy group","rate_multiplier":1}]}`))
		case "/api/v1/model-plaza":
			http.NotFound(w, r)
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[],"pages":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := NewClient().Fetch(context.Background(), AccountCredentials{
		BaseURL:     server.URL,
		BearerToken: "legacy-token",
		Type:        models.UpstreamTypeSub2API,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(snapshot.Groups) != 1 || snapshot.Groups[0].CatalogSource != "groups_api" {
		t.Fatalf("groups = %#v", snapshot.Groups)
	}
	if len(snapshot.Groups[0].Models) != 0 {
		t.Fatalf("legacy models = %#v, want empty", snapshot.Groups[0].Models)
	}
}
