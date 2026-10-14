// Package bootstrap wires the application's dependencies (database,
// repositories, services, HTTP router) so both the standalone server
// binary and the Vercel serverless entrypoint share one construction
// path.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"upstream-x/server/api"
	"upstream-x/server/config"
	"upstream-x/server/crypto"
	"upstream-x/server/mailer"
	"upstream-x/server/models"
	"upstream-x/server/repository"
	"upstream-x/server/scheduler"
	"upstream-x/server/service"
	"upstream-x/server/upstream"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	DefaultScheduleIntervalMinutes = 10
	DefaultLogRetentionDays        = 90
)

type App struct {
	Router    *gin.Engine
	Detector  *service.DetectorService
	Settings  *service.SettingsService
	Accounts  *service.AccountService
	Scheduler *scheduler.Scheduler
	DB        *gorm.DB
}

// OpenDatabase selects Postgres when a DATABASE_URL is configured
// (serverless deployments), otherwise the local SQLite file.
func OpenDatabase(cfg *config.Config) (*gorm.DB, error) {
	if dsn := strings.TrimSpace(cfg.Database.PostgresURL); dsn != "" {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("open postgres database: %w", err)
		}
		if sqlDB, err := db.DB(); err == nil {
			// Serverless instances are short-lived and numerous; keep the
			// per-instance pool small and recycle connections quickly.
			sqlDB.SetMaxOpenConns(5)
			sqlDB.SetMaxIdleConns(2)
			sqlDB.SetConnMaxLifetime(5 * time.Minute)
		}
		return db, nil
	}

	sqlitePath := cfg.Database.SQLitePath
	if os.Getenv("VERCEL") == "1" && !filepath.IsAbs(sqlitePath) {
		// Serverless functions have a read-only working directory; the
		// only writable location is /tmp. Without DATABASE_URL the app
		// still boots on an ephemeral SQLite file there (data does not
		// survive cold starts) so the deployment stays usable for
		// evaluation until a Postgres DATABASE_URL is configured.
		sqlitePath = filepath.Join(os.TempDir(), "upstream-x", filepath.Base(sqlitePath))
	}
	directory := filepath.Dir(filepath.Clean(sqlitePath))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory %s: %w", directory, err)
	}
	db, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	return db, nil
}

// Build constructs the full application. It does not start the cron
// scheduler: the binary entrypoint starts it explicitly, while the
// serverless entrypoint relies on Vercel Cron invoking the detector
// through an internal endpoint instead.
func Build(cfg *config.Config, staticDir string) (*App, error) {
	db, err := OpenDatabase(cfg)
	if err != nil {
		return nil, err
	}
	if err := models.AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	repo := repository.New(db)
	cipher := crypto.New(cfg.Security.CredentialKey)
	accountService := service.NewAccountService(repo, cipher)
	settingsService := service.NewSettingsService(
		repo,
		DefaultScheduleIntervalMinutes,
		config.SMTPConfig{
			Port:   465,
			UseTLS: true,
		},
		DefaultLogRetentionDays,
	)

	alertMailer := mailer.New(func() (config.SMTPConfig, error) {
		smtpSettings, err := settingsService.GetSMTPSettings()
		if err != nil {
			return config.SMTPConfig{}, err
		}
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

	upstreamClient := upstream.NewClientWithProxyProvider(func() (string, error) {
		return settingsService.GetProxyURL()
	})
	detectorService := service.NewDetectorService(
		repo,
		accountService,
		upstreamClient,
		alertMailer,
		settingsService,
	)
	jobScheduler := scheduler.New(settingsService, detectorService)

	handler, err := api.NewHandler(
		cfg,
		accountService,
		detectorService,
		settingsService,
		jobScheduler,
		staticDir,
	)
	if err != nil {
		return nil, fmt.Errorf("create handler: %w", err)
	}
	router, err := handler.Router()
	if err != nil {
		return nil, fmt.Errorf("build router: %w", err)
	}

	return &App{
		Router:    router,
		Detector:  detectorService,
		Settings:  settingsService,
		Accounts:  accountService,
		Scheduler: jobScheduler,
		DB:        db,
	}, nil
}
