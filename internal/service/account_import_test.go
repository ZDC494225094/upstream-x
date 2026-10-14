package service

import (
	"path/filepath"
	"testing"

	"upstream-x/internal/crypto"
	"upstream-x/internal/models"
	"upstream-x/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestAccountService(t *testing.T) *AccountService {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "accounts.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("migrate models: %v", err)
	}

	repo := repository.New(db)
	cipher := crypto.New("test-secret")
	return NewAccountService(repo, cipher)
}

func TestImportBundleIgnoresForeignIDCollisions(t *testing.T) {
	t.Parallel()

	service := newTestAccountService(t)

	_, err := service.Create(AccountPayload{
		Name:                   "local-existing",
		Type:                   models.UpstreamTypeSub2API,
		BaseURL:                "https://local.example.com",
		Sub2APIAuthMode:        models.Sub2APIAuthModeRefreshToken,
		RefreshToken:           "local-refresh-token",
		LowBalanceThresholdUSD: 1,
		Enabled:                true,
	})
	if err != nil {
		t.Fatalf("create local account: %v", err)
	}

	result, err := service.ImportBundle(AccountTransferBundle{
		Accounts: []AccountTransferItem{
			{
				ID:                     1,
				Name:                   "import-1",
				Type:                   models.UpstreamTypeSub2API,
				BaseURL:                "https://import-1.example.com",
				Sub2APIAuthMode:        models.Sub2APIAuthModeRefreshToken,
				RefreshToken:           "refresh-1",
				LowBalanceThresholdUSD: 1,
				Enabled:                true,
			},
			{
				ID:                     2,
				Name:                   "import-2",
				Type:                   models.UpstreamTypeSub2API,
				BaseURL:                "https://import-2.example.com",
				Sub2APIAuthMode:        models.Sub2APIAuthModeRefreshToken,
				RefreshToken:           "refresh-2",
				LowBalanceThresholdUSD: 2,
				Enabled:                true,
			},
			{
				ID:                     3,
				Name:                   "import-3",
				Type:                   models.UpstreamTypeSub2API,
				BaseURL:                "https://import-3.example.com",
				Sub2APIAuthMode:        models.Sub2APIAuthModeRefreshToken,
				RefreshToken:           "refresh-3",
				LowBalanceThresholdUSD: 3,
				Enabled:                true,
			},
		},
	})
	if err != nil {
		t.Fatalf("import bundle: %v", err)
	}

	if result.Created != 3 || result.Updated != 0 || result.Failed != 0 {
		t.Fatalf("unexpected import result: %#v", result)
	}

	accounts, err := service.List()
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 4 {
		t.Fatalf("expected 4 accounts after import, got %d", len(accounts))
	}

	names := make(map[string]bool, len(accounts))
	for _, account := range accounts {
		names[account.Name] = true
	}

	for _, expected := range []string{"local-existing", "import-1", "import-2", "import-3"} {
		if !names[expected] {
			t.Fatalf("missing account %q after import", expected)
		}
	}
}
