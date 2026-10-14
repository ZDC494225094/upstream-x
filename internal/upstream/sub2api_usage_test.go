package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"upstream-x/internal/models"
)

func TestFetchSub2APIEnrichesKeyUsage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":12.5}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":7,"name":"OpenAI","rate_multiplier":1}]}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":10,"key":"sk-test","name":"used","group_id":7,"status":"active","quota_used":3.25,"group":{"id":7,"name":"OpenAI"}},{"id":11,"key":"sk-old","name":"unused","group_id":7,"status":"active","group":{"id":7,"name":"OpenAI"}}],"pages":1}}`))
		case "/api/v1/usage/dashboard/api-keys-usage":
			if r.Method != http.MethodPost {
				t.Fatalf("usage method = %s, want POST", r.Method)
			}
			var payload struct {
				IDs []int `json:"api_key_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode usage payload: %v", err)
			}
			if len(payload.IDs) != 2 || payload.IDs[0] != 10 || payload.IDs[1] != 11 {
				t.Fatalf("usage ids = %v", payload.IDs)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"stats":{"10":{"api_key_id":10,"total_actual_cost":4.5},"11":{"api_key_id":11,"total_actual_cost":0}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := NewClient().Fetch(context.Background(), AccountCredentials{
		BaseURL:     server.URL,
		BearerToken: "token",
		Type:        models.UpstreamTypeSub2API,
	})
	if err != nil {
		t.Fatalf("fetch sub2api: %v", err)
	}
	if len(snapshot.Tokens) != 2 {
		t.Fatalf("token count = %d, want 2", len(snapshot.Tokens))
	}
	if snapshot.Tokens[0].UsedQuota == nil || *snapshot.Tokens[0].UsedQuota != 4.5 {
		t.Fatalf("first token usage = %v, want 4.5", snapshot.Tokens[0].UsedQuota)
	}
	if snapshot.Tokens[1].UsedQuota == nil || *snapshot.Tokens[1].UsedQuota != 0 {
		t.Fatalf("second token usage = %v, want 0", snapshot.Tokens[1].UsedQuota)
	}
}

func TestFetchSub2APIPreservesFallbackUsageWhenDashboardUnsupported(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":12.5}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":10,"key":"sk-test","name":"used","group_id":7,"status":"active","quota_used":3.25,"group":{"id":7,"name":"OpenAI"}}],"pages":1}}`))
		case "/api/v1/usage/dashboard/api-keys-usage":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	snapshot, err := NewClient().Fetch(context.Background(), AccountCredentials{
		BaseURL:     server.URL,
		BearerToken: "token",
		Type:        models.UpstreamTypeSub2API,
	})
	if err != nil {
		t.Fatalf("fetch sub2api: %v", err)
	}
	if len(snapshot.Tokens) != 1 || snapshot.Tokens[0].UsedQuota == nil || *snapshot.Tokens[0].UsedQuota != 3.25 {
		t.Fatalf("fallback token usage = %+v, want 3.25", snapshot.Tokens)
	}
}
