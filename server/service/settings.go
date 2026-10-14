package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"upstream-x/server/config"
	"upstream-x/server/repository"
)

const (
	smtpEnabledKey     = "smtp.enabled"
	smtpHostKey        = "smtp.host"
	smtpPortKey        = "smtp.port"
	smtpUsernameKey    = "smtp.username"
	smtpPasswordKey    = "smtp.password"
	smtpFromKey        = "smtp.from"
	smtpRecipientsKey  = "smtp.recipients"
	smtpUseTLSKey      = "smtp.use_tls"
	screenLayoutKey    = "screen.layout"
	customMenuLinksKey = "ui.custom_menu_links"
)

type SettingsService struct {
	repo                    *repository.Repository
	defaultInterval         int
	defaultSMTP             config.SMTPConfig
	defaultLogRetentionDays int
}

type SMTPSettingsPayload struct {
	Enabled    bool     `json:"enabled"`
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	From       string   `json:"from"`
	Recipients []string `json:"recipients"`
	UseTLS     bool     `json:"use_tls"`
}

type LogCleanupResult struct {
	RetentionDays int                     `json:"log_retention_days"`
	CleanBefore   time.Time               `json:"clean_before"`
	Deleted       repository.PurgeSummary `json:"deleted"`
	DeletedTotal  int64                   `json:"deleted_total"`
}

type SettingsSnapshot struct {
	ScheduleIntervalMinutes int                 `json:"schedule_interval_minutes"`
	LogRetentionDays        int                 `json:"log_retention_days"`
	SMTP                    SMTPSettingsPayload `json:"smtp"`
	CustomMenuLinks         []CustomMenuLink    `json:"custom_menu_links"`
}

type CustomMenuLink struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	EmbedIframe bool   `json:"embed_iframe"`
}

type CustomMenuLinksSettings struct {
	Items []CustomMenuLink `json:"items"`
}

type ScreenLayoutItem struct {
	AccountID uint `json:"account_id"`
	Collapsed bool `json:"collapsed"`
}

type ScreenLayoutSettings struct {
	Items []ScreenLayoutItem `json:"items"`
}

type SettingsTransferBundle struct {
	Version    string           `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Settings   SettingsSnapshot `json:"settings"`
}

func NewSettingsService(repo *repository.Repository, defaultInterval int, defaultSMTP config.SMTPConfig, defaultLogRetentionDays int) *SettingsService {
	return &SettingsService{
		repo:                    repo,
		defaultInterval:         defaultInterval,
		defaultSMTP:             defaultSMTP,
		defaultLogRetentionDays: defaultLogRetentionDays,
	}
}

func (s *SettingsService) GetScheduleInterval() (int, error) {
	return s.repo.GetScheduleInterval(s.defaultInterval)
}

func (s *SettingsService) UpdateScheduleInterval(minutes int) error {
	return s.repo.SetScheduleInterval(minutes)
}

func (s *SettingsService) GetLogRetentionDays() (int, error) {
	return s.repo.GetLogRetentionDays(s.defaultLogRetentionDays)
}

func (s *SettingsService) UpdateLogRetentionDays(days int) error {
	if days <= 0 {
		return fmt.Errorf("log_retention_days must be > 0")
	}
	return s.repo.SetLogRetentionDays(days)
}

func (s *SettingsService) GetSMTPSettings() (SMTPSettingsPayload, error) {
	recipientsValue, err := s.repo.GetSystemSetting(smtpRecipientsKey)
	if err != nil {
		return SMTPSettingsPayload{}, err
	}

	recipients := append([]string{}, s.defaultSMTP.Recipients...)
	if strings.TrimSpace(recipientsValue) != "" {
		if err := json.Unmarshal([]byte(recipientsValue), &recipients); err != nil {
			return SMTPSettingsPayload{}, fmt.Errorf("parse smtp recipients: %w", err)
		}
	}

	enabled := s.defaultSMTP.Enabled
	if value, err := s.repo.GetSystemSetting(smtpEnabledKey); err != nil {
		return SMTPSettingsPayload{}, err
	} else if strings.TrimSpace(value) != "" {
		enabled = strings.EqualFold(value, "true")
	}

	port := s.defaultSMTP.Port
	if value, err := s.repo.GetSystemSetting(smtpPortKey); err != nil {
		return SMTPSettingsPayload{}, err
	} else if strings.TrimSpace(value) != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return SMTPSettingsPayload{}, fmt.Errorf("parse smtp port: %w", parseErr)
		}
		port = parsed
	}

	useTLS := s.defaultSMTP.UseTLS
	if value, err := s.repo.GetSystemSetting(smtpUseTLSKey); err != nil {
		return SMTPSettingsPayload{}, err
	} else if strings.TrimSpace(value) != "" {
		useTLS = strings.EqualFold(value, "true")
	}

	host, err := s.repo.GetSystemSetting(smtpHostKey)
	if err != nil {
		return SMTPSettingsPayload{}, err
	}
	if strings.TrimSpace(host) == "" {
		host = s.defaultSMTP.Host
	}

	username, err := s.repo.GetSystemSetting(smtpUsernameKey)
	if err != nil {
		return SMTPSettingsPayload{}, err
	}
	if strings.TrimSpace(username) == "" {
		username = s.defaultSMTP.UserName
	}

	password, err := s.repo.GetSystemSetting(smtpPasswordKey)
	if err != nil {
		return SMTPSettingsPayload{}, err
	}
	if strings.TrimSpace(password) == "" {
		password = s.defaultSMTP.Password
	}

	from, err := s.repo.GetSystemSetting(smtpFromKey)
	if err != nil {
		return SMTPSettingsPayload{}, err
	}
	if strings.TrimSpace(from) == "" {
		from = s.defaultSMTP.From
	}

	return SMTPSettingsPayload{
		Enabled:    enabled,
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		From:       from,
		Recipients: recipients,
		UseTLS:     useTLS,
	}, nil
}

func (s *SettingsService) UpdateSMTPSettings(payload SMTPSettingsPayload) error {
	if payload.Port <= 0 {
		return fmt.Errorf("smtp.port must be > 0")
	}

	recipients := make([]string, 0, len(payload.Recipients))
	for _, recipient := range payload.Recipients {
		trimmed := strings.TrimSpace(recipient)
		if trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}
	rawRecipients, err := json.Marshal(recipients)
	if err != nil {
		return fmt.Errorf("marshal smtp recipients: %w", err)
	}

	return s.repo.SetSystemSettings(map[string]string{
		smtpEnabledKey:    strconv.FormatBool(payload.Enabled),
		smtpHostKey:       strings.TrimSpace(payload.Host),
		smtpPortKey:       strconv.Itoa(payload.Port),
		smtpUsernameKey:   strings.TrimSpace(payload.Username),
		smtpPasswordKey:   payload.Password,
		smtpFromKey:       strings.TrimSpace(payload.From),
		smtpRecipientsKey: string(rawRecipients),
		smtpUseTLSKey:     strconv.FormatBool(payload.UseTLS),
	})
}

func (s *SettingsService) Snapshot() (*SettingsSnapshot, error) {
	interval, err := s.GetScheduleInterval()
	if err != nil {
		return nil, err
	}
	logRetentionDays, err := s.GetLogRetentionDays()
	if err != nil {
		return nil, err
	}
	smtp, err := s.GetSMTPSettings()
	if err != nil {
		return nil, err
	}
	customMenuLinks, err := s.GetCustomMenuLinks()
	if err != nil {
		return nil, err
	}
	return &SettingsSnapshot{
		ScheduleIntervalMinutes: interval,
		LogRetentionDays:        logRetentionDays,
		SMTP:                    smtp,
		CustomMenuLinks:         customMenuLinks,
	}, nil
}

func (s *SettingsService) GetScreenLayout() (*ScreenLayoutSettings, error) {
	raw, err := s.repo.GetSystemSetting(screenLayoutKey)
	if err != nil {
		return nil, err
	}
	layout := &ScreenLayoutSettings{Items: []ScreenLayoutItem{}}
	if strings.TrimSpace(raw) == "" {
		return layout, nil
	}
	if err := json.Unmarshal([]byte(raw), layout); err != nil {
		return nil, fmt.Errorf("parse screen layout: %w", err)
	}
	if layout.Items == nil {
		layout.Items = []ScreenLayoutItem{}
	}
	return layout, nil
}

func (s *SettingsService) UpdateScreenLayout(payload ScreenLayoutSettings) (*ScreenLayoutSettings, error) {
	normalized := ScreenLayoutSettings{Items: make([]ScreenLayoutItem, 0, len(payload.Items))}
	seen := make(map[uint]struct{}, len(payload.Items))
	for _, item := range payload.Items {
		if item.AccountID == 0 {
			return nil, fmt.Errorf("account_id must be > 0")
		}
		if _, ok := seen[item.AccountID]; ok {
			continue
		}
		seen[item.AccountID] = struct{}{}
		normalized.Items = append(normalized.Items, item)
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal screen layout: %w", err)
	}
	if err := s.repo.SetSystemSettings(map[string]string{
		screenLayoutKey: string(raw),
	}); err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (s *SettingsService) GetCustomMenuLinks() ([]CustomMenuLink, error) {
	raw, err := s.repo.GetSystemSetting(customMenuLinksKey)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return []CustomMenuLink{}, nil
	}

	var items []CustomMenuLink
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("parse custom menu links: %w", err)
	}
	if items == nil {
		return []CustomMenuLink{}, nil
	}
	return items, nil
}

func (s *SettingsService) UpdateCustomMenuLinks(payload CustomMenuLinksSettings) (*CustomMenuLinksSettings, error) {
	normalized := make([]CustomMenuLink, 0, len(payload.Items))
	seenIDs := make(map[string]struct{}, len(payload.Items))

	for _, item := range payload.Items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			return nil, fmt.Errorf("custom menu link title is required")
		}

		linkURL, err := normalizeMenuLinkURL(item.URL)
		if err != nil {
			return nil, err
		}

		id := strings.TrimSpace(item.ID)
		if id == "" {
			id, err = newCustomMenuLinkID()
			if err != nil {
				return nil, err
			}
		}
		if _, ok := seenIDs[id]; ok {
			return nil, fmt.Errorf("duplicate custom menu link id: %s", id)
		}
		seenIDs[id] = struct{}{}

		normalized = append(normalized, CustomMenuLink{
			ID:          id,
			Title:       title,
			URL:         linkURL,
			EmbedIframe: item.EmbedIframe,
		})
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal custom menu links: %w", err)
	}
	if err := s.repo.SetSystemSettings(map[string]string{
		customMenuLinksKey: string(raw),
	}); err != nil {
		return nil, err
	}
	return &CustomMenuLinksSettings{Items: normalized}, nil
}

func (s *SettingsService) ExportBundle() (*SettingsTransferBundle, error) {
	snapshot, err := s.Snapshot()
	if err != nil {
		return nil, err
	}

	return &SettingsTransferBundle{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Settings:   *snapshot,
	}, nil
}

func (s *SettingsService) ImportBundle(bundle SettingsTransferBundle) (*SettingsSnapshot, error) {
	if bundle.Settings.ScheduleIntervalMinutes <= 0 {
		return nil, fmt.Errorf("schedule_interval_minutes must be > 0")
	}
	if bundle.Settings.LogRetentionDays <= 0 {
		return nil, fmt.Errorf("log_retention_days must be > 0")
	}
	if err := s.UpdateScheduleInterval(bundle.Settings.ScheduleIntervalMinutes); err != nil {
		return nil, err
	}
	if err := s.UpdateLogRetentionDays(bundle.Settings.LogRetentionDays); err != nil {
		return nil, err
	}
	if err := s.UpdateSMTPSettings(bundle.Settings.SMTP); err != nil {
		return nil, err
	}
	if _, err := s.UpdateCustomMenuLinks(CustomMenuLinksSettings{
		Items: bundle.Settings.CustomMenuLinks,
	}); err != nil {
		return nil, err
	}

	return s.Snapshot()
}

func (s *SettingsService) PurgeLogs(retentionDays int) (*LogCleanupResult, error) {
	if retentionDays <= 0 {
		return nil, fmt.Errorf("log_retention_days must be > 0")
	}
	before := time.Now().AddDate(0, 0, -retentionDays)
	summary, err := s.repo.PurgeLogsBefore(before)
	if err != nil {
		return nil, err
	}
	return &LogCleanupResult{
		RetentionDays: retentionDays,
		CleanBefore:   before,
		Deleted:       *summary,
		DeletedTotal:  summary.Total(),
	}, nil
}

func normalizeMenuLinkURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("custom menu link url is required")
	}
	if strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "//") {
		return trimmed, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse custom menu link url: %w", err)
	}
	if !parsed.IsAbs() {
		return "", fmt.Errorf("custom menu link url must start with http://, https://, or /")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("custom menu link url must use http or https")
	}
	return trimmed, nil
}

func newCustomMenuLinkID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate custom menu link id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
