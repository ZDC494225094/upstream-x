// Vercel serverless entrypoint. Every /api/* request (and /healthz) is
// rewritten to this function by vercel.json; the Gin router inside sees
// the original request path.
//
// Configuration comes exclusively from environment variables:
// ADMIN_USERNAME, ADMIN_PASSWORD, AUTH_SESSION_SECRET,
// SECURITY_CREDENTIAL_KEY and DATABASE_URL (Postgres, e.g. Neon).
// Scheduled checks run through Vercel Cron hitting
// /api/internal/check-all, authenticated with CRON_SECRET.
package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"upstream-x/internal/bootstrap"
	"upstream-x/internal/config"
)

var (
	once    sync.Once
	app     *bootstrap.App
	initErr error
)

func initApp() {
	cfg, err := config.Load("")
	if err != nil {
		initErr = err
		return
	}
	if location, err := time.LoadLocation(cfg.App.TimeZone); err == nil {
		time.Local = location
	}
	// No static dir: Vercel serves the built frontend itself, this
	// function only handles API traffic.
	app, initErr = bootstrap.Build(cfg, "")
	if initErr != nil {
		log.Printf("upstream-x init: %v", initErr)
	}
}

// Handler is the Vercel function entrypoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(initApp)
	if initErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "service initialization failed")
		return
	}
	if r.URL.Path == "/api/internal/check-all" {
		handleCronCheck(w, r)
		return
	}
	app.Router.ServeHTTP(w, r)
}

// handleCronCheck runs one full detection pass. Vercel Cron sends
// "Authorization: Bearer <CRON_SECRET>" automatically when the
// CRON_SECRET environment variable is set on the project.
func handleCronCheck(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(os.Getenv("CRON_SECRET"))
	if secret == "" || r.Header.Get("Authorization") != "Bearer "+secret {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := app.Detector.CheckAllEnabled(r.Context(), "vercel-cron"); err != nil {
		log.Printf("upstream-x cron check: %v", err)
		writeJSONError(w, http.StatusInternalServerError, "check run failed")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
