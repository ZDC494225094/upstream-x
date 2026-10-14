package models

import (
	"time"

	"gorm.io/gorm"
)

type UpstreamType string

type Sub2APIAuthMode string
type NewAPIAuthMode string

const (
	UpstreamTypeSub2API UpstreamType = "sub2api"
	UpstreamTypeNewAPI  UpstreamType = "newapi"

	Sub2APIAuthModeRefreshToken Sub2APIAuthMode = "refresh_token"
	Sub2APIAuthModePassword     Sub2APIAuthMode = "password"

	NewAPIAuthModeManual NewAPIAuthMode = "manual"
)

type RunStatus string

const (
	RunStatusSuccess RunStatus = "success"
	RunStatusFailed  RunStatus = "failed"
)

type AlertType string

const (
	AlertTypeLowBalance      AlertType = "low_balance"
	AlertTypeBalanceRecovery AlertType = "balance_recovery"
	AlertTypeAccountError    AlertType = "account_error"
	AlertTypeAccountRecover  AlertType = "account_recovery"
	AlertTypeGroupChanged    AlertType = "group_changed"
)

type UpstreamAccount struct {
	ID                       uint            `gorm:"primarykey" json:"id"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	DeletedAt                gorm.DeletedAt  `gorm:"index" json:"-"`
	Name                     string          `gorm:"size:120;not null" json:"name"`
	Remark                   string          `gorm:"size:500" json:"remark"`
	Type                     UpstreamType    `gorm:"size:20;not null;index" json:"type"`
	BaseURL                  string          `gorm:"size:255;not null" json:"base_url"`
	Sub2APIAuthMode          Sub2APIAuthMode `gorm:"size:30;not null;default:refresh_token" json:"sub2api_auth_mode"`
	Sub2APILoginBodyTemplate string          `gorm:"type:text" json:"sub2api_login_body_template"`
	NewAPIAuthMode           NewAPIAuthMode  `gorm:"size:30;not null;default:manual" json:"newapi_auth_mode"`
	BearerTokenEncrypted     string          `gorm:"type:text;not null" json:"-"`
	RefreshTokenEncrypted    string          `gorm:"type:text" json:"-"`
	Sub2APIEmailEncrypted    string          `gorm:"type:text" json:"-"`
	Sub2APIPasswordEncrypted string          `gorm:"type:text" json:"-"`
	NewAPIUserEncrypted      string          `gorm:"type:text" json:"-"`
	LowBalanceThresholdUSD   float64         `gorm:"not null" json:"low_balance_threshold_usd"`
	Enabled                  bool            `gorm:"default:true" json:"enabled"`
	LastStatus               string          `gorm:"size:30" json:"last_status"`
	LastError                string          `gorm:"type:text" json:"last_error"`
	LastCheckedAt            *time.Time      `json:"last_checked_at"`
	LastBalanceUSD           *float64        `json:"last_balance_usd"`
}

type SystemSetting struct {
	Key       string    `gorm:"primarykey;size:80"`
	Value     string    `gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DetectionRun struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	AccountID      uint      `gorm:"index;not null" json:"account_id"`
	Status         RunStatus `gorm:"size:20;not null" json:"status"`
	ErrorMessage   string    `gorm:"type:text" json:"error_message"`
	BalanceUSD     *float64  `json:"balance_usd"`
	GroupsCount    int       `json:"groups_count"`
	TokensCount    int       `json:"tokens_count"`
	TriggeredBy    string    `gorm:"size:30;not null" json:"triggered_by"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	DurationMillis int64     `json:"duration_millis"`
	CreatedAt      time.Time `json:"created_at"`
}

type BalanceSnapshot struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	AccountID  uint      `gorm:"index;not null" json:"account_id"`
	RunID      uint      `gorm:"index;not null" json:"run_id"`
	BalanceUSD float64   `gorm:"not null" json:"balance_usd"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

type GroupSnapshot struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	AccountID       uint      `gorm:"index;not null" json:"account_id"`
	RunID           uint      `gorm:"index;not null" json:"run_id"`
	GroupKey        string    `gorm:"size:191;index;not null" json:"group_key"`
	GroupName       string    `gorm:"size:191;not null" json:"group_name"`
	Description     string    `gorm:"type:text" json:"description"`
	RateMultiplier  float64   `gorm:"not null" json:"rate_multiplier"`
	ImagePrice1K    *float64  `json:"image_price_1k"`
	ImagePrice2K    *float64  `json:"image_price_2k"`
	ImagePrice4K    *float64  `json:"image_price_4k"`
	TokenUsageCount int       `gorm:"not null" json:"token_usage_count"`
	IsUsedByToken   bool      `gorm:"not null" json:"is_used_by_token"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
}

type GroupCatalog struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	AccountID   uint      `gorm:"uniqueIndex:idx_group_catalog_account_key,priority:1;not null" json:"account_id"`
	GroupKey    string    `gorm:"size:191;uniqueIndex:idx_group_catalog_account_key,priority:2;not null" json:"group_key"`
	GroupName   string    `gorm:"size:191;not null" json:"group_name"`
	Platform    string    `gorm:"size:80" json:"platform"`
	Source      string    `gorm:"size:40;not null" json:"source"`
	ModelsJSON  string    `gorm:"type:text;not null" json:"-"`
	ModelsCount int       `gorm:"not null" json:"models_count"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DailyUsageSnapshot stores an upstream's authoritative daily actual cost.
// Newer Sub2API versions expose this without requiring per-key aggregation.
type DailyUsageSnapshot struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	AccountID    uint      `gorm:"uniqueIndex:idx_daily_usage_account_date,priority:1;index;not null" json:"account_id"`
	Date         string    `gorm:"size:10;uniqueIndex:idx_daily_usage_account_date,priority:2;not null" json:"date"`
	UsedQuotaUSD float64   `gorm:"not null" json:"used_quota_usd"`
	Source       string    `gorm:"size:40;not null" json:"source"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TokenSnapshot struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	AccountID    uint       `gorm:"index;index:idx_token_snapshots_usage,priority:1;not null" json:"account_id"`
	RunID        uint       `gorm:"index;not null" json:"run_id"`
	RemoteID     string     `gorm:"size:120;index;index:idx_token_snapshots_usage,priority:2;not null" json:"remote_id"`
	Name         string     `gorm:"size:191;not null" json:"name"`
	MaskedKey    string     `gorm:"size:191;not null" json:"masked_key"`
	Status       string     `gorm:"size:50;not null" json:"status"`
	GroupKey     string     `gorm:"size:191;index;not null" json:"group_key"`
	GroupName    string     `gorm:"size:191;not null" json:"group_name"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	UsedQuotaUSD *float64   `json:"used_quota_usd"`
	CreatedAt    time.Time  `gorm:"index;index:idx_token_snapshots_usage,priority:3" json:"created_at"`
}

type AlertEvent struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	AccountID    uint      `gorm:"index;not null" json:"account_id"`
	RunID        *uint     `gorm:"index" json:"run_id"`
	Type         AlertType `gorm:"size:40;index;not null" json:"type"`
	Title        string    `gorm:"size:255;not null" json:"title"`
	Message      string    `gorm:"type:text;not null" json:"message"`
	Severity     string    `gorm:"size:20;not null" json:"severity"`
	GroupKey     string    `gorm:"size:191;index" json:"group_key"`
	PrevValue    string    `gorm:"type:text" json:"prev_value"`
	CurrentValue string    `gorm:"type:text" json:"current_value"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&UpstreamAccount{},
		&SystemSetting{},
		&DetectionRun{},
		&BalanceSnapshot{},
		&GroupSnapshot{},
		&GroupCatalog{},
		&DailyUsageSnapshot{},
		&TokenSnapshot{},
		&AlertEvent{},
	); err != nil {
		return err
	}
	if err := db.Model(&UpstreamAccount{}).
		Where(&UpstreamAccount{Type: UpstreamTypeSub2API, Sub2APIAuthMode: "server_browser"}).
		Update("Sub2APIAuthMode", Sub2APIAuthModeRefreshToken).Error; err != nil {
		return err
	}
	return db.Model(&UpstreamAccount{}).
		Where(&UpstreamAccount{Type: UpstreamTypeNewAPI, NewAPIAuthMode: "server_browser"}).
		Update("NewAPIAuthMode", NewAPIAuthModeManual).Error
}
