package main

import (
	"path/filepath"
	"testing"
	"time"
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

func TestEnsureSQLiteDirectory(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "nested", "app.db")
	if err := ensureSQLiteDirectory(databasePath); err != nil {
		t.Fatalf("ensure sqlite directory: %v", err)
	}
}
