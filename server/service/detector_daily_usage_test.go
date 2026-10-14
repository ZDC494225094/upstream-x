package service

import (
	"path/filepath"
	"testing"
	"time"

	"upstream-x/server/models"
	"upstream-x/server/repository"
	"upstream-x/server/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDailyUsageAggregatesPositiveTokenDeltas(t *testing.T) {
	originalLocation := time.Local
	time.Local = time.UTC
	t.Cleanup(func() {
		time.Local = originalLocation
	})

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "daily-usage.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	today := startOfLocalDay(time.Now())
	yesterday := today.AddDate(0, 0, -1)
	rows := []models.TokenSnapshot{
		usageSnapshot(1, "a", 10, yesterday.Add(8*time.Hour)),
		usageSnapshot(1, "b", 5, yesterday.Add(9*time.Hour)),
		usageSnapshot(1, "a", 13, today.Add(8*time.Hour)),
		usageSnapshot(1, "a", 15, today.Add(9*time.Hour)),
		usageSnapshot(1, "b", 7, today.Add(10*time.Hour)),
		usageSnapshot(1, "a", 2, today.Add(11*time.Hour)),
		usageSnapshot(1, "a", 3, today.Add(12*time.Hour)),
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	detector := &DetectorService{repo: repository.New(db)}
	summary, err := detector.DailyUsage("3d")
	if err != nil {
		t.Fatalf("daily usage: %v", err)
	}
	if len(summary.Points) != 3 {
		t.Fatalf("points = %d, want 3", len(summary.Points))
	}
	if summary.TodayUsedQuotaUSD != 8 {
		t.Fatalf("today usage = %v, want 8", summary.TodayUsedQuotaUSD)
	}
	if summary.TotalUsedQuotaUSD != 8 {
		t.Fatalf("total usage = %v, want 8", summary.TotalUsedQuotaUSD)
	}
	if summary.Points[2].Date != today.Format("2006-01-02") || summary.Points[2].UsedQuotaUSD != 8 {
		t.Fatalf("today point = %#v", summary.Points[2])
	}
}

func TestDailyUsagePrefersAuthoritativeAccountTrend(t *testing.T) {
	originalLocation := time.Local
	time.Local = time.UTC
	t.Cleanup(func() {
		time.Local = originalLocation
	})

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "daily-trend.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	today := startOfLocalDay(time.Now())
	yesterday := today.AddDate(0, 0, -1)
	direct := []models.DailyUsageSnapshot{
		{AccountID: 1, Date: yesterday.Format("2006-01-02"), UsedQuotaUSD: 2, Source: "sub2api_trend", UpdatedAt: yesterday},
		{AccountID: 1, Date: today.Format("2006-01-02"), UsedQuotaUSD: 4, Source: "sub2api_trend", UpdatedAt: today},
	}
	if err := db.Create(&direct).Error; err != nil {
		t.Fatalf("create direct usage: %v", err)
	}
	rows := []models.TokenSnapshot{
		usageSnapshot(1, "direct-key", 10, yesterday.Add(8*time.Hour)),
		usageSnapshot(1, "direct-key", 99, today.Add(8*time.Hour)),
		usageSnapshot(2, "legacy-key", 5, yesterday.Add(9*time.Hour)),
		usageSnapshot(2, "legacy-key", 8, today.Add(9*time.Hour)),
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create snapshots: %v", err)
	}

	detector := &DetectorService{repo: repository.New(db)}
	summary, err := detector.DailyUsage("3d")
	if err != nil {
		t.Fatalf("daily usage: %v", err)
	}
	if summary.TodayUsedQuotaUSD != 7 {
		t.Fatalf("today usage = %v, want 7", summary.TodayUsedQuotaUSD)
	}
	if summary.TotalUsedQuotaUSD != 9 {
		t.Fatalf("total usage = %v, want 9", summary.TotalUsedQuotaUSD)
	}
}

func TestAccountOverviewIncludesModelPlazaCatalog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "account-overview.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	account := models.UpstreamAccount{Name: "plaza", Type: models.UpstreamTypeSub2API, BaseURL: "https://example.test"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	run := models.DetectionRun{AccountID: account.ID, Status: models.RunStatusSuccess, CreatedAt: time.Now()}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := db.Create(&models.GroupSnapshot{AccountID: account.ID, RunID: run.ID, GroupKey: "7", GroupName: "OpenAI"}).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := db.Create(&models.GroupCatalog{
		AccountID:   account.ID,
		GroupKey:    "7",
		GroupName:   "OpenAI",
		Platform:    "openai",
		Source:      "model_plaza",
		ModelsJSON:  `[{"name":"gpt-5","platform":"openai"}]`,
		ModelsCount: 1,
		UpdatedAt:   time.Now(),
	}).Error; err != nil {
		t.Fatalf("create catalog: %v", err)
	}

	overview, err := (&DetectorService{repo: repository.New(db)}).AccountOverview(account.ID)
	if err != nil {
		t.Fatalf("account overview: %v", err)
	}
	if len(overview.LatestGroups) != 1 || overview.LatestGroups[0].CatalogSource != "model_plaza" {
		t.Fatalf("latest groups = %#v", overview.LatestGroups)
	}
	if len(overview.LatestGroups[0].Models) != 1 || overview.LatestGroups[0].Models[0].Name != "gpt-5" {
		t.Fatalf("models = %#v", overview.LatestGroups[0].Models)
	}
}

func usageSnapshot(accountID uint, remoteID string, usedQuota float64, createdAt time.Time) models.TokenSnapshot {
	return models.TokenSnapshot{
		AccountID:    accountID,
		RunID:        1,
		RemoteID:     remoteID,
		Name:         remoteID,
		MaskedKey:    "sk-****",
		Status:       "active",
		GroupKey:     "default",
		GroupName:    "Default",
		UsedQuotaUSD: utils.PtrFloat64(usedQuota),
		CreatedAt:    createdAt,
	}
}
