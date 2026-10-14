package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"upstream-x/server/bootstrap"
	"upstream-x/server/config"
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

	staticDir := filepath.Join("frontend", "dist")
	app, err := bootstrap.Build(cfg, staticDir)
	if err != nil {
		log.Fatalf("build application: %v", err)
	}

	if err := app.Scheduler.Start(); err != nil {
		log.Fatalf("start scheduler: %v", err)
	}
	defer app.Scheduler.Stop()

	addr := ":" + cfg.Server.Port
	log.Printf("server listening on %s", addr)
	if err := app.Router.Run(addr); err != nil {
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
