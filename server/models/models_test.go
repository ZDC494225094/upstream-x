package models

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateConvertsLegacyBrowserAuthModes(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy-auth.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("initial migration: %v", err)
	}

	legacy := []UpstreamAccount{
		{Name: "sub2api", Type: UpstreamTypeSub2API, BaseURL: "https://sub2api.example.com", Sub2APIAuthMode: "server_browser", BearerTokenEncrypted: "token"},
		{Name: "newapi", Type: UpstreamTypeNewAPI, BaseURL: "https://newapi.example.com", NewAPIAuthMode: "server_browser", BearerTokenEncrypted: "token"},
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy accounts: %v", err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("migration with legacy accounts: %v", err)
	}

	var accounts []UpstreamAccount
	if err := db.Order("id asc").Find(&accounts).Error; err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("account count = %d, want 2", len(accounts))
	}
	if accounts[0].Sub2APIAuthMode != Sub2APIAuthModeRefreshToken {
		t.Fatalf("sub2api auth mode = %q", accounts[0].Sub2APIAuthMode)
	}
	if accounts[1].NewAPIAuthMode != NewAPIAuthModeManual {
		t.Fatalf("newapi auth mode = %q", accounts[1].NewAPIAuthMode)
	}
}
