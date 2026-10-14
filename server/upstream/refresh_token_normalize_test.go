package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"upstream-x/server/models"
)

func TestNormalizeRefreshToken(t *testing.T) {
	cases := map[string]string{
		"rt_abc":         "rt_abc",
		"  rt_abc  ":     "rt_abc",
		`"rt_abc"`:       "rt_abc",
		"'rt_abc'":       "rt_abc",
		" \" rt_abc \" ": "rt_abc",
		"":               "",
	}
	for in, want := range cases {
		if got := NormalizeRefreshToken(in); got != want {
			t.Errorf("NormalizeRefreshToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRefreshSub2APITokenAcceptsQuotedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.RefreshToken != "rt_good" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"invalid refresh token","reason":"REFRESH_TOKEN_INVALID"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"at_new","refresh_token":"rt_next"}}`))
	}))
	defer server.Close()

	client := NewClient()
	result, err := client.RefreshSub2APIToken(context.Background(), AccountCredentials{
		BaseURL:      server.URL,
		Type:         models.UpstreamTypeSub2API,
		RefreshToken: `"rt_good"`,
	})
	if err != nil {
		t.Fatalf("quoted token should refresh fine: %v", err)
	}
	if result.RefreshToken != "rt_next" || result.AccessToken != "at_new" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
