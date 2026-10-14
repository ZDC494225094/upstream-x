package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"upstream-x/server/config"

	"github.com/gin-gonic/gin"
)

func TestFrameEmbeddingHeadersAllowAll(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		cfg: &config.Config{
			Auth: config.AuthConfig{
				AllowIframeEmbedding: boolPtr(true),
			},
		},
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(handler.frameEmbeddingHeaders())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("Content-Security-Policy = %q, want empty", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want empty", got)
	}
}

func TestFrameEmbeddingHeadersSameOriginWhenDisabled(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		cfg: &config.Config{
			Auth: config.AuthConfig{
				AllowIframeEmbedding: boolPtr(false),
			},
		},
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(handler.frameEmbeddingHeaders())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Fatalf("X-Frame-Options = %q, want %q", got, "SAMEORIGIN")
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "frame-ancestors 'self'" {
		t.Fatalf("Content-Security-Policy = %q, want %q", got, "frame-ancestors 'self'")
	}
}

func TestSessionOptionsForSecureRequestUsesSameSiteNone(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		cfg: &config.Config{
			Auth: config.AuthConfig{
				SessionMaxAge:   168,
				SessionSameSite: "none",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	options := handler.sessionOptionsForRequest(c, 3600)
	if !options.Secure {
		t.Fatal("Secure = false, want true")
	}
	if options.SameSite != http.SameSiteNoneMode {
		t.Fatalf("SameSite = %v, want %v", options.SameSite, http.SameSiteNoneMode)
	}
}

func TestSessionOptionsForInsecureRequestFallsBackToLax(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		cfg: &config.Config{
			Auth: config.AuthConfig{
				SessionSameSite: "none",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	options := handler.sessionOptionsForRequest(c, 3600)
	if options.Secure {
		t.Fatal("Secure = true, want false")
	}
	if options.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want %v", options.SameSite, http.SameSiteLaxMode)
	}
}

func TestSessionOptionsForLocalhostTreatsAsSecure(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		cfg: &config.Config{
			Auth: config.AuthConfig{
				SessionSameSite: "none",
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/", nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	options := handler.sessionOptionsForRequest(c, 3600)
	if !options.Secure {
		t.Fatal("Secure = false, want true for localhost")
	}
	if options.SameSite != http.SameSiteNoneMode {
		t.Fatalf("SameSite = %v, want %v", options.SameSite, http.SameSiteNoneMode)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
