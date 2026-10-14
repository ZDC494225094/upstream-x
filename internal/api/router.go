package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"upstream-x/internal/config"
	"upstream-x/internal/mailer"
	"upstream-x/internal/scheduler"
	"upstream-x/internal/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const sessionName = "upstream-x-session"

type Handler struct {
	cfg       *config.Config
	accounts  *service.AccountService
	detector  *service.DetectorService
	settings  *service.SettingsService
	scheduler *scheduler.Scheduler
	adminHash []byte
	staticDir string
}

func NewHandler(cfg *config.Config, accounts *service.AccountService, detector *service.DetectorService, settings *service.SettingsService, scheduler *scheduler.Scheduler, staticDir string) (*Handler, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &Handler{
		cfg:       cfg,
		accounts:  accounts,
		detector:  detector,
		settings:  settings,
		scheduler: scheduler,
		adminHash: hash,
		staticDir: staticDir,
	}, nil
}

func (h *Handler) Router() (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	store := cookie.NewStore([]byte(h.cfg.Auth.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   int(h.cfg.Auth.SessionMaxAge.Seconds()),
		HttpOnly: true,
	})
	r.Use(sessions.Sessions(sessionName, store))
	r.Use(h.sessionOptionsMiddleware())
	r.Use(h.frameEmbeddingHeaders())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.POST("/auth/login", h.login)
		api.POST("/auth/logout", h.requireAuth, h.logout)
		api.GET("/auth/me", h.requireAuth, h.me)

		protected := api.Group("/")
		protected.Use(h.requireAuth)
		protected.GET("/dashboard/summary", h.dashboardSummary)
		protected.GET("/dashboard/daily-usage", h.dashboardDailyUsage)
		protected.GET("/dashboard/screen", h.dashboardScreen)
		protected.POST("/dashboard/refresh", h.refreshDashboardData)
		protected.GET("/dashboard/screen-layout", h.getScreenLayout)
		protected.PUT("/dashboard/screen-layout", h.updateScreenLayout)
		protected.GET("/accounts", h.listAccounts)
		protected.GET("/accounts/export", h.exportAccounts)
		protected.POST("/accounts/import", h.importAccounts)
		protected.POST("/accounts/validate", h.validateDraftAccount)
		protected.POST("/accounts", h.createAccount)
		protected.PUT("/accounts/:id", h.updateAccount)
		protected.DELETE("/accounts/:id", h.deleteAccount)
		protected.POST("/accounts/:id/validate", h.validateAccount)
		protected.POST("/accounts/:id/check", h.checkAccount)
		protected.GET("/accounts/:id/overview", h.accountOverview)
		protected.GET("/accounts/:id/balance-history", h.balanceHistory)
		protected.GET("/accounts/:id/group-history", h.groupHistory)
		protected.GET("/accounts/:id/latest-groups", h.latestGroups)
		protected.GET("/accounts/:id/run-logs", h.runLogs)
		protected.GET("/settings", h.getSettings)
		protected.GET("/settings/export", h.exportSettings)
		protected.POST("/settings/import", h.importSettings)
		protected.GET("/settings/schedule", h.getSchedule)
		protected.PUT("/settings/schedule", h.updateSchedule)
		protected.PUT("/settings/logs", h.updateLogRetention)
		protected.POST("/settings/logs/cleanup", h.cleanupLogs)
		protected.GET("/settings/custom-menu-links", h.getCustomMenuLinks)
		protected.PUT("/settings/custom-menu-links", h.updateCustomMenuLinks)
		protected.PUT("/settings/smtp", h.updateSMTP)
		protected.POST("/settings/smtp/test", h.sendSMTPTest)
	}
	h.registerFrontend(r)

	return r, nil
}

func sessionSameSiteMode(value string, secure bool) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		if !secure {
			return http.SameSiteLaxMode, nil
		}
		return http.SameSiteNoneMode, nil
	default:
		return http.SameSiteDefaultMode, errors.New("invalid auth.session_same_site")
	}
}

func (h *Handler) sessionOptionsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		session.Options(h.sessionOptionsForRequest(c, int(h.cfg.Auth.SessionMaxAge.Seconds())))
		c.Next()
	}
}

func (h *Handler) sessionOptionsForRequest(c *gin.Context, maxAge int) sessions.Options {
	secure := h.sessionSecureForRequest(c.Request)
	sameSite, err := sessionSameSiteMode(h.cfg.Auth.SessionSameSite, secure)
	if err != nil {
		sameSite = http.SameSiteLaxMode
	}

	return sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	}
}

func (h *Handler) sessionSecureForRequest(r *http.Request) bool {
	if h.cfg.Auth.SessionSecure != nil {
		return *h.cfg.Auth.SessionSecure
	}
	return requestIsSecure(r) || isLocalhostHost(r.Host)
}

func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Ssl")), "on") {
		return true
	}
	for _, part := range strings.Split(r.Header.Get("Forwarded"), ";") {
		if strings.EqualFold(strings.TrimSpace(part), "proto=https") {
			return true
		}
	}
	return false
}

func isLocalhostHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err == nil {
			host = parsed.Host
		}
	}
	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			host = parsedHost
		}
	}
	host = strings.Trim(host, "[]")
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}

func (h *Handler) frameEmbeddingHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.cfg.Auth.IframeEmbeddingEnabled() {
			origins := h.cfg.Auth.IframeAllowedOrigins
			if len(origins) > 0 {
				c.Header("Content-Security-Policy", "frame-ancestors "+strings.Join(origins, " "))
			}
			c.Writer.Header().Del("X-Frame-Options")
		} else {
			c.Header("X-Frame-Options", "SAMEORIGIN")
			c.Header("Content-Security-Policy", "frame-ancestors 'self'")
		}
		c.Next()
	}
}

func (h *Handler) login(c *gin.Context) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.Username != h.cfg.Admin.Username || bcrypt.CompareHashAndPassword(h.adminHash, []byte(payload.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	session := sessions.Default(c)
	session.Options(h.sessionOptionsForRequest(c, int(h.cfg.Auth.SessionMaxAge.Seconds())))
	session.Set("authenticated", true)
	session.Save()
	c.JSON(http.StatusOK, gin.H{"username": h.cfg.Admin.Username})
}

func (h *Handler) logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Options(h.sessionOptionsForRequest(c, -1))
	_ = session.Save()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"username": h.cfg.Admin.Username})
}

func (h *Handler) dashboardSummary(c *gin.Context) {
	summary, err := h.detector.Dashboard(c.DefaultQuery("range", "3d"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) dashboardDailyUsage(c *gin.Context) {
	summary, err := h.detector.DailyUsage(c.DefaultQuery("range", "30d"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) dashboardScreen(c *gin.Context) {
	data, err := h.detector.ScreenOverview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) refreshDashboardData(c *gin.Context) {
	data, err := h.detector.RefreshScreenOverview(c.Request.Context(), "manual")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) getScreenLayout(c *gin.Context) {
	layout, err := h.settings.GetScreenLayout()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, layout)
}

func (h *Handler) updateScreenLayout(c *gin.Context) {
	var payload service.ScreenLayoutSettings
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	layout, err := h.settings.UpdateScreenLayout(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, layout)
}

func (h *Handler) listAccounts(c *gin.Context) {
	accounts, err := h.accounts.ListViews()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": accounts})
}

func (h *Handler) exportAccounts(c *gin.Context) {
	bundle, err := h.accounts.ExportBundle()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *Handler) importAccounts(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bundle service.AccountTransferBundle
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body is empty"})
		return
	}

	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &bundle.Accounts); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	default:
		if err := json.Unmarshal(trimmed, &bundle); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result, err := h.accounts.ImportBundle(bundle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := http.StatusOK
	if result.Failed > 0 {
		status = http.StatusMultiStatus
	}
	c.JSON(status, result)
}

func (h *Handler) createAccount(c *gin.Context) {
	var payload service.AccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.accounts.Create(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	view, err := h.accounts.ToView(account)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (h *Handler) validateDraftAccount(c *gin.Context) {
	var payload service.AccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.accounts.BuildTransient(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	snapshot, err := h.detector.ValidateAccount(c.Request.Context(), account)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) updateAccount(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var payload service.AccountPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.accounts.Update(id, payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	view, err := h.accounts.ToView(account)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *Handler) deleteAccount(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.accounts.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) validateAccount(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.accounts.Get(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if c.Request.Body != nil {
		body, readErr := io.ReadAll(c.Request.Body)
		if readErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": readErr.Error()})
			return
		}
		if len(bytes.TrimSpace(body)) > 0 {
			var payload service.AccountPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			merged, err := h.accounts.BuildValidationPayload(account, payload)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			account, err = h.accounts.BuildTransient(merged)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		} else {
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}
	}

	snapshot, err := h.detector.ValidateAccount(c.Request.Context(), account)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) checkAccount(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.accounts.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err := h.detector.CheckAccount(c.Request.Context(), account, "manual"); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) accountOverview(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := h.detector.AccountOverview(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) balanceHistory(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	points, err := h.detector.BalanceHistory(id, c.DefaultQuery("range", "3d"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": points})
}

func (h *Handler) groupHistory(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	points, err := h.detector.GroupHistory(id, c.DefaultQuery("range", "3d"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": points})
}

func (h *Handler) latestGroups(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	overview, err := h.detector.AccountOverview(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": overview.LatestGroups})
}

func (h *Handler) runLogs(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	runs, err := h.detector.RunLogs(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs})
}

func (h *Handler) getSchedule(c *gin.Context) {
	interval, err := h.settings.GetScheduleInterval()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"interval_minutes": interval})
}

func (h *Handler) updateSchedule(c *gin.Context) {
	var payload struct {
		IntervalMinutes int `json:"interval_minutes"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if payload.IntervalMinutes <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "interval_minutes must be > 0"})
		return
	}
	if err := h.settings.UpdateScheduleInterval(payload.IntervalMinutes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.scheduler.UpdateInterval(payload.IntervalMinutes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"interval_minutes": payload.IntervalMinutes})
}

func (h *Handler) updateLogRetention(c *gin.Context) {
	var payload struct {
		LogRetentionDays int `json:"log_retention_days"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.settings.UpdateLogRetentionDays(payload.LogRetentionDays); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"log_retention_days": payload.LogRetentionDays})
}

func (h *Handler) cleanupLogs(c *gin.Context) {
	var payload struct {
		LogRetentionDays int `json:"log_retention_days"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.settings.PurgeLogs(payload.LogRetentionDays)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) getCustomMenuLinks(c *gin.Context) {
	links, err := h.settings.GetCustomMenuLinks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": links})
}

func (h *Handler) updateCustomMenuLinks(c *gin.Context) {
	var payload service.CustomMenuLinksSettings
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.settings.UpdateCustomMenuLinks(payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) getSettings(c *gin.Context) {
	snapshot, err := h.settings.Snapshot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) exportSettings(c *gin.Context) {
	bundle, err := h.settings.ExportBundle()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (h *Handler) importSettings(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var bundle service.SettingsTransferBundle
	if err := json.Unmarshal(bytes.TrimSpace(body), &bundle); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snapshot, err := h.settings.ImportBundle(bundle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.scheduler.UpdateInterval(snapshot.ScheduleIntervalMinutes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

func (h *Handler) updateSMTP(c *gin.Context) {
	var payload service.SMTPSettingsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.settings.UpdateSMTPSettings(payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) sendSMTPTest(c *gin.Context) {
	var payload struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	smtpSettings, err := h.settings.GetSMTPSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	testMailer := mailer.New(func() (config.SMTPConfig, error) {
		return config.SMTPConfig{
			Enabled:    smtpSettings.Enabled,
			Host:       smtpSettings.Host,
			Port:       smtpSettings.Port,
			UserName:   smtpSettings.Username,
			Password:   smtpSettings.Password,
			From:       smtpSettings.From,
			Recipients: smtpSettings.Recipients,
			UseTLS:     smtpSettings.UseTLS,
		}, nil
	})

	subject := payload.Subject
	if subject == "" {
		subject = "Upstream X SMTP 测试邮件"
	}
	body := payload.Body
	if body == "" {
		body = "这是一封来自 Upstream X 的 SMTP 测试邮件。"
	}
	if err := testMailer.SendTest(subject, body); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handler) requireAuth(c *gin.Context) {
	session := sessions.Default(c)
	authenticated, ok := session.Get("authenticated").(bool)
	if !ok || !authenticated {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.Next()
}

func parseUintParam(c *gin.Context, key string) (uint, error) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

func (h *Handler) registerFrontend(r *gin.Engine) {
	indexPath := filepath.Join(h.staticDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{"error": "frontend not built"})
		})
		return
	}

	r.Static("/assets", filepath.Join(h.staticDir, "assets"))
	r.StaticFile("/favicon.png", filepath.Join(h.staticDir, "favicon.png"))
	r.StaticFile("/newapi.svg", filepath.Join(h.staticDir, "newapi.svg"))
	r.StaticFile("/sub2api.logo.png", filepath.Join(h.staticDir, "sub2api.logo.png"))
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(indexPath)
	})
}
