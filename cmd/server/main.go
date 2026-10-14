package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"upstream-x/internal/api"
	"upstream-x/internal/config"
	"upstream-x/internal/crypto"
	"upstream-x/internal/mailer"
	"upstream-x/internal/models"
	"upstream-x/internal/repository"
	"upstream-x/internal/scheduler"
	"upstream-x/internal/service"
	"upstream-x/internal/upstream"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	defaultScheduleIntervalMinutes = 10
	defaultLogRetentionDays        = 90
)

func main() {
	configPath := strings.TrimSpace(os.Getenv("APP_CONFIG"))
	if configPath == "" {
		configPath = filepath.Join("config", "config.yaml")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := configureTimeZone(cfg.App.TimeZone); err != nil {
		log.Fatalf("configure timezone: %v", err)
	}
	log.Printf("application timezone set to %s", time.Local)

	if err := ensureSQLiteDirectory(cfg.Database.SQLitePath); err != nil {
		log.Fatalf("prepare sqlite database directory: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(cfg.Database.SQLitePath), &gorm.Config{})
	if err != nil {
		log.Fatalf("open sqlite database: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	repo := repository.New(db)
	cipher := crypto.New(cfg.Security.CredentialKey)
	accountService := service.NewAccountService(repo, cipher)
	settingsService := service.NewSettingsService(
		repo,
		defaultScheduleIntervalMinutes,
		config.SMTPConfig{
			Port:   465,
			UseTLS: true,
		},
		defaultLogRetentionDays,
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

	detectorService := service.NewDetectorService(
		repo,
		accountService,
		upstream.NewClient(),
		alertMailer,
		settingsService,
	)
	jobScheduler := scheduler.New(settingsService, detectorService)
	if err := jobScheduler.Start(); err != nil {
		log.Fatalf("start scheduler: %v", err)
	}
	defer jobScheduler.Stop()

	staticDir := filepath.Join("frontend", "dist")
	handler, err := api.NewHandler(
		cfg,
		accountService,
		detectorService,
		settingsService,
		jobScheduler,
		staticDir,
	)
	if err != nil {
		log.Fatalf("create handler: %v", err)
	}
	router, err := handler.Router()
	if err != nil {
		log.Fatalf("build router: %v", err)
	}

	addr := ":" + cfg.Server.Port
	log.Printf("server listening on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func configureTimeZone(name string) error {
	timeZone := strings.TrimSpace(name)
	if timeZone == "" {
		timeZone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return fmt.Errorf("load %s: %w", timeZone, err)
	}
	time.Local = location
	return nil
}

func ensureSQLiteDirectory(path string) error {
	directory := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", directory, err)
	}
	return nil
}
