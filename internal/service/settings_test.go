package service

import (
	"path/filepath"
	"testing"

	"upstream-x/internal/config"
	"upstream-x/internal/models"
	"upstream-x/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestSettingsService(t *testing.T) *SettingsService {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("migrate models: %v", err)
	}

	return NewSettingsService(
		repository.New(db),
		10,
		config.SMTPConfig{},
		30,
	)
}

func TestScreenLayoutSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	service := newTestSettingsService(t)

	saved, err := service.UpdateScreenLayout(ScreenLayoutSettings{
		Items: []ScreenLayoutItem{
			{AccountID: 3, Collapsed: true},
			{AccountID: 1, Collapsed: false},
			{AccountID: 2, Collapsed: true},
		},
	})
	if err != nil {
		t.Fatalf("update screen layout: %v", err)
	}

	if len(saved.Items) != 3 {
		t.Fatalf("expected 3 saved items, got %d", len(saved.Items))
	}

	loaded, err := service.GetScreenLayout()
	if err != nil {
		t.Fatalf("get screen layout: %v", err)
	}

	if len(loaded.Items) != 3 {
		t.Fatalf("expected 3 loaded items, got %d", len(loaded.Items))
	}

	if loaded.Items[0].AccountID != 3 || !loaded.Items[0].Collapsed {
		t.Fatalf("unexpected first item: %#v", loaded.Items[0])
	}
	if loaded.Items[1].AccountID != 1 || loaded.Items[1].Collapsed {
		t.Fatalf("unexpected second item: %#v", loaded.Items[1])
	}
	if loaded.Items[2].AccountID != 2 || !loaded.Items[2].Collapsed {
		t.Fatalf("unexpected third item: %#v", loaded.Items[2])
	}
}

func TestScreenLayoutSettingsDeduplicatesAccounts(t *testing.T) {
	t.Parallel()

	service := newTestSettingsService(t)

	loaded, err := service.UpdateScreenLayout(ScreenLayoutSettings{
		Items: []ScreenLayoutItem{
			{AccountID: 2, Collapsed: false},
			{AccountID: 2, Collapsed: true},
			{AccountID: 4, Collapsed: true},
		},
	})
	if err != nil {
		t.Fatalf("update screen layout: %v", err)
	}

	if len(loaded.Items) != 2 {
		t.Fatalf("expected deduplicated items, got %d", len(loaded.Items))
	}

	if loaded.Items[0].AccountID != 2 || loaded.Items[0].Collapsed {
		t.Fatalf("unexpected first deduplicated item: %#v", loaded.Items[0])
	}
	if loaded.Items[1].AccountID != 4 || !loaded.Items[1].Collapsed {
		t.Fatalf("unexpected second deduplicated item: %#v", loaded.Items[1])
	}
}

func TestCustomMenuLinksRoundTrip(t *testing.T) {
	t.Parallel()

	service := newTestSettingsService(t)

	saved, err := service.UpdateCustomMenuLinks(CustomMenuLinksSettings{
		Items: []CustomMenuLink{
			{Title: "Grafana", URL: "https://grafana.example.com/d/home", EmbedIframe: true},
			{ID: "docs-link", Title: "内部文档", URL: "/docs", EmbedIframe: false},
		},
	})
	if err != nil {
		t.Fatalf("update custom menu links: %v", err)
	}

	if len(saved.Items) != 2 {
		t.Fatalf("expected 2 saved items, got %d", len(saved.Items))
	}
	if saved.Items[0].ID == "" {
		t.Fatalf("expected generated id for first item")
	}
	if saved.Items[1].ID != "docs-link" {
		t.Fatalf("expected stable id for second item, got %q", saved.Items[1].ID)
	}

	loaded, err := service.GetCustomMenuLinks()
	if err != nil {
		t.Fatalf("get custom menu links: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded items, got %d", len(loaded))
	}
	if !loaded[0].EmbedIframe {
		t.Fatalf("expected first link to preserve iframe flag")
	}
	if loaded[1].URL != "/docs" {
		t.Fatalf("expected second link url to round-trip, got %q", loaded[1].URL)
	}

	snapshot, err := service.Snapshot()
	if err != nil {
		t.Fatalf("snapshot settings: %v", err)
	}
	if len(snapshot.CustomMenuLinks) != 2 {
		t.Fatalf("expected snapshot to include custom menu links, got %d", len(snapshot.CustomMenuLinks))
	}
}

func TestCustomMenuLinksRejectInvalidURL(t *testing.T) {
	t.Parallel()

	service := newTestSettingsService(t)

	_, err := service.UpdateCustomMenuLinks(CustomMenuLinksSettings{
		Items: []CustomMenuLink{
			{Title: "Bad Link", URL: "javascript:alert(1)", EmbedIframe: true},
		},
	})
	if err == nil {
		t.Fatal("expected invalid custom menu link url to fail")
	}
}
