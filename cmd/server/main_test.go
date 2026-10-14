package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"upstream-x/server/bootstrap"
	"upstream-x/server/config"
)

func TestConfigureTimeZone(t *testing.T) {
	original := time.Local
	t.Cleanup(func() {
		time.Local = original
	})

	if err := configureTimeZone("Asia/Shanghai"); err != nil {
		t.Fatalf("configure timezone: %v", err)
	}
	if got := time.Local.String(); got != "Asia/Shanghai" {
		t.Fatalf("expected Asia/Shanghai, got %s", got)
	}
}

func TestConfigureTimeZoneRejectsUnknownZone(t *testing.T) {
	if err := configureTimeZone("Invalid/TimeZone"); err == nil {
		t.Fatal("expected invalid timezone to return an error")
	}
}

func TestOpenDatabaseCreatesSQLiteDirectory(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "nested", "app.db")
	cfg := &config.Config{}
	cfg.Database.SQLitePath = databasePath
	db, err := bootstrap.OpenDatabase(cfg)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(databasePath)); err != nil {
		t.Fatalf("expected sqlite directory to exist: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
