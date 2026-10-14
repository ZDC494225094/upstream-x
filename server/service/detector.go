package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"upstream-x/server/mailer"
	"upstream-x/server/models"
	"upstream-x/server/repository"
	"upstream-x/server/upstream"
	"upstream-x/server/utils"
)

type DetectorService struct {
	repo     *repository.Repository
	accounts *AccountService
	client   *upstream.Client
	mailer   *mailer.Mailer
	settings *SettingsService
	// accountLocks serializes checks per account: Sub2API refresh tokens are
	// single-use rotating tokens, so two concurrent renewals for the same
	// account would burn each other's token.
	accountLocks sync.Map // uint -> *sync.Mutex
}

func (s *DetectorService) lockAccount(id uint) func() {
	value, _ := s.accountLocks.LoadOrStore(id, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

const maxConcurrentChecks = 4

func NewDetectorService(repo *repository.Repository, accounts *AccountService, client *upstream.Client, mailer *mailer.Mailer, settings *SettingsService) *DetectorService {
	return &DetectorService{
		repo:     repo,
		accounts: accounts,
		client:   client,
		mailer:   mailer,
		settings: settings,
	}
}

// ValidateAccount fetches a snapshot and also returns the credentials that
// were actually used. For Sub2API accounts the renewal step may rotate the
// refresh token; the returned credentials carry the NEW token so callers
// validating a not-yet-saved (transient) account can persist it instead of
// the now-burned one from the form.
func (s *DetectorService) ValidateAccount(ctx context.Context, account *models.UpstreamAccount) (*upstream.Snapshot, *upstream.AccountCredentials, error) {
	unlock := s.lockAccount(account.ID)
	defer unlock()

	creds, err := s.credentialsFor(account)
	if err != nil {
		return nil, nil, err
	}
	snapshot, used, err := s.fetchSnapshot(ctx, account, creds)
	if err != nil {
		return nil, nil, err
	}
	return snapshot, used, nil
}

func (s *DetectorService) CheckAccount(ctx context.Context, account *models.UpstreamAccount, triggeredBy string) error {
	startedAt := time.Now()
	snapshot, _, err := s.ValidateAccount(ctx, account)
	if err != nil {
		run := &models.DetectionRun{
			AccountID:      account.ID,
			Status:         models.RunStatusFailed,
			ErrorMessage:   err.Error(),
			GroupsCount:    0,
			TokensCount:    0,
			TriggeredBy:    triggeredBy,
			StartedAt:      startedAt,
			CompletedAt:    time.Now(),
			DurationMillis: time.Since(startedAt).Milliseconds(),
		}
		alerts := s.buildFailureAlerts(account, err)
		update := map[string]any{
			"last_status":     string(models.RunStatusFailed),
			"last_error":      err.Error(),
			"last_checked_at": time.Now(),
		}
		if saveErr := s.repo.SaveFailedRun(run, alerts, update); saveErr != nil {
			return fmt.Errorf("save failed run: %w", saveErr)
		}
		s.dispatchAlerts(alerts)
		return err
	}

	groupUsage := map[string]int{}
	for _, token := range snapshot.Tokens {
		groupUsage[token.GroupKey]++
	}

	groups := make([]models.GroupSnapshot, 0, len(snapshot.Groups))
	catalogs := make([]models.GroupCatalog, 0, len(snapshot.Groups))
	for _, group := range snapshot.Groups {
		groups = append(groups, models.GroupSnapshot{
			AccountID:       account.ID,
			GroupKey:        group.Key,
			GroupName:       group.Name,
			Description:     group.Description,
			RateMultiplier:  group.RateMultiplier,
			ImagePrice1K:    group.ImagePrice1K,
			ImagePrice2K:    group.ImagePrice2K,
			ImagePrice4K:    group.ImagePrice4K,
			TokenUsageCount: groupUsage[group.Key],
			IsUsedByToken:   groupUsage[group.Key] > 0,
			CreatedAt:       time.Now(),
		})
		groupModels := group.Models
		if groupModels == nil {
			groupModels = make([]upstream.ModelData, 0)
		}
		modelsJSON, err := json.Marshal(groupModels)
		if err != nil {
			return fmt.Errorf("marshal group model catalog: %w", err)
		}
		source := group.CatalogSource
		if source == "" {
			source = "groups_api"
		}
		catalogs = append(catalogs, models.GroupCatalog{
			GroupKey:    group.Key,
			GroupName:   group.Name,
			Platform:    group.Platform,
			Source:      source,
			ModelsJSON:  string(modelsJSON),
			ModelsCount: len(groupModels),
			UpdatedAt:   time.Now(),
		})
	}

	dailyUsage := make([]models.DailyUsageSnapshot, 0, len(snapshot.DailyUsage))
	if snapshot.DailyUsageEnabled {
		source := snapshot.DailyUsageSource
		if source == "" {
			source = "sub2api_trend"
		}
		for _, point := range snapshot.DailyUsage {
			if _, err := time.Parse("2006-01-02", point.Date); err != nil {
				continue
			}
			value := point.UsedQuotaUSD
			if value < 0 {
				value = 0
			}
			dailyUsage = append(dailyUsage, models.DailyUsageSnapshot{
				AccountID:    account.ID,
				Date:         point.Date,
				UsedQuotaUSD: value,
				Source:       source,
				UpdatedAt:    time.Now(),
			})
		}
	}

	tokens := make([]models.TokenSnapshot, 0, len(snapshot.Tokens))
	for _, token := range snapshot.Tokens {
		tokens = append(tokens, models.TokenSnapshot{
			AccountID:    account.ID,
			RemoteID:     token.RemoteID,
			Name:         token.Name,
			MaskedKey:    token.MaskedKey,
			Status:       token.Status,
			GroupKey:     token.GroupKey,
			GroupName:    token.GroupName,
			LastUsedAt:   token.LastUsedAt,
			UsedQuotaUSD: token.UsedQuota,
			CreatedAt:    time.Now(),
		})
	}

	run := &models.DetectionRun{
		AccountID:      account.ID,
		Status:         models.RunStatusSuccess,
		BalanceUSD:     utils.PtrFloat64(snapshot.BalanceUSD),
		GroupsCount:    len(groups),
		TokensCount:    len(tokens),
		TriggeredBy:    triggeredBy,
		StartedAt:      startedAt,
		CompletedAt:    time.Now(),
		DurationMillis: time.Since(startedAt).Milliseconds(),
	}
	balance := &models.BalanceSnapshot{
		AccountID:  account.ID,
		BalanceUSD: snapshot.BalanceUSD,
		CreatedAt:  time.Now(),
	}

	alerts, err := s.buildSuccessAlerts(account, snapshot.BalanceUSD, groups)
	if err != nil {
		return err
	}
	update := map[string]any{
		"last_status":      string(models.RunStatusSuccess),
		"last_error":       "",
		"last_checked_at":  time.Now(),
		"last_balance_usd": snapshot.BalanceUSD,
	}
	if err := s.repo.SaveRunBundle(run, balance, groups, catalogs, dailyUsage, tokens, alerts, update); err != nil {
		return fmt.Errorf("save run bundle: %w", err)
	}
	s.dispatchAlerts(alerts)
	return nil
}

func (s *DetectorService) CheckAllEnabled(ctx context.Context, triggeredBy string) error {
	accounts, err := s.repo.ListEnabledAccounts()
	if err != nil {
		return err
	}

	workerCount := maxConcurrentChecks
	if len(accounts) > 0 && len(accounts) < workerCount {
		workerCount = len(accounts)
	}

	sem := make(chan struct{}, workerCount)
	var wg sync.WaitGroup

	for _, account := range accounts {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(account models.UpstreamAccount) {
			defer wg.Done()
			defer func() {
				<-sem
			}()

			if err := s.CheckAccount(ctx, &account, triggeredBy); err != nil {
				log.Printf("check account %d failed: %v", account.ID, err)
			}
		}(account)
	}

	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}

	if s.settings != nil {
		retentionDays, retentionErr := s.settings.GetLogRetentionDays()
		if retentionErr != nil {
			log.Printf("get log retention days failed: %v", retentionErr)
		} else if _, purgeErr := s.settings.PurgeLogs(retentionDays); purgeErr != nil {
			log.Printf("purge expired logs failed: %v", purgeErr)
		}
	}
	return nil
}

func (s *DetectorService) RefreshScreenOverview(ctx context.Context, triggeredBy string) (*ScreenOverview, error) {
	if err := s.CheckAllEnabled(ctx, triggeredBy); err != nil {
		return nil, err
	}
	return s.ScreenOverview()
}

type DashboardSummary struct {
	AccountsTotal      int                    `json:"accounts_total"`
	AccountsEnabled    int                    `json:"accounts_enabled"`
	LowBalanceAccounts int                    `json:"low_balance_accounts"`
	FailedAccounts     int                    `json:"failed_accounts"`
	LatestRuns         []DetectionRunSummary  `json:"latest_runs"`
	RecentAlerts       []AlertEventSummary    `json:"recent_alerts"`
	AccountBalances    []AccountBalanceSeries `json:"account_balances"`
}

type DailyUsagePoint struct {
	Date         string  `json:"date"`
	UsedQuotaUSD float64 `json:"used_quota_usd"`
}

type DailyUsageSummary struct {
	TotalUsedQuotaUSD float64           `json:"total_used_quota_usd"`
	TodayUsedQuotaUSD float64           `json:"today_used_quota_usd"`
	Points            []DailyUsagePoint `json:"points"`
}

type DetectionRunSummary struct {
	models.DetectionRun
	AccountName string              `json:"account_name"`
	AccountType models.UpstreamType `json:"account_type"`
}

type AlertEventSummary struct {
	models.AlertEvent
	AccountName string              `json:"account_name"`
	AccountType models.UpstreamType `json:"account_type"`
}

type ScreenGroupSummary struct {
	GroupKey         string   `json:"group_key"`
	GroupName        string   `json:"group_name"`
	RateMultiplier   float64  `json:"rate_multiplier"`
	TokenUsageCount  int      `json:"token_usage_count"`
	UsedQuotaUSD     *float64 `json:"used_quota_usd"`
	HasObservedUsage bool     `json:"has_observed_usage"`
}

type ScreenAccountSummary struct {
	AccountID              uint                 `json:"account_id"`
	AccountName            string               `json:"account_name"`
	AccountRemark          string               `json:"account_remark"`
	Type                   models.UpstreamType  `json:"type"`
	BaseURL                string               `json:"base_url"`
	Enabled                bool                 `json:"enabled"`
	LastStatus             string               `json:"last_status"`
	LastError              string               `json:"last_error"`
	LastCheckedAt          *time.Time           `json:"last_checked_at"`
	LastBalanceUSD         *float64             `json:"last_balance_usd"`
	LowBalanceThresholdUSD float64              `json:"low_balance_threshold_usd"`
	UsedGroups             []ScreenGroupSummary `json:"used_groups"`
}

type ScreenOverview struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	AccountsTotal   int                    `json:"accounts_total"`
	AccountsEnabled int                    `json:"accounts_enabled"`
	LowBalanceCount int                    `json:"low_balance_count"`
	UsedGroupsCount int                    `json:"used_groups_count"`
	Items           []ScreenAccountSummary `json:"items"`
}

type AccountBalanceSeries struct {
	AccountID   uint                `json:"account_id"`
	AccountName string              `json:"account_name"`
	Type        models.UpstreamType `json:"type"`
	Points      []BalancePoint      `json:"points"`
}

type BalancePoint struct {
	Time       time.Time `json:"time"`
	BalanceUSD float64   `json:"balance_usd"`
}

func (s *DetectorService) Dashboard(rangeValue string) (*DashboardSummary, error) {
	accounts, err := s.repo.ListAccounts()
	if err != nil {
		return nil, err
	}
	runs, err := s.repo.ListDetectionRuns(0, 10)
	if err != nil {
		return nil, err
	}
	alerts, err := s.repo.ListAlerts(0, 20)
	if err != nil {
		return nil, err
	}

	since := utils.RangeStart(rangeValue)
	series := make([]AccountBalanceSeries, 0, len(accounts))
	runSummaries := make([]DetectionRunSummary, 0, len(runs))
	alertSummaries := make([]AlertEventSummary, 0, len(alerts))
	lowBalance := 0
	failed := 0
	enabled := 0
	accountMap := make(map[uint]models.UpstreamAccount, len(accounts))
	for _, account := range accounts {
		accountMap[account.ID] = account
		if account.Enabled {
			enabled++
		}
		if account.LastStatus == string(models.RunStatusFailed) {
			failed++
		}
		if account.LastBalanceUSD != nil && *account.LastBalanceUSD <= account.LowBalanceThresholdUSD {
			lowBalance++
		}
		balances, err := s.repo.ListBalanceSnapshots(account.ID, since)
		if err != nil {
			return nil, err
		}
		points := make([]BalancePoint, 0, len(balances))
		for _, row := range balances {
			points = append(points, BalancePoint{Time: row.CreatedAt, BalanceUSD: row.BalanceUSD})
		}
		series = append(series, AccountBalanceSeries{
			AccountID:   account.ID,
			AccountName: account.Name,
			Type:        account.Type,
			Points:      points,
		})
	}

	for _, run := range runs {
		summary := DetectionRunSummary{DetectionRun: run}
		if account, ok := accountMap[run.AccountID]; ok {
			summary.AccountName = account.Name
			summary.AccountType = account.Type
		}
		runSummaries = append(runSummaries, summary)
	}

	for _, alert := range alerts {
		summary := AlertEventSummary{AlertEvent: alert}
		if account, ok := accountMap[alert.AccountID]; ok {
			summary.AccountName = account.Name
			summary.AccountType = account.Type
		}
		alertSummaries = append(alertSummaries, summary)
	}

	return &DashboardSummary{
		AccountsTotal:      len(accounts),
		AccountsEnabled:    enabled,
		LowBalanceAccounts: lowBalance,
		FailedAccounts:     failed,
		LatestRuns:         runSummaries,
		RecentAlerts:       alertSummaries,
		AccountBalances:    series,
	}, nil
}

func (s *DetectorService) DailyUsage(rangeValue string) (*DailyUsageSummary, error) {
	now := time.Now().In(time.Local)
	since := dailyUsageRangeStart(rangeValue, now)
	today := startOfLocalDay(now)
	untilDate := today.AddDate(0, 0, 1).Format("2006-01-02")
	sinceDate := ""
	if !since.IsZero() {
		sinceDate = startOfLocalDay(since).Format("2006-01-02")
	}
	directRows, err := s.repo.ListDailyUsage(sinceDate, untilDate)
	if err != nil {
		return nil, err
	}
	deltas, err := s.repo.ListTokenUsageDeltas(since)
	if err != nil {
		return nil, err
	}

	directByAccount := make(map[uint]map[string]float64)
	for _, row := range directRows {
		if _, err := time.Parse("2006-01-02", row.Date); err != nil {
			continue
		}
		if directByAccount[row.AccountID] == nil {
			directByAccount[row.AccountID] = make(map[string]float64)
		}
		directByAccount[row.AccountID][row.Date] = math.Max(0, row.UsedQuotaUSD)
	}

	usageByDate := make(map[string]float64)
	for _, delta := range deltas {
		if delta.UsedQuotaUSD <= 0 {
			continue
		}
		date := delta.CreatedAt.In(time.Local).Format("2006-01-02")
		if accountUsage, ok := directByAccount[delta.AccountID]; ok {
			if _, covered := accountUsage[date]; covered {
				continue
			}
		}
		usageByDate[date] += delta.UsedQuotaUSD
	}
	for _, accountUsage := range directByAccount {
		for date, value := range accountUsage {
			usageByDate[date] += value
		}
	}

	firstDay := since
	if firstDay.IsZero() {
		for _, row := range directRows {
			parsed, parseErr := time.ParseInLocation("2006-01-02", row.Date, time.Local)
			if parseErr == nil && (firstDay.IsZero() || parsed.Before(firstDay)) {
				firstDay = parsed
			}
		}
		for _, delta := range deltas {
			day := startOfLocalDay(delta.CreatedAt.In(time.Local))
			if firstDay.IsZero() || day.Before(firstDay) {
				firstDay = day
			}
		}
	}
	points := []DailyUsagePoint{}
	total := 0.0
	if !firstDay.IsZero() && !firstDay.After(today) {
		for day := startOfLocalDay(firstDay); !day.After(today); day = day.AddDate(0, 0, 1) {
			date := day.Format("2006-01-02")
			value := usageByDate[date]
			points = append(points, DailyUsagePoint{Date: date, UsedQuotaUSD: value})
			total += value
		}
	}

	return &DailyUsageSummary{
		TotalUsedQuotaUSD: total,
		TodayUsedQuotaUSD: usageByDate[today.Format("2006-01-02")],
		Points:            points,
	}, nil
}

func dailyUsageRangeStart(rangeValue string, now time.Time) time.Time {
	days := 0
	switch rangeValue {
	case "3d":
		days = 3
	case "7d":
		days = 7
	case "30d":
		days = 30
	case "90d":
		days = 90
	}
	if days == 0 {
		return time.Time{}
	}
	return startOfLocalDay(now).AddDate(0, 0, -(days - 1))
}

func startOfLocalDay(value time.Time) time.Time {
	local := value.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func (s *DetectorService) ScreenOverview() (*ScreenOverview, error) {
	accounts, err := s.repo.ListAccounts()
	if err != nil {
		return nil, err
	}

	items := make([]ScreenAccountSummary, 0, len(accounts))
	enabledCount := 0
	lowBalanceCount := 0
	usedGroupsCount := 0

	for _, account := range accounts {
		if account.Enabled {
			enabledCount++
		}
		if account.LastBalanceUSD != nil && *account.LastBalanceUSD <= account.LowBalanceThresholdUSD {
			lowBalanceCount++
		}

		item := ScreenAccountSummary{
			AccountID:              account.ID,
			AccountName:            account.Name,
			AccountRemark:          account.Remark,
			Type:                   account.Type,
			BaseURL:                account.BaseURL,
			Enabled:                account.Enabled,
			LastStatus:             account.LastStatus,
			LastError:              account.LastError,
			LastCheckedAt:          account.LastCheckedAt,
			LastBalanceUSD:         account.LastBalanceUSD,
			LowBalanceThresholdUSD: account.LowBalanceThresholdUSD,
			UsedGroups:             []ScreenGroupSummary{},
		}

		run, err := s.repo.GetLatestSuccessfulRun(account.ID)
		if err != nil {
			return nil, err
		}
		if run != nil {
			groups, err := s.repo.GetGroupSnapshotsByRun(run.ID)
			if err != nil {
				return nil, err
			}
			tokens, err := s.repo.GetTokenSnapshotsByRun(run.ID)
			if err != nil {
				return nil, err
			}
			item.UsedGroups = buildScreenGroupSummaries(groups, tokens)
		}

		usedGroupsCount += len(item.UsedGroups)
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Enabled != items[j].Enabled {
			return items[i].Enabled
		}
		if items[i].LastBalanceUSD != nil && items[j].LastBalanceUSD != nil && !sameFloat(*items[i].LastBalanceUSD, *items[j].LastBalanceUSD) {
			return *items[i].LastBalanceUSD > *items[j].LastBalanceUSD
		}
		if items[i].LastCheckedAt != nil && items[j].LastCheckedAt != nil && !items[i].LastCheckedAt.Equal(*items[j].LastCheckedAt) {
			return items[i].LastCheckedAt.After(*items[j].LastCheckedAt)
		}
		return items[i].AccountName < items[j].AccountName
	})

	return &ScreenOverview{
		GeneratedAt:     time.Now(),
		AccountsTotal:   len(accounts),
		AccountsEnabled: enabledCount,
		LowBalanceCount: lowBalanceCount,
		UsedGroupsCount: usedGroupsCount,
		Items:           items,
	}, nil
}

func buildScreenGroupSummaries(groups []models.GroupSnapshot, tokens []models.TokenSnapshot) []ScreenGroupSummary {
	type quotaSummary struct {
		value     float64
		collected bool
		observed  bool
	}

	quotasByGroup := make(map[string]quotaSummary, len(tokens))
	for _, token := range tokens {
		if token.UsedQuotaUSD == nil {
			continue
		}
		quota := quotasByGroup[token.GroupKey]
		quota.value += *token.UsedQuotaUSD
		quota.collected = true
		if *token.UsedQuotaUSD > 0 {
			quota.observed = true
		}
		quotasByGroup[token.GroupKey] = quota
	}
	for _, token := range tokens {
		if token.LastUsedAt == nil {
			continue
		}
		quota := quotasByGroup[token.GroupKey]
		quota.observed = true
		quotasByGroup[token.GroupKey] = quota
	}

	summaries := make([]ScreenGroupSummary, 0, len(groups))
	for _, group := range groups {
		if !group.IsUsedByToken || group.TokenUsageCount <= 0 {
			continue
		}
		summary := ScreenGroupSummary{
			GroupKey:         group.GroupKey,
			GroupName:        group.GroupName,
			RateMultiplier:   group.RateMultiplier,
			TokenUsageCount:  group.TokenUsageCount,
			HasObservedUsage: quotasByGroup[group.GroupKey].observed,
		}
		if quota := quotasByGroup[group.GroupKey]; quota.collected {
			summary.UsedQuotaUSD = utils.PtrFloat64(quota.value)
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].HasObservedUsage != summaries[j].HasObservedUsage {
			return summaries[i].HasObservedUsage
		}
		if summaries[i].TokenUsageCount != summaries[j].TokenUsageCount {
			return summaries[i].TokenUsageCount > summaries[j].TokenUsageCount
		}
		if !sameFloat(summaries[i].RateMultiplier, summaries[j].RateMultiplier) {
			return summaries[i].RateMultiplier > summaries[j].RateMultiplier
		}
		return summaries[i].GroupName < summaries[j].GroupName
	})

	return summaries
}

func (s *DetectorService) RunLogs(accountID uint) ([]models.DetectionRun, error) {
	return s.repo.ListDetectionRuns(accountID, 50)
}

type AccountOverview struct {
	Account      *models.UpstreamAccount `json:"account"`
	LatestRun    *models.DetectionRun    `json:"latest_run"`
	RecentAlerts []models.AlertEvent     `json:"recent_alerts"`
	LatestGroups []GroupDetail           `json:"latest_groups"`
}

type GroupDetail struct {
	GroupKey        string                 `json:"group_key"`
	GroupName       string                 `json:"group_name"`
	Description     string                 `json:"description"`
	RateMultiplier  float64                `json:"rate_multiplier"`
	ImagePrice1K    *float64               `json:"image_price_1k"`
	ImagePrice2K    *float64               `json:"image_price_2k"`
	ImagePrice4K    *float64               `json:"image_price_4k"`
	TokenUsageCount int                    `json:"token_usage_count"`
	IsUsedByToken   bool                   `json:"is_used_by_token"`
	Platform        string                 `json:"platform"`
	CatalogSource   string                 `json:"catalog_source"`
	ModelsCount     int                    `json:"models_count"`
	Models          []upstream.ModelData   `json:"models"`
	Tokens          []models.TokenSnapshot `json:"tokens"`
}

func (s *DetectorService) AccountOverview(accountID uint) (*AccountOverview, error) {
	account, err := s.repo.GetAccount(accountID)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.GetLatestSuccessfulRun(accountID)
	if err != nil {
		return nil, err
	}
	alerts, err := s.repo.ListAlerts(accountID, 20)
	if err != nil {
		return nil, err
	}

	groupDetails := []GroupDetail{}
	if run != nil {
		catalogs, err := s.repo.ListGroupCatalogs(accountID)
		if err != nil {
			return nil, err
		}
		catalogMap := make(map[string]models.GroupCatalog, len(catalogs))
		for _, catalog := range catalogs {
			catalogMap[catalog.GroupKey] = catalog
		}
		groups, err := s.repo.GetGroupSnapshotsByRun(run.ID)
		if err != nil {
			return nil, err
		}
		tokens, err := s.repo.GetTokenSnapshotsByRun(run.ID)
		if err != nil {
			return nil, err
		}
		tokenMap := map[string][]models.TokenSnapshot{}
		for _, token := range tokens {
			tokenMap[token.GroupKey] = append(tokenMap[token.GroupKey], token)
		}
		for _, group := range groups {
			catalog := catalogMap[group.GroupKey]
			groupModels := []upstream.ModelData{}
			if catalog.ModelsJSON != "" {
				if err := json.Unmarshal([]byte(catalog.ModelsJSON), &groupModels); err != nil {
					return nil, fmt.Errorf("decode group model catalog: %w", err)
				}
			}
			groupDetails = append(groupDetails, GroupDetail{
				GroupKey:        group.GroupKey,
				GroupName:       group.GroupName,
				Description:     group.Description,
				RateMultiplier:  group.RateMultiplier,
				ImagePrice1K:    group.ImagePrice1K,
				ImagePrice2K:    group.ImagePrice2K,
				ImagePrice4K:    group.ImagePrice4K,
				TokenUsageCount: group.TokenUsageCount,
				IsUsedByToken:   group.IsUsedByToken,
				Platform:        catalog.Platform,
				CatalogSource:   catalog.Source,
				ModelsCount:     catalog.ModelsCount,
				Models:          groupModels,
				Tokens:          tokenMap[group.GroupKey],
			})
		}
	}

	return &AccountOverview{
		Account:      account,
		LatestRun:    run,
		RecentAlerts: alerts,
		LatestGroups: groupDetails,
	}, nil
}

type BalanceHistoryPoint struct {
	Time       time.Time `json:"time"`
	BalanceUSD float64   `json:"balance_usd"`
}

func (s *DetectorService) BalanceHistory(accountID uint, rangeValue string) ([]BalanceHistoryPoint, error) {
	rows, err := s.repo.ListBalanceSnapshots(accountID, utils.RangeStart(rangeValue))
	if err != nil {
		return nil, err
	}
	points := make([]BalanceHistoryPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, BalanceHistoryPoint{
			Time:       row.CreatedAt,
			BalanceUSD: row.BalanceUSD,
		})
	}
	return points, nil
}

type GroupHistoryPoint struct {
	RunID          uint      `json:"run_id"`
	Time           time.Time `json:"time"`
	GroupKey       string    `json:"group_key"`
	GroupName      string    `json:"group_name"`
	RateMultiplier float64   `json:"rate_multiplier"`
	ImagePrice1K   *float64  `json:"image_price_1k"`
	ImagePrice2K   *float64  `json:"image_price_2k"`
	ImagePrice4K   *float64  `json:"image_price_4k"`
}

func (s *DetectorService) GroupHistory(accountID uint, rangeValue string) ([]GroupHistoryPoint, error) {
	rows, err := s.repo.ListGroupSnapshots(accountID, utils.RangeStart(rangeValue))
	if err != nil {
		return nil, err
	}
	points := make([]GroupHistoryPoint, 0, len(rows))
	for _, row := range rows {
		points = append(points, GroupHistoryPoint{
			RunID:          row.RunID,
			Time:           row.CreatedAt,
			GroupKey:       row.GroupKey,
			GroupName:      row.GroupName,
			RateMultiplier: row.RateMultiplier,
			ImagePrice1K:   row.ImagePrice1K,
			ImagePrice2K:   row.ImagePrice2K,
			ImagePrice4K:   row.ImagePrice4K,
		})
	}
	return points, nil
}

func (s *DetectorService) buildFailureAlerts(account *models.UpstreamAccount, runErr error) []models.AlertEvent {
	if account.LastStatus == string(models.RunStatusFailed) && account.LastError == runErr.Error() {
		return nil
	}

	return []models.AlertEvent{
		{
			AccountID: account.ID,
			Type:      models.AlertTypeAccountError,
			Title:     fmt.Sprintf("%s 检测失败", account.Name),
			Message:   runErr.Error(),
			Severity:  "error",
			CreatedAt: time.Now(),
		},
	}
}

func (s *DetectorService) buildSuccessAlerts(account *models.UpstreamAccount, balance float64, groups []models.GroupSnapshot) ([]models.AlertEvent, error) {
	alerts := make([]models.AlertEvent, 0)

	if account.LastStatus == string(models.RunStatusFailed) {
		alerts = append(alerts, models.AlertEvent{
			AccountID: account.ID,
			Type:      models.AlertTypeAccountRecover,
			Title:     fmt.Sprintf("%s 已恢复", account.Name),
			Message:   "账号检测恢复成功",
			Severity:  "info",
			CreatedAt: time.Now(),
		})
	}

	wasLow := account.LastBalanceUSD != nil && *account.LastBalanceUSD <= account.LowBalanceThresholdUSD
	isLow := balance <= account.LowBalanceThresholdUSD
	if !wasLow && isLow {
		alerts = append(alerts, models.AlertEvent{
			AccountID:    account.ID,
			Type:         models.AlertTypeLowBalance,
			Title:        fmt.Sprintf("%s 余额不足", account.Name),
			Message:      fmt.Sprintf("当前余额 $%.4f，低于阈值 $%.4f", balance, account.LowBalanceThresholdUSD),
			Severity:     "warning",
			PrevValue:    fmt.Sprintf("%.4f", derefFloat(account.LastBalanceUSD)),
			CurrentValue: fmt.Sprintf("%.4f", balance),
			CreatedAt:    time.Now(),
		})
	}
	if wasLow && !isLow {
		alerts = append(alerts, models.AlertEvent{
			AccountID:    account.ID,
			Type:         models.AlertTypeBalanceRecovery,
			Title:        fmt.Sprintf("%s 余额恢复", account.Name),
			Message:      fmt.Sprintf("当前余额 $%.4f，已高于阈值 $%.4f", balance, account.LowBalanceThresholdUSD),
			Severity:     "info",
			PrevValue:    fmt.Sprintf("%.4f", derefFloat(account.LastBalanceUSD)),
			CurrentValue: fmt.Sprintf("%.4f", balance),
			CreatedAt:    time.Now(),
		})
	}

	latestRun, err := s.repo.GetLatestSuccessfulRun(account.ID)
	if err != nil {
		return nil, err
	}
	if latestRun == nil {
		return alerts, nil
	}
	prevGroups, err := s.repo.GetGroupSnapshotsByRun(latestRun.ID)
	if err != nil {
		return nil, err
	}
	prevMap := map[string]models.GroupSnapshot{}
	for _, row := range prevGroups {
		prevMap[row.GroupKey] = row
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].GroupName < groups[j].GroupName
	})
	for _, current := range groups {
		prev, ok := prevMap[current.GroupKey]
		if !ok {
			continue
		}
		if !sameFloat(prev.RateMultiplier, current.RateMultiplier) {
			alerts = append(alerts, models.AlertEvent{
				AccountID:    account.ID,
				Type:         models.AlertTypeGroupChanged,
				Title:        fmt.Sprintf("%s 分组倍率变更", account.Name),
				Message:      fmt.Sprintf("%s 倍率由 %.4f 变为 %.4f", current.GroupName, prev.RateMultiplier, current.RateMultiplier),
				Severity:     "info",
				GroupKey:     current.GroupKey,
				PrevValue:    fmt.Sprintf("%.4f", prev.RateMultiplier),
				CurrentValue: fmt.Sprintf("%.4f", current.RateMultiplier),
				CreatedAt:    time.Now(),
			})
		}
		if !sameFloatPtr(prev.ImagePrice1K, current.ImagePrice1K) || !sameFloatPtr(prev.ImagePrice2K, current.ImagePrice2K) || !sameFloatPtr(prev.ImagePrice4K, current.ImagePrice4K) {
			alerts = append(alerts, models.AlertEvent{
				AccountID:    account.ID,
				Type:         models.AlertTypeGroupChanged,
				Title:        fmt.Sprintf("%s 分组价格变更", account.Name),
				Message:      fmt.Sprintf("%s 图片价格发生变化", current.GroupName),
				Severity:     "info",
				GroupKey:     current.GroupKey,
				PrevValue:    formatPriceSet(prev.ImagePrice1K, prev.ImagePrice2K, prev.ImagePrice4K),
				CurrentValue: formatPriceSet(current.ImagePrice1K, current.ImagePrice2K, current.ImagePrice4K),
				CreatedAt:    time.Now(),
			})
		}
	}

	return alerts, nil
}

func (s *DetectorService) dispatchAlerts(alerts []models.AlertEvent) {
	if len(alerts) == 0 || !s.mailer.Enabled() {
		return
	}
	lines := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		lines = append(lines, fmt.Sprintf("[%s] %s\n%s", alert.Severity, alert.Title, alert.Message))
	}
	subject := fmt.Sprintf("上游监控告警 %d 条", len(alerts))
	if err := s.mailer.Send(subject, strings.Join(lines, "\n\n")); err != nil {
		log.Printf("send alert mail failed: %v", err)
	}
}

func (s *DetectorService) credentialsFor(account *models.UpstreamAccount) (upstream.AccountCredentials, error) {
	bearer, refreshToken, sub2apiEmail, sub2apiPassword, newAPIUser, err := s.accounts.DecryptCredentials(account)
	if err != nil {
		return upstream.AccountCredentials{}, err
	}
	return upstream.AccountCredentials{
		BaseURL:                  account.BaseURL,
		Sub2APIAuthMode:          account.Sub2APIAuthMode,
		Sub2APIEmail:             sub2apiEmail,
		Sub2APIPassword:          sub2apiPassword,
		Sub2APILoginBodyTemplate: account.Sub2APILoginBodyTemplate,
		BearerToken:              bearer,
		RefreshToken:             refreshToken,
		NewAPIUser:               newAPIUser,
		Type:                     account.Type,
	}, nil
}

func (s *DetectorService) fetchSnapshot(ctx context.Context, account *models.UpstreamAccount, creds upstream.AccountCredentials) (*upstream.Snapshot, *upstream.AccountCredentials, error) {
	if account.Type == models.UpstreamTypeSub2API && strings.TrimSpace(creds.BearerToken) == "" {
		var err error
		creds, err = s.renewSub2APICredentials(ctx, account, creds)
		if err != nil {
			return nil, nil, err
		}
	}

	snapshot, err := s.client.Fetch(ctx, creds)
	if err == nil {
		return snapshot, &creds, nil
	}

	if account.Type != models.UpstreamTypeSub2API || !isAuthFailure(err) {
		return nil, nil, err
	}
	renewed, renewErr := s.renewSub2APICredentials(ctx, account, creds)
	if renewErr != nil {
		action := "refresh"
		if normalizeSub2APIAuthMode(account.Type, account.Sub2APIAuthMode) == models.Sub2APIAuthModePassword {
			action = "password login"
		}
		return nil, nil, fmt.Errorf("%w; %s failed: %v", err, action, renewErr)
	}
	creds = renewed
	snapshot, err = s.client.Fetch(ctx, creds)
	if err != nil {
		return nil, nil, err
	}
	return snapshot, &creds, nil
}

func (s *DetectorService) renewSub2APICredentials(ctx context.Context, account *models.UpstreamAccount, creds upstream.AccountCredentials) (upstream.AccountCredentials, error) {
	switch normalizeSub2APIAuthMode(account.Type, account.Sub2APIAuthMode) {
	case models.Sub2APIAuthModePassword:
		result, err := s.client.LoginSub2API(ctx, creds)
		if err != nil {
			return creds, err
		}
		if result.TurnstileRequired {
			provider := result.CaptchaProvider
			if provider == "" {
				provider = "captcha"
			}
			return creds, fmt.Errorf("sub2api 上游开启了人机验证（%s），服务端无法用账号密码自动登录；请改用 refresh token 模式，或在上游关闭人机验证", provider)
		}
		if result.Requires2FA {
			return creds, fmt.Errorf("sub2api login requires 2FA; please switch to refresh token mode")
		}
		if err := s.accounts.UpdateRuntimeCredentials(account.ID, result.AccessToken, result.RefreshToken); err != nil {
			return creds, fmt.Errorf("sub2api login succeeded but failed to persist credentials: %w", err)
		}
		creds.BearerToken = result.AccessToken
		creds.RefreshToken = result.RefreshToken
		return creds, nil
	case models.Sub2APIAuthModeRefreshToken:
		if strings.TrimSpace(creds.RefreshToken) == "" {
			return creds, fmt.Errorf("sub2api refresh token is missing")
		}
		result, err := s.client.RefreshSub2APIToken(ctx, creds)
		if err != nil {
			return creds, err
		}
		if err := s.accounts.UpdateRuntimeCredentials(account.ID, result.AccessToken, result.RefreshToken); err != nil {
			return creds, fmt.Errorf("refresh token succeeded but failed to persist credentials: %w", err)
		}
		creds.BearerToken = result.AccessToken
		creds.RefreshToken = result.RefreshToken
		return creds, nil
	default:
		return creds, fmt.Errorf("invalid sub2api_auth_mode")
	}
}

func isAuthFailure(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "status 401") ||
		strings.Contains(message, "token has expired") ||
		strings.Contains(message, "invalid token") ||
		strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "session_binding") ||
		strings.Contains(message, "session binding") ||
		strings.Contains(message, "token_revoked")
}

func sameFloat(a, b float64) bool {
	return math.Abs(a-b) < 0.000001
}

func sameFloatPtr(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return sameFloat(*a, *b)
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func formatPriceSet(a, b, c *float64) string {
	return fmt.Sprintf("1k=%s,2k=%s,4k=%s", formatFloatPtr(a), formatFloatPtr(b), formatFloatPtr(c))
}

func formatFloatPtr(v *float64) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%.4f", *v)
}
