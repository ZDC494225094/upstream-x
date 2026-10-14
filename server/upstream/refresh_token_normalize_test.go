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

func TestRefreshUsesCustomUserAgent(t *testing.T) {
	var seenUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"a","refresh_token":"rt_next"}}`))
	}))
	defer server.Close()

	client := NewClient()
	browserUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
	if _, err := client.RefreshSub2APIToken(context.Background(), AccountCredentials{
		BaseURL: server.URL, Type: models.UpstreamTypeSub2API, RefreshToken: "rt_x", UserAgent: browserUA,
	}); err != nil {
		t.Fatal(err)
	}
	if seenUA != browserUA {
		t.Fatalf("custom UA not sent: got %q", seenUA)
	}
	if _, err := client.RefreshSub2APIToken(context.Background(), AccountCredentials{
		BaseURL: server.URL, Type: models.UpstreamTypeSub2API, RefreshToken: "rt_x",
	}); err != nil {
		t.Fatal(err)
	}
	if seenUA != userAgent {
		t.Fatalf("default UA changed: got %q", seenUA)
	}
}

func TestRefreshSendsBrowserContextHeaders(t *testing.T) {
	var origin, referer, secFetch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin = r.Header.Get("Origin")
		referer = r.Header.Get("Referer")
		secFetch = r.Header.Get("Sec-Fetch-Site")
		_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"a","refresh_token":"rt_next"}}`))
	}))
	defer server.Close()

	client := NewClient()
	if _, err := client.RefreshSub2APIToken(context.Background(), AccountCredentials{
		BaseURL: server.URL, Type: models.UpstreamTypeSub2API, RefreshToken: "rt_x",
	}); err != nil {
		t.Fatal(err)
	}
	if origin != server.URL || referer != server.URL+"/" || secFetch != "same-origin" {
		t.Fatalf("browser-context headers missing: origin=%q referer=%q sec-fetch-site=%q", origin, referer, secFetch)
	}
}
