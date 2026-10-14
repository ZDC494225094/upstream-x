package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"upstream-x/server/models"
)

type AccountCredentials struct {
	BaseURL                  string
	Sub2APIAuthMode          models.Sub2APIAuthMode
	Sub2APIEmail             string
	Sub2APIPassword          string
	Sub2APILoginBodyTemplate string
	BearerToken              string
	RefreshToken             string
	NewAPIUser               string
	Type                     models.UpstreamType
}

type GroupData struct {
	Key            string
	Name           string
	Description    string
	Platform       string
	RateMultiplier float64
	ImagePrice1K   *float64
	ImagePrice2K   *float64
	ImagePrice4K   *float64
	CatalogSource  string
	Models         []ModelData
}

type ModelData struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type DailyUsageData struct {
	Date         string  `json:"date"`
	UsedQuotaUSD float64 `json:"used_quota_usd"`
}

type TokenData struct {
	RemoteID   string
	Name       string
	MaskedKey  string
	Status     string
	GroupKey   string
	GroupName  string
	LastUsedAt *time.Time
	UsedQuota  *float64
}

type Snapshot struct {
	BalanceUSD        float64
	Groups            []GroupData
	Tokens            []TokenData
	DailyUsage        []DailyUsageData
	DailyUsageSource  string
	DailyUsageEnabled bool
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
}

type LoginResult struct {
	AccessToken       string
	RefreshToken      string
	Requires2FA       bool
	TurnstileRequired bool
	// CaptchaProvider names the captcha system blocking password login:
	// "turnstile", "tencent" or "aliyun" (empty when TurnstileRequired is false).
	CaptchaProvider string
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// userAgent is sent on every upstream request. Sub2API (since v0.2.14/0.2.15)
// can bind a session to the login-time IP + User-Agent fingerprint
// (session_binding); a stable UA keeps refresh/login/data calls consistent.
const userAgent = "Upstream-X/1.1"

// upstreamErrorReason extracts the machine-readable reason code from a
// Sub2API-style error body: {"code":401,"message":"...","reason":"..."}.
func upstreamErrorReason(raw []byte) string {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Reason)
}

// describeUpstreamError turns an upstream non-2xx response into an actionable
// message. Since the Sub2API protocol change, refresh tokens are single-use
// rotating tokens: any validation/check consumes the presented token and the
// server only honours the newest one it issued.
func describeUpstreamError(action string, status int, raw []byte) error {
	reason := upstreamErrorReason(raw)
	switch reason {
	case "REFRESH_TOKEN_INVALID":
		return fmt.Errorf("upstream %s status %d: refresh token 已失效（REFRESH_TOKEN_INVALID）。新版 Sub2API 的 refresh token 是一次性轮换的：每次校验/检测都会消耗旧 token 并换发新 token，常见原因：①该 token 已被校验、检测或浏览器会话使用过；②填入的不是以 rt_ 开头的 refresh token（不要填 access token 或 API Key）；③上游账号改过密码导致整个 token 家族被撤销；④token 复制不完整或带了空格/换行。请在上游重新登录获取最新 refresh token 后立即填入，中途不要再用该会话做其他操作；更推荐改用账号密码模式（每次自动重新登录，不受轮换影响）: %s", action, status, string(raw))
	case "REFRESH_TOKEN_EXPIRED":
		return fmt.Errorf("upstream %s status %d: refresh token 已过期（REFRESH_TOKEN_EXPIRED），请重新登录上游获取新 token，或改用账号密码模式: %s", action, status, string(raw))
	case "REFRESH_TOKEN_REUSED":
		return fmt.Errorf("upstream %s status %d: refresh token 被重复使用（REFRESH_TOKEN_REUSED），上游已撤销整个 token 家族，请重新登录上游获取全新 token: %s", action, status, string(raw))
	case "SESSION_BINDING_MISMATCH":
		return fmt.Errorf("upstream %s status %d: 上游开启了会话 IP/UA 绑定（SESSION_BINDING_MISMATCH），该 token 不是在本服务所在服务器/IP 下登录生成的，整个会话家族已被撤销。请改用账号密码模式让本服务自行登录，或在上游关闭会话绑定: %s", action, status, string(raw))
	}
	return fmt.Errorf("upstream %s status %d: %s", action, status, string(raw))
}

func (c *Client) RefreshSub2APIToken(ctx context.Context, creds AccountCredentials) (*RefreshResult, error) {
	if creds.Type != models.UpstreamTypeSub2API {
		return nil, fmt.Errorf("refresh token is only supported for sub2api")
	}
	refreshToken := strings.TrimSpace(creds.RefreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("sub2api refresh token is missing")
	}
	if strings.HasPrefix(refreshToken, "sk-") {
		return nil, fmt.Errorf("填入的内容以 sk- 开头，这是 API Key 不是 refresh token；Sub2API 的 refresh token 以 rt_ 开头，请在上游重新登录后获取，或改用账号密码模式")
	}

	base := strings.TrimRight(creds.BaseURL, "/")
	fullURL := base + "/api/v1/auth/refresh"
	body, err := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal refresh payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request upstream refresh: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, describeUpstreamError("refresh", resp.StatusCode, raw)
	}

	var refreshResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(raw, &refreshResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}

	accessToken := strings.TrimSpace(refreshResp.AccessToken)
	newRefreshToken := strings.TrimSpace(refreshResp.RefreshToken)
	if accessToken == "" {
		accessToken = strings.TrimSpace(refreshResp.Data.AccessToken)
	}
	if newRefreshToken == "" {
		newRefreshToken = strings.TrimSpace(refreshResp.Data.RefreshToken)
	}
	if accessToken == "" {
		return nil, fmt.Errorf("upstream refresh response missing access_token")
	}
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}

	return &RefreshResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (c *Client) LoginSub2API(ctx context.Context, creds AccountCredentials) (*LoginResult, error) {
	if creds.Type != models.UpstreamTypeSub2API {
		return nil, fmt.Errorf("password login is only supported for sub2api")
	}
	if strings.TrimSpace(creds.Sub2APIEmail) == "" || strings.TrimSpace(creds.Sub2APIPassword) == "" {
		return nil, fmt.Errorf("sub2api email or password is missing")
	}

	if provider, blocked := c.Sub2APICaptchaBlocker(ctx, creds.BaseURL); blocked {
		return &LoginResult{TurnstileRequired: true, CaptchaProvider: provider}, nil
	}

	base := strings.TrimRight(creds.BaseURL, "/")
	fullURL := base + "/api/v1/auth/login"
	body, err := RenderSub2APILoginBody(creds.Sub2APILoginBodyTemplate, creds.Sub2APIEmail, creds.Sub2APIPassword)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request upstream login: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read login response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		lowerRaw := strings.ToLower(string(raw))
		switch {
		case strings.Contains(lowerRaw, "turnstile"):
			return &LoginResult{TurnstileRequired: true, CaptchaProvider: "turnstile"}, nil
		case strings.Contains(lowerRaw, "tencent") || strings.Contains(lowerRaw, "captcha"):
			return &LoginResult{TurnstileRequired: true, CaptchaProvider: "captcha"}, nil
		}
		return nil, describeUpstreamError("login", resp.StatusCode, raw)
	}

	var loginResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Requires2FA  bool   `json:"requires_2fa"`
			TempToken    string `json:"temp_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &loginResp); err != nil {
		return nil, fmt.Errorf("decode login response: %w", err)
	}

	if loginResp.Data.Requires2FA {
		return &LoginResult{Requires2FA: true}, nil
	}
	if strings.TrimSpace(loginResp.Data.AccessToken) == "" || strings.TrimSpace(loginResp.Data.RefreshToken) == "" {
		return nil, fmt.Errorf("upstream login response missing access_token or refresh_token")
	}

	return &LoginResult{
		AccessToken:  strings.TrimSpace(loginResp.Data.AccessToken),
		RefreshToken: strings.TrimSpace(loginResp.Data.RefreshToken),
	}, nil
}

func (c *Client) IsSub2APITurnstileEnabled(ctx context.Context, baseURL string) (bool, error) {
	fullURL := strings.TrimRight(baseURL, "/") + "/api/v1/settings/public"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return false, fmt.Errorf("new settings request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request public settings: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read public settings response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("public settings status %d: %s", resp.StatusCode, string(raw))
	}

	var settingsResp struct {
		Code int `json:"code"`
		Data struct {
			TurnstileEnabled      bool `json:"turnstile_enabled"`
			TencentCaptchaEnabled bool `json:"tencent_captcha_enabled"`
			AliyunCaptchaEnabled  bool `json:"aliyun_captcha_enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &settingsResp); err != nil {
		return false, fmt.Errorf("decode public settings response: %w", err)
	}
	return settingsResp.Data.TurnstileEnabled, nil
}

// Sub2APICaptchaBlocker reports whether password login against this Sub2API
// deployment is impossible for a server-side client because a human captcha
// is enabled (Turnstile / Tencent / Aliyun). Newer Sub2API versions support
// three captcha providers; checking only Turnstile used to misdiagnose the
// other two as generic login failures.
func (c *Client) Sub2APICaptchaBlocker(ctx context.Context, baseURL string) (string, bool) {
	fullURL := strings.TrimRight(baseURL, "/") + "/api/v1/settings/public"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}

	var settingsResp struct {
		Data struct {
			TurnstileEnabled      bool `json:"turnstile_enabled"`
			TencentCaptchaEnabled bool `json:"tencent_captcha_enabled"`
			AliyunCaptchaEnabled  bool `json:"aliyun_captcha_enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &settingsResp); err != nil {
		return "", false
	}
	switch {
	case settingsResp.Data.TurnstileEnabled:
		return "turnstile", true
	case settingsResp.Data.TencentCaptchaEnabled:
		return "tencent", true
	case settingsResp.Data.AliyunCaptchaEnabled:
		return "aliyun", true
	}
	return "", false
}

func (c *Client) Fetch(ctx context.Context, creds AccountCredentials) (*Snapshot, error) {
	switch creds.Type {
	case models.UpstreamTypeSub2API:
		snapshot, err := c.fetchSub2API(ctx, creds)
		if err != nil {
			return nil, err
		}
		_ = c.enrichSub2APIKeyUsage(ctx, creds, snapshot)
		return snapshot, nil
	case models.UpstreamTypeNewAPI:
		return c.fetchNewAPI(ctx, creds)
	default:
		return nil, fmt.Errorf("unsupported upstream type: %s", creds.Type)
	}
}

func (c *Client) fetchSub2API(ctx context.Context, creds AccountCredentials) (*Snapshot, error) {
	var me struct {
		Code int `json:"code"`
		Data struct {
			Balance float64 `json:"balance"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getJSON(ctx, creds, "/api/v1/auth/me", nil, &me); err != nil {
		return nil, err
	}

	plazaGroups, plazaAvailable := c.fetchSub2APIModelPlaza(ctx, creds)

	var groupsResp struct {
		Code int `json:"code"`
		Data []struct {
			ID             int      `json:"id"`
			Name           string   `json:"name"`
			Description    string   `json:"description"`
			Platform       string   `json:"platform"`
			RateMultiplier float64  `json:"rate_multiplier"`
			ImagePrice1K   *float64 `json:"image_price_1k"`
			ImagePrice2K   *float64 `json:"image_price_2k"`
			ImagePrice4K   *float64 `json:"image_price_4k"`
		} `json:"data"`
	}
	groupsErr := c.getJSON(ctx, creds, "/api/v1/groups/available", nil, &groupsResp)
	if groupsErr != nil && !plazaAvailable {
		return nil, groupsErr
	}

	groups := make([]GroupData, 0, len(groupsResp.Data)+len(plazaGroups))
	if groupsErr == nil {
		for _, item := range groupsResp.Data {
			groups = append(groups, GroupData{
				Key:            strconv.Itoa(item.ID),
				Name:           item.Name,
				Description:    item.Description,
				Platform:       item.Platform,
				RateMultiplier: item.RateMultiplier,
				ImagePrice1K:   item.ImagePrice1K,
				ImagePrice2K:   item.ImagePrice2K,
				ImagePrice4K:   item.ImagePrice4K,
				CatalogSource:  "groups_api",
			})
		}
	}
	if plazaAvailable {
		groups = mergeSub2APIModelPlaza(groups, plazaGroups)
	}
	dailyUsage, dailyUsageAvailable := c.fetchSub2APIDailyUsage(ctx, creds)

	type keyItem struct {
		ID         int      `json:"id"`
		Key        string   `json:"key"`
		Name       string   `json:"name"`
		GroupID    int      `json:"group_id"`
		Status     string   `json:"status"`
		LastUsedAt *string  `json:"last_used_at"`
		QuotaUsed  *float64 `json:"quota_used"`
		Group      struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"group"`
	}

	items := make([]keyItem, 0)
	if !dailyUsageAvailable {
		page := 1
		for {
			var keysResp struct {
				Code int `json:"code"`
				Data struct {
					Items []keyItem `json:"items"`
					Pages int       `json:"pages"`
				} `json:"data"`
			}
			params := map[string]string{
				"page":      strconv.Itoa(page),
				"page_size": "100",
			}
			if err := c.getJSON(ctx, creds, "/api/v1/keys", params, &keysResp); err != nil {
				// Model Plaza and the daily trend endpoint are independent of API-key
				// enumeration. Newer deployments may intentionally hide /keys.
				if plazaAvailable {
					items = nil
					break
				}
				return nil, err
			}
			items = append(items, keysResp.Data.Items...)
			if page >= keysResp.Data.Pages || len(keysResp.Data.Items) == 0 {
				break
			}
			page++
		}
	}

	tokens := make([]TokenData, 0, len(items))
	for _, item := range items {
		var lastUsed *time.Time
		if item.LastUsedAt != nil && *item.LastUsedAt != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, *item.LastUsedAt); err == nil {
				lastUsed = &parsed
			}
		}
		tokens = append(tokens, TokenData{
			RemoteID:   strconv.Itoa(item.ID),
			Name:       item.Name,
			MaskedKey:  maskToken(item.Key),
			Status:     item.Status,
			GroupKey:   strconv.Itoa(item.GroupID),
			GroupName:  item.Group.Name,
			LastUsedAt: lastUsed,
			UsedQuota:  item.QuotaUsed,
		})
	}

	return &Snapshot{
		BalanceUSD:        me.Data.Balance,
		Groups:            groups,
		Tokens:            tokens,
		DailyUsage:        dailyUsage,
		DailyUsageSource:  "sub2api_trend",
		DailyUsageEnabled: dailyUsageAvailable,
	}, nil
}

func (c *Client) fetchSub2APIModelPlaza(ctx context.Context, creds AccountCredentials) ([]GroupData, bool) {
	type plazaGroup struct {
		ID                 int      `json:"id"`
		Name               string   `json:"name"`
		Description        string   `json:"description"`
		Platform           string   `json:"platform"`
		RateMultiplier     float64  `json:"rate_multiplier"`
		UserRateMultiplier *float64 `json:"user_rate_multiplier"`
		Models             []struct {
			Name     string `json:"name"`
			Platform string `json:"platform"`
		} `json:"models"`
	}
	type plazaPayload struct {
		Groups []plazaGroup `json:"groups"`
	}
	var response struct {
		Code   int          `json:"code"`
		Data   plazaPayload `json:"data"`
		Groups []plazaGroup `json:"groups"`
	}
	if err := c.getJSON(ctx, creds, "/api/v1/model-plaza", nil, &response); err != nil {
		return nil, false
	}
	items := response.Data.Groups
	if len(items) == 0 {
		items = response.Groups
	}
	if response.Code != 0 || len(items) == 0 {
		return nil, false
	}

	groups := make([]GroupData, 0, len(items))
	for _, item := range items {
		models := make([]ModelData, 0, len(item.Models))
		seen := make(map[string]struct{}, len(item.Models))
		for _, model := range item.Models {
			name := strings.TrimSpace(model.Name)
			if name == "" {
				continue
			}
			platform := strings.TrimSpace(model.Platform)
			key := platform + "\x00" + name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			models = append(models, ModelData{Name: name, Platform: platform})
		}
		rate := item.RateMultiplier
		if item.UserRateMultiplier != nil {
			rate = *item.UserRateMultiplier
		}
		groups = append(groups, GroupData{
			Key:            strconv.Itoa(item.ID),
			Name:           item.Name,
			Description:    item.Description,
			Platform:       item.Platform,
			RateMultiplier: rate,
			CatalogSource:  "model_plaza",
			Models:         models,
		})
	}
	return groups, true
}

// fetchSub2APIDailyUsage reads the authoritative daily actual-cost trend when
// available. The returned points always cover the requested window, including
// zero-use days, so callers can distinguish a supported endpoint from a
// legacy deployment that has no trend API.
func (c *Client) fetchSub2APIDailyUsage(ctx context.Context, creds AccountCredentials) ([]DailyUsageData, bool) {
	type trendPoint struct {
		Date            string   `json:"date"`
		ActualCost      *float64 `json:"actual_cost"`
		TotalActualCost *float64 `json:"total_actual_cost"`
		Cost            *float64 `json:"cost"`
	}
	var response struct {
		Code int `json:"code"`
		Data struct {
			Trend []trendPoint `json:"trend"`
		} `json:"data"`
		Trend []trendPoint `json:"trend"`
	}

	today := startOfLocalDay(time.Now())
	start := today.AddDate(0, 0, -89)
	params := map[string]string{
		"start_date":  start.Format("2006-01-02"),
		"end_date":    today.Format("2006-01-02"),
		"granularity": "day",
	}
	if location := time.Local.String(); location != "" && location != "Local" {
		params["timezone"] = location
	}
	if err := c.getJSON(ctx, creds, "/api/v1/usage/dashboard/trend", params, &response); err != nil {
		return nil, false
	}
	if response.Code != 0 {
		return nil, false
	}
	trend := response.Data.Trend
	if len(trend) == 0 {
		trend = response.Trend
	}
	byDate := make(map[string]float64, len(trend))
	for _, point := range trend {
		date := strings.TrimSpace(point.Date)
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		value := 0.0
		switch {
		case point.ActualCost != nil:
			value = *point.ActualCost
		case point.TotalActualCost != nil:
			value = *point.TotalActualCost
		case point.Cost != nil:
			value = *point.Cost
		}
		if value < 0 {
			value = 0
		}
		byDate[date] = value
	}

	points := make([]DailyUsageData, 0, 90)
	for day := start; !day.After(today); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		points = append(points, DailyUsageData{Date: date, UsedQuotaUSD: byDate[date]})
	}
	return points, true
}

func startOfLocalDay(value time.Time) time.Time {
	local := value.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func mergeSub2APIModelPlaza(groups []GroupData, plazaGroups []GroupData) []GroupData {
	byKey := make(map[string]int, len(groups))
	for index := range groups {
		byKey[groups[index].Key] = index
	}
	for _, plazaGroup := range plazaGroups {
		if index, ok := byKey[plazaGroup.Key]; ok {
			groups[index].CatalogSource = plazaGroup.CatalogSource
			groups[index].Models = plazaGroup.Models
			if groups[index].Description == "" {
				groups[index].Description = plazaGroup.Description
			}
			if groups[index].Platform == "" {
				groups[index].Platform = plazaGroup.Platform
			}
			continue
		}
		byKey[plazaGroup.Key] = len(groups)
		groups = append(groups, plazaGroup)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].RateMultiplier != groups[j].RateMultiplier {
			return groups[i].RateMultiplier < groups[j].RateMultiplier
		}
		return groups[i].Name < groups[j].Name
	})
	return groups
}

const sub2APIUsageBatchSize = 100

func (c *Client) enrichSub2APIKeyUsage(ctx context.Context, creds AccountCredentials, snapshot *Snapshot) error {
	if snapshot == nil || len(snapshot.Tokens) == 0 {
		return nil
	}

	ids := make([]int, 0, len(snapshot.Tokens))
	for _, token := range snapshot.Tokens {
		id, err := strconv.Atoi(token.RemoteID)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	usageByID := make(map[int]float64, len(ids))
	for start := 0; start < len(ids); start += sub2APIUsageBatchSize {
		end := start + sub2APIUsageBatchSize
		if end > len(ids) {
			end = len(ids)
		}

		var response struct {
			Code int `json:"code"`
			Data struct {
				Stats map[string]struct {
					APIKeyID        int     `json:"api_key_id"`
					TotalActualCost float64 `json:"total_actual_cost"`
				} `json:"stats"`
			} `json:"data"`
		}
		if err := c.postJSON(ctx, creds, "/api/v1/usage/dashboard/api-keys-usage", map[string]any{
			"api_key_ids": ids[start:end],
		}, &response); err != nil {
			return err
		}
		if response.Code != 0 {
			return fmt.Errorf("sub2api key usage endpoint returned code %d", response.Code)
		}
		for mapID, stat := range response.Data.Stats {
			id := stat.APIKeyID
			if id <= 0 {
				id, _ = strconv.Atoi(mapID)
			}
			if id > 0 {
				usageByID[id] = stat.TotalActualCost
			}
		}
	}

	for index := range snapshot.Tokens {
		id, err := strconv.Atoi(snapshot.Tokens[index].RemoteID)
		if err != nil {
			continue
		}
		if usage, ok := usageByID[id]; ok {
			snapshot.Tokens[index].UsedQuota = &usage
		}
	}
	return nil
}

func (c *Client) fetchNewAPI(ctx context.Context, creds AccountCredentials) (*Snapshot, error) {
	var me struct {
		Success bool `json:"success"`
		Data    struct {
			Quota float64 `json:"quota"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getJSON(ctx, creds, "/api/user/self", nil, &me); err != nil {
		return nil, err
	}

	var groupsResp struct {
		Success bool `json:"success"`
		Data    map[string]struct {
			Desc  string  `json:"desc"`
			Ratio float64 `json:"ratio"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := c.getJSON(ctx, creds, "/api/user/self/groups", nil, &groupsResp); err != nil {
		return nil, err
	}

	groups := make([]GroupData, 0, len(groupsResp.Data))
	for key, item := range groupsResp.Data {
		groups = append(groups, GroupData{
			Key:            key,
			Name:           key,
			Description:    item.Desc,
			RateMultiplier: item.Ratio,
			CatalogSource:  "groups_api",
		})
	}

	type tokenItem struct {
		ID           int     `json:"id"`
		Key          string  `json:"key"`
		Status       int     `json:"status"`
		Name         string  `json:"name"`
		AccessedTime int64   `json:"accessed_time"`
		UsedQuota    float64 `json:"used_quota"`
		Group        string  `json:"group"`
	}

	items := make([]tokenItem, 0)
	page := 1
	for {
		var tokenResp struct {
			Success bool `json:"success"`
			Data    struct {
				Page     int         `json:"page"`
				PageSize int         `json:"page_size"`
				Total    int         `json:"total"`
				Items    []tokenItem `json:"items"`
			} `json:"data"`
		}
		params := map[string]string{
			"p":    strconv.Itoa(page),
			"size": "100",
		}
		if err := c.getJSON(ctx, creds, "/api/token/", params, &tokenResp); err != nil {
			return nil, err
		}
		items = append(items, tokenResp.Data.Items...)
		if len(items) >= tokenResp.Data.Total || len(tokenResp.Data.Items) == 0 {
			break
		}
		page++
	}

	tokens := make([]TokenData, 0, len(items))
	for _, item := range items {
		var lastUsed *time.Time
		if item.AccessedTime > 0 {
			t := time.Unix(item.AccessedTime, 0)
			lastUsed = &t
		}
		usedQuotaUSD := item.UsedQuota / 500000
		tokens = append(tokens, TokenData{
			RemoteID:   strconv.Itoa(item.ID),
			Name:       item.Name,
			MaskedKey:  maskToken(item.Key),
			Status:     strconv.Itoa(item.Status),
			GroupKey:   item.Group,
			GroupName:  item.Group,
			LastUsedAt: lastUsed,
			UsedQuota:  &usedQuotaUSD,
		})
	}

	return &Snapshot{
		BalanceUSD: me.Data.Quota / 500000,
		Groups:     groups,
		Tokens:     tokens,
	}, nil
}

func (c *Client) getJSON(ctx context.Context, creds AccountCredentials, path string, query map[string]string, target any) error {
	base := strings.TrimRight(creds.BaseURL, "/")
	fullURL, err := url.Parse(base + path)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	values := fullURL.Query()
	for key, value := range query {
		values.Set(key, value)
	}
	fullURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL.String(), nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(creds.BearerToken))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if creds.Type == models.UpstreamTypeNewAPI && creds.NewAPIUser != "" {
		req.Header.Set("New-Api-User", creds.NewAPIUser)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request upstream: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, creds AccountCredentials, path string, payload any, target any) error {
	base := strings.TrimRight(creds.BaseURL, "/")
	fullURL, err := url.Parse(base + path)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(creds.BearerToken))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if creds.Type == models.UpstreamTypeNewAPI && creds.NewAPIUser != "" {
		req.Header.Set("New-Api-User", creds.NewAPIUser)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request upstream: %w", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(body))
	}
	if target == nil || len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func maskToken(value string) string {
	if len(value) <= 8 {
		return value
	}
	if strings.Contains(value, "*") {
		return value
	}
	return value[:4] + "********" + value[len(value)-4:]
}
