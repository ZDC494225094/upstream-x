package repository

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"upstream-x/server/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const scheduleIntervalKey = "scheduler.interval_minutes"
const logRetentionDaysKey = "logs.retention_days"
const smtpEnabledKey = "smtp.enabled"
const smtpHostKey = "smtp.host"
const smtpPortKey = "smtp.port"
const smtpUsernameKey = "smtp.username"
const smtpPasswordKey = "smtp.password"
const smtpFromKey = "smtp.from"
const smtpRecipientsKey = "smtp.recipients"
const smtpUseTLSKey = "smtp.use_tls"
const screenLayoutKey = "screen.layout"

type Repository struct {
	db *gorm.DB
}

type TokenUsageDelta struct {
	AccountID    uint      `gorm:"column:account_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UsedQuotaUSD float64   `gorm:"column:used_quota_usd"`
}

type PurgeSummary struct {
	BalanceSnapshots int64 `json:"balance_snapshots"`
	GroupSnapshots   int64 `json:"group_snapshots"`
	TokenSnapshots   int64 `json:"token_snapshots"`
	AlertEvents      int64 `json:"alert_events"`
	DetectionRuns    int64 `json:"detection_runs"`
}

func (p PurgeSummary) Total() int64 {
	return p.BalanceSnapshots + p.GroupSnapshots + p.TokenSnapshots + p.AlertEvents + p.DetectionRuns
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) ListAccounts() ([]models.UpstreamAccount, error) {
	var accounts []models.UpstreamAccount
	if err := r.db.Order("created_at desc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *Repository) ListEnabledAccounts() ([]models.UpstreamAccount, error) {
	var accounts []models.UpstreamAccount
	if err := r.db.Where("enabled = ?", true).Order("created_at asc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *Repository) GetAccount(id uint) (*models.UpstreamAccount, error) {
	var account models.UpstreamAccount
	if err := r.db.First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) FindAccountByNameAndType(name string, accountType models.UpstreamType) (*models.UpstreamAccount, error) {
	var account models.UpstreamAccount
	err := r.db.Where("name = ? AND type = ?", name, accountType).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) CreateAccount(account *models.UpstreamAccount) error {
	return r.db.Create(account).Error
}

func (r *Repository) UpdateAccount(account *models.UpstreamAccount) error {
	return r.db.Save(account).Error
}

func (r *Repository) DeleteAccount(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ?", id).Delete(&models.GroupCatalog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", id).Delete(&models.DailyUsageSnapshot{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.UpstreamAccount{}, id).Error
	})
}

func (r *Repository) SaveRunBundle(run *models.DetectionRun, balance *models.BalanceSnapshot, groups []models.GroupSnapshot, catalogs []models.GroupCatalog, dailyUsage []models.DailyUsageSnapshot, tokens []models.TokenSnapshot, alerts []models.AlertEvent, accountUpdate map[string]any) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(run).Error; err != nil {
			return err
		}

		if balance != nil {
			balance.RunID = run.ID
			if err := tx.Create(balance).Error; err != nil {
				return err
			}
		}

		for idx := range groups {
			groups[idx].RunID = run.ID
		}
		if len(groups) > 0 {
			if err := tx.Create(&groups).Error; err != nil {
				return err
			}
		}

		// Keep the last known Model Plaza models when a single check falls back
		// to groups/available (for example, during a temporary plaza outage).
		var previousCatalogs []models.GroupCatalog
		if err := tx.Where("account_id = ?", run.AccountID).Find(&previousCatalogs).Error; err != nil {
			return err
		}
		previousByKey := make(map[string]models.GroupCatalog, len(previousCatalogs))
		for _, catalog := range previousCatalogs {
			previousByKey[catalog.GroupKey] = catalog
		}
		for index := range catalogs {
			if catalogs[index].Source == "groups_api" && catalogs[index].ModelsCount == 0 {
				if previous, ok := previousByKey[catalogs[index].GroupKey]; ok && previous.Source == "model_plaza" && previous.ModelsCount > 0 {
					catalogs[index].Source = previous.Source
					catalogs[index].ModelsJSON = previous.ModelsJSON
					catalogs[index].ModelsCount = previous.ModelsCount
				}
			}
		}
		if err := tx.Where("account_id = ?", run.AccountID).Delete(&models.GroupCatalog{}).Error; err != nil {
			return err
		}
		for idx := range catalogs {
			catalogs[idx].AccountID = run.AccountID
		}
		if len(catalogs) > 0 {
			if err := tx.Create(&catalogs).Error; err != nil {
				return err
			}
		}

		if len(dailyUsage) == 0 {
			if err := tx.Where("account_id = ?", run.AccountID).Delete(&models.DailyUsageSnapshot{}).Error; err != nil {
				return err
			}
		} else {
			for index := range dailyUsage {
				dailyUsage[index].AccountID = run.AccountID
				if dailyUsage[index].UpdatedAt.IsZero() {
					dailyUsage[index].UpdatedAt = time.Now()
				}
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "account_id"}, {Name: "date"}},
				DoUpdates: clause.AssignmentColumns([]string{"used_quota_usd", "source", "updated_at"}),
			}).Create(&dailyUsage).Error; err != nil {
				return err
			}
		}

		for idx := range tokens {
			tokens[idx].RunID = run.ID
		}
		if len(tokens) > 0 {
			if err := tx.Create(&tokens).Error; err != nil {
				return err
			}
		}

		for idx := range alerts {
			alerts[idx].RunID = &run.ID
		}
		if len(alerts) > 0 {
			if err := tx.Create(&alerts).Error; err != nil {
				return err
			}
		}

		if len(accountUpdate) > 0 {
			if err := tx.Model(&models.UpstreamAccount{}).Where("id = ?", run.AccountID).Updates(accountUpdate).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) SaveFailedRun(run *models.DetectionRun, alerts []models.AlertEvent, accountUpdate map[string]any) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", run.AccountID).Delete(&models.DailyUsageSnapshot{}).Error; err != nil {
			return err
		}

		for idx := range alerts {
			alerts[idx].RunID = &run.ID
		}
		if len(alerts) > 0 {
			if err := tx.Create(&alerts).Error; err != nil {
				return err
			}
		}

		if len(accountUpdate) > 0 {
			if err := tx.Model(&models.UpstreamAccount{}).Where("id = ?", run.AccountID).Updates(accountUpdate).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *Repository) GetLatestSuccessfulRun(accountID uint) (*models.DetectionRun, error) {
	var run models.DetectionRun
	err := r.db.Where("account_id = ? AND status = ?", accountID, models.RunStatusSuccess).
		Order("created_at desc").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) GetPreviousSuccessfulRun(accountID uint, beforeID uint) (*models.DetectionRun, error) {
	var run models.DetectionRun
	err := r.db.Where("account_id = ? AND status = ? AND id < ?", accountID, models.RunStatusSuccess, beforeID).
		Order("id desc").
		First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *Repository) GetGroupSnapshotsByRun(runID uint) ([]models.GroupSnapshot, error) {
	var rows []models.GroupSnapshot
	if err := r.db.Where("run_id = ?", runID).Order("group_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetTokenSnapshotsByRun(runID uint) ([]models.TokenSnapshot, error) {
	var rows []models.TokenSnapshot
	if err := r.db.Where("run_id = ?", runID).Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListGroupCatalogs(accountID uint) ([]models.GroupCatalog, error) {
	var rows []models.GroupCatalog
	if err := r.db.Where("account_id = ?", accountID).Order("group_name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListTokenUsageDeltas(since time.Time) ([]TokenUsageDelta, error) {
	query := `
WITH ordered AS (
	SELECT
		id,
		account_id,
		created_at,
		used_quota_usd,
		LAG(used_quota_usd) OVER (
			PARTITION BY account_id, remote_id
			ORDER BY created_at ASC, id ASC
		) AS previous_used_quota_usd
	FROM token_snapshots
	WHERE used_quota_usd IS NOT NULL
)
SELECT
	account_id,
	created_at,
	CASE
		WHEN previous_used_quota_usd IS NULL THEN 0
		WHEN used_quota_usd > previous_used_quota_usd THEN used_quota_usd - previous_used_quota_usd
		ELSE 0
	END AS used_quota_usd
FROM ordered`

	args := []any{}
	if !since.IsZero() {
		query += " WHERE created_at >= ?"
		args = append(args, since)
	}
	query += " ORDER BY created_at ASC, id ASC"

	var rows []TokenUsageDelta
	if err := r.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListDailyUsage(since, until string) ([]models.DailyUsageSnapshot, error) {
	var rows []models.DailyUsageSnapshot
	query := r.db.Order("date asc, account_id asc")
	if since != "" {
		query = query.Where("date >= ?", since)
	}
	if until != "" {
		query = query.Where("date < ?", until)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListDetectionRuns(accountID uint, limit int) ([]models.DetectionRun, error) {
	var rows []models.DetectionRun
	query := r.db.Order("created_at desc")
	if accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListBalanceSnapshots(accountID uint, since time.Time) ([]models.BalanceSnapshot, error) {
	var rows []models.BalanceSnapshot
	query := r.db.Where("account_id = ?", accountID).Order("created_at asc")
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListGroupSnapshots(accountID uint, since time.Time) ([]models.GroupSnapshot, error) {
	var rows []models.GroupSnapshot
	query := r.db.Where("account_id = ?", accountID).Order("created_at asc, group_name asc")
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) ListAlerts(accountID uint, limit int) ([]models.AlertEvent, error) {
	var rows []models.AlertEvent
	query := r.db.Order("created_at desc")
	if accountID > 0 {
		query = query.Where("account_id = ?", accountID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) GetScheduleInterval(defaultValue int) (int, error) {
	var setting models.SystemSetting
	err := r.db.First(&setting, "key = ?", scheduleIntervalKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultValue, nil
	}
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 0, fmt.Errorf("parse schedule interval: %w", err)
	}
	return value, nil
}

func (r *Repository) SetScheduleInterval(minutes int) error {
	return r.db.Save(&models.SystemSetting{
		Key:       scheduleIntervalKey,
		Value:     strconv.Itoa(minutes),
		UpdatedAt: time.Now(),
	}).Error
}

func (r *Repository) GetLogRetentionDays(defaultValue int) (int, error) {
	var setting models.SystemSetting
	err := r.db.First(&setting, "key = ?", logRetentionDaysKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultValue, nil
	}
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(setting.Value)
	if err != nil {
		return 0, fmt.Errorf("parse log retention days: %w", err)
	}
	return value, nil
}

func (r *Repository) SetLogRetentionDays(days int) error {
	return r.db.Save(&models.SystemSetting{
		Key:       logRetentionDaysKey,
		Value:     strconv.Itoa(days),
		UpdatedAt: time.Now(),
	}).Error
}

func (r *Repository) GetSystemSetting(key string) (string, error) {
	var setting models.SystemSetting
	err := r.db.First(&setting, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *Repository) SetSystemSettings(values map[string]string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for key, value := range values {
			if err := tx.Save(&models.SystemSetting{
				Key:       key,
				Value:     value,
				UpdatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) PurgeLogsBefore(before time.Time) (*PurgeSummary, error) {
	summary := &PurgeSummary{}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("created_at < ?", before).Delete(&models.BalanceSnapshot{})
		if result.Error != nil {
			return result.Error
		}
		summary.BalanceSnapshots = result.RowsAffected

		result = tx.Where("created_at < ?", before).Delete(&models.GroupSnapshot{})
		if result.Error != nil {
			return result.Error
		}
		summary.GroupSnapshots = result.RowsAffected

		result = tx.Where("created_at < ?", before).Delete(&models.TokenSnapshot{})
		if result.Error != nil {
			return result.Error
		}
		summary.TokenSnapshots = result.RowsAffected

		result = tx.Where("created_at < ?", before).Delete(&models.AlertEvent{})
		if result.Error != nil {
			return result.Error
		}
		summary.AlertEvents = result.RowsAffected

		result = tx.Where("created_at < ?", before).Delete(&models.DetectionRun{})
		if result.Error != nil {
			return result.Error
		}
		summary.DetectionRuns = result.RowsAffected

		return nil
	})
	if err != nil {
		return nil, err
	}

	return summary, nil
}
