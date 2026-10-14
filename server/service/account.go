package service

import (
	"errors"
	"strings"
	"time"

	"upstream-x/server/crypto"
	"upstream-x/server/models"
	"upstream-x/server/repository"
	"upstream-x/server/upstream"

	"gorm.io/gorm"
)

type AccountPayload struct {
	Name                     string                 `json:"name"`
	Remark                   string                 `json:"remark"`
	Type                     models.UpstreamType    `json:"type"`
	BaseURL                  string                 `json:"base_url"`
	Sub2APIAuthMode          models.Sub2APIAuthMode `json:"sub2api_auth_mode"`
	Sub2APILoginBodyTemplate string                 `json:"sub2api_login_body_template"`
	NewAPIAuthMode           models.NewAPIAuthMode  `json:"newapi_auth_mode"`
	BearerToken              string                 `json:"bearer_token"`
	RefreshToken             string                 `json:"refresh_token"`
	Sub2APIEmail             string                 `json:"sub2api_email"`
	Sub2APIPassword          string                 `json:"sub2api_password"`
	NewAPIUser               string                 `json:"new_api_user"`
	LowBalanceThresholdUSD   float64                `json:"low_balance_threshold_usd"`
	Enabled                  bool                   `json:"enabled"`
}

type AccountTransferItem struct {
	ID                       uint                   `json:"id,omitempty"`
	Name                     string                 `json:"name"`
	Remark                   string                 `json:"remark,omitempty"`
	Type                     models.UpstreamType    `json:"type"`
	BaseURL                  string                 `json:"base_url"`
	Sub2APIAuthMode          models.Sub2APIAuthMode `json:"sub2api_auth_mode,omitempty"`
	Sub2APILoginBodyTemplate string                 `json:"sub2api_login_body_template,omitempty"`
	NewAPIAuthMode           models.NewAPIAuthMode  `json:"newapi_auth_mode,omitempty"`
	BearerToken              string                 `json:"bearer_token"`
	RefreshToken             string                 `json:"refresh_token,omitempty"`
	Sub2APIEmail             string                 `json:"sub2api_email,omitempty"`
	Sub2APIPassword          string                 `json:"sub2api_password,omitempty"`
	NewAPIUser               string                 `json:"new_api_user"`
	LowBalanceThresholdUSD   float64                `json:"low_balance_threshold_usd"`
	Enabled                  bool                   `json:"enabled"`
}

type AccountTransferBundle struct {
	Version    string                `json:"version"`
	ExportedAt time.Time             `json:"exported_at"`
	Accounts   []AccountTransferItem `json:"accounts"`
}

type AccountImportError struct {
	Index   int    `json:"index"`
	ID      uint   `json:"id,omitempty"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

type AccountImportResult struct {
	Created int                  `json:"created"`
	Updated int                  `json:"updated"`
	Failed  int                  `json:"failed"`
	Errors  []AccountImportError `json:"errors"`
}

type AccountView struct {
	ID                       uint                   `json:"id"`
	CreatedAt                time.Time              `json:"created_at"`
	UpdatedAt                time.Time              `json:"updated_at"`
	Name                     string                 `json:"name"`
	Remark                   string                 `json:"remark"`
	Type                     models.UpstreamType    `json:"type"`
	BaseURL                  string                 `json:"base_url"`
	Sub2APIAuthMode          models.Sub2APIAuthMode `json:"sub2api_auth_mode,omitempty"`
	Sub2APILoginBodyTemplate string                 `json:"sub2api_login_body_template,omitempty"`
	NewAPIAuthMode           models.NewAPIAuthMode  `json:"newapi_auth_mode,omitempty"`
	Sub2APIEmail             string                 `json:"sub2api_email,omitempty"`
	NewAPIUser               string                 `json:"new_api_user,omitempty"`
	LowBalanceThresholdUSD   float64                `json:"low_balance_threshold_usd"`
	Enabled                  bool                   `json:"enabled"`
	LastStatus               string                 `json:"last_status"`
	LastError                string                 `json:"last_error"`
	LastCheckedAt            *time.Time             `json:"last_checked_at"`
	LastBalanceUSD           *float64               `json:"last_balance_usd"`
}

type AccountService struct {
	repo   *repository.Repository
	cipher *crypto.Cipher
}

func NewAccountService(repo *repository.Repository, cipher *crypto.Cipher) *AccountService {
	return &AccountService{repo: repo, cipher: cipher}
}

func (s *AccountService) List() ([]models.UpstreamAccount, error) {
	return s.repo.ListAccounts()
}

func (s *AccountService) ListViews() ([]AccountView, error) {
	accounts, err := s.repo.ListAccounts()
	if err != nil {
		return nil, err
	}

	views := make([]AccountView, 0, len(accounts))
	for idx := range accounts {
		view, err := s.ToView(&accounts[idx])
		if err != nil {
			return nil, err
		}
		views = append(views, *view)
	}
	return views, nil
}

func (s *AccountService) Get(id uint) (*models.UpstreamAccount, error) {
	return s.repo.GetAccount(id)
}

func (s *AccountService) ToView(account *models.UpstreamAccount) (*AccountView, error) {
	sub2apiEmail := ""
	if account.Type == models.UpstreamTypeSub2API && account.Sub2APIAuthMode == models.Sub2APIAuthModePassword && account.Sub2APIEmailEncrypted != "" {
		email, err := s.cipher.Decrypt(account.Sub2APIEmailEncrypted)
		if err != nil {
			return nil, err
		}
		sub2apiEmail = email
	}

	newAPIUser := ""
	if account.Type == models.UpstreamTypeNewAPI && account.NewAPIUserEncrypted != "" {
		user, err := s.cipher.Decrypt(account.NewAPIUserEncrypted)
		if err != nil {
			return nil, err
		}
		newAPIUser = user
	}

	return &AccountView{
		ID:                       account.ID,
		CreatedAt:                account.CreatedAt,
		UpdatedAt:                account.UpdatedAt,
		Name:                     account.Name,
		Remark:                   account.Remark,
		Type:                     account.Type,
		BaseURL:                  account.BaseURL,
		Sub2APIAuthMode:          normalizeSub2APIAuthMode(account.Type, account.Sub2APIAuthMode),
		Sub2APILoginBodyTemplate: account.Sub2APILoginBodyTemplate,
		NewAPIAuthMode:           normalizeNewAPIAuthMode(account.Type, account.NewAPIAuthMode),
		Sub2APIEmail:             sub2apiEmail,
		NewAPIUser:               newAPIUser,
		LowBalanceThresholdUSD:   account.LowBalanceThresholdUSD,
		Enabled:                  account.Enabled,
		LastStatus:               account.LastStatus,
		LastError:                account.LastError,
		LastCheckedAt:            account.LastCheckedAt,
		LastBalanceUSD:           account.LastBalanceUSD,
	}, nil
}

func (s *AccountService) Create(payload AccountPayload) (*models.UpstreamAccount, error) {
	account, err := s.BuildTransient(payload)
	if err != nil {
		return nil, err
	}

	if err := s.repo.CreateAccount(account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *AccountService) BuildTransient(payload AccountPayload) (*models.UpstreamAccount, error) {
	if err := validateAccountPayload(payload, validationOptions{RequireCredentials: true}); err != nil {
		return nil, err
	}

	bearer, err := s.cipher.Encrypt(payload.BearerToken)
	if err != nil {
		return nil, err
	}
	var refreshToken string
	if strings.TrimSpace(payload.RefreshToken) != "" {
		refreshToken, err = s.cipher.Encrypt(upstream.NormalizeRefreshToken(payload.RefreshToken))
		if err != nil {
			return nil, err
		}
	}
	var sub2apiEmail string
	if strings.TrimSpace(payload.Sub2APIEmail) != "" {
		sub2apiEmail, err = s.cipher.Encrypt(payload.Sub2APIEmail)
		if err != nil {
			return nil, err
		}
	}
	var sub2apiPassword string
	if strings.TrimSpace(payload.Sub2APIPassword) != "" {
		sub2apiPassword, err = s.cipher.Encrypt(payload.Sub2APIPassword)
		if err != nil {
			return nil, err
		}
	}
	var newAPIUser string
	if payload.NewAPIUser != "" {
		newAPIUser, err = s.cipher.Encrypt(payload.NewAPIUser)
		if err != nil {
			return nil, err
		}
	}

	account := &models.UpstreamAccount{
		Name:                     strings.TrimSpace(payload.Name),
		Remark:                   strings.TrimSpace(payload.Remark),
		Type:                     payload.Type,
		BaseURL:                  strings.TrimRight(strings.TrimSpace(payload.BaseURL), "/"),
		Sub2APIAuthMode:          normalizeSub2APIAuthMode(payload.Type, payload.Sub2APIAuthMode),
		Sub2APILoginBodyTemplate: strings.TrimSpace(payload.Sub2APILoginBodyTemplate),
		NewAPIAuthMode:           normalizeNewAPIAuthMode(payload.Type, payload.NewAPIAuthMode),
		BearerTokenEncrypted:     bearer,
		RefreshTokenEncrypted:    refreshToken,
		Sub2APIEmailEncrypted:    sub2apiEmail,
		Sub2APIPasswordEncrypted: sub2apiPassword,
		NewAPIUserEncrypted:      newAPIUser,
		LowBalanceThresholdUSD:   payload.LowBalanceThresholdUSD,
		Enabled:                  payload.Enabled,
	}

	return account, nil
}

func (s *AccountService) Update(id uint, payload AccountPayload) (*models.UpstreamAccount, error) {
	account, err := s.repo.GetAccount(id)
	if err != nil {
		return nil, err
	}

	previousType := account.Type
	merged, err := s.BuildValidationPayload(account, payload)
	if err != nil {
		return nil, err
	}

	account.Name = strings.TrimSpace(merged.Name)
	account.Remark = strings.TrimSpace(merged.Remark)
	account.Type = merged.Type
	account.BaseURL = strings.TrimRight(strings.TrimSpace(merged.BaseURL), "/")
	account.Sub2APIAuthMode = normalizeSub2APIAuthMode(merged.Type, merged.Sub2APIAuthMode)
	account.Sub2APILoginBodyTemplate = strings.TrimSpace(merged.Sub2APILoginBodyTemplate)
	account.NewAPIAuthMode = normalizeNewAPIAuthMode(merged.Type, merged.NewAPIAuthMode)
	account.LowBalanceThresholdUSD = merged.LowBalanceThresholdUSD
	account.Enabled = merged.Enabled

	if strings.TrimSpace(payload.BearerToken) != "" || previousType != merged.Type {
		encrypted, err := s.cipher.Encrypt(merged.BearerToken)
		if err != nil {
			return nil, err
		}
		account.BearerTokenEncrypted = encrypted
	}
	if merged.Type == models.UpstreamTypeSub2API {
		if strings.TrimSpace(payload.RefreshToken) != "" || previousType != merged.Type {
			encrypted, err := s.cipher.Encrypt(upstream.NormalizeRefreshToken(merged.RefreshToken))
			if err != nil {
				return nil, err
			}
			account.RefreshTokenEncrypted = encrypted
		}
		if strings.TrimSpace(payload.Sub2APIEmail) != "" || previousType != merged.Type {
			encrypted, err := s.cipher.Encrypt(merged.Sub2APIEmail)
			if err != nil {
				return nil, err
			}
			account.Sub2APIEmailEncrypted = encrypted
		}
		if strings.TrimSpace(payload.Sub2APIPassword) != "" || previousType != merged.Type {
			encrypted, err := s.cipher.Encrypt(merged.Sub2APIPassword)
			if err != nil {
				return nil, err
			}
			account.Sub2APIPasswordEncrypted = encrypted
		}
	} else {
		account.RefreshTokenEncrypted = ""
		account.Sub2APIEmailEncrypted = ""
		account.Sub2APIPasswordEncrypted = ""
	}
	if merged.Type == models.UpstreamTypeNewAPI {
		if strings.TrimSpace(payload.NewAPIUser) != "" || previousType != merged.Type {
			encrypted, err := s.cipher.Encrypt(merged.NewAPIUser)
			if err != nil {
				return nil, err
			}
			account.NewAPIUserEncrypted = encrypted
		}
	} else {
		account.NewAPIUserEncrypted = ""
	}

	if err := s.repo.UpdateAccount(account); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *AccountService) BuildValidationPayload(account *models.UpstreamAccount, payload AccountPayload) (AccountPayload, error) {
	bearer, refreshToken, sub2apiEmail, sub2apiPassword, newAPIUser, err := s.DecryptCredentials(account)
	if err != nil {
		return AccountPayload{}, err
	}

	merged := AccountPayload{
		Name:                     payload.Name,
		Remark:                   payload.Remark,
		Type:                     payload.Type,
		BaseURL:                  payload.BaseURL,
		Sub2APIAuthMode:          payload.Sub2APIAuthMode,
		Sub2APILoginBodyTemplate: payload.Sub2APILoginBodyTemplate,
		NewAPIAuthMode:           payload.NewAPIAuthMode,
		BearerToken:              payload.BearerToken,
		RefreshToken:             payload.RefreshToken,
		Sub2APIEmail:             payload.Sub2APIEmail,
		Sub2APIPassword:          payload.Sub2APIPassword,
		NewAPIUser:               payload.NewAPIUser,
		LowBalanceThresholdUSD:   payload.LowBalanceThresholdUSD,
		Enabled:                  payload.Enabled,
	}

	if strings.TrimSpace(merged.Name) == "" {
		merged.Name = account.Name
	}
	if merged.Type == "" {
		merged.Type = account.Type
	}
	if merged.Sub2APIAuthMode == "" {
		merged.Sub2APIAuthMode = account.Sub2APIAuthMode
	}
	if merged.NewAPIAuthMode == "" {
		merged.NewAPIAuthMode = account.NewAPIAuthMode
	}
	if merged.Type == account.Type && merged.Type == models.UpstreamTypeSub2API && strings.TrimSpace(merged.Sub2APILoginBodyTemplate) == "" {
		merged.Sub2APILoginBodyTemplate = account.Sub2APILoginBodyTemplate
	}
	if strings.TrimSpace(merged.BaseURL) == "" {
		merged.BaseURL = account.BaseURL
	}
	if merged.Type == account.Type && strings.TrimSpace(merged.BearerToken) == "" {
		merged.BearerToken = bearer
	}
	if merged.Type == account.Type && merged.Type == models.UpstreamTypeSub2API && strings.TrimSpace(merged.RefreshToken) == "" {
		merged.RefreshToken = refreshToken
	}
	if merged.Type == account.Type && merged.Type == models.UpstreamTypeSub2API && strings.TrimSpace(merged.Sub2APIEmail) == "" {
		merged.Sub2APIEmail = sub2apiEmail
	}
	if merged.Type == account.Type && merged.Type == models.UpstreamTypeSub2API && strings.TrimSpace(merged.Sub2APIPassword) == "" {
		merged.Sub2APIPassword = sub2apiPassword
	}
	if merged.Type == account.Type && merged.Type == models.UpstreamTypeNewAPI && strings.TrimSpace(merged.NewAPIUser) == "" {
		merged.NewAPIUser = newAPIUser
	}
	if merged.LowBalanceThresholdUSD == 0 && account.LowBalanceThresholdUSD != 0 {
		merged.LowBalanceThresholdUSD = account.LowBalanceThresholdUSD
	}

	return merged, validateAccountPayload(merged, validationOptions{RequireCredentials: true})
}

func (s *AccountService) Delete(id uint) error {
	return s.repo.DeleteAccount(id)
}

func (s *AccountService) ExportBundle() (*AccountTransferBundle, error) {
	accounts, err := s.repo.ListAccounts()
	if err != nil {
		return nil, err
	}

	items := make([]AccountTransferItem, 0, len(accounts))
	for _, account := range accounts {
		bearerToken, refreshToken, sub2apiEmail, sub2apiPassword, newAPIUser, err := s.DecryptCredentials(&account)
		if err != nil {
			return nil, err
		}
		if account.Type == models.UpstreamTypeSub2API {
			bearerToken = ""
		}

		items = append(items, AccountTransferItem{
			ID:                       account.ID,
			Name:                     account.Name,
			Remark:                   account.Remark,
			Type:                     account.Type,
			BaseURL:                  account.BaseURL,
			Sub2APIAuthMode:          account.Sub2APIAuthMode,
			Sub2APILoginBodyTemplate: account.Sub2APILoginBodyTemplate,
			NewAPIAuthMode:           normalizeNewAPIAuthMode(account.Type, account.NewAPIAuthMode),
			BearerToken:              bearerToken,
			RefreshToken:             refreshToken,
			Sub2APIEmail:             sub2apiEmail,
			Sub2APIPassword:          sub2apiPassword,
			NewAPIUser:               newAPIUser,
			LowBalanceThresholdUSD:   account.LowBalanceThresholdUSD,
			Enabled:                  account.Enabled,
		})
	}

	return &AccountTransferBundle{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Accounts:   items,
	}, nil
}

func (s *AccountService) ImportBundle(bundle AccountTransferBundle) (*AccountImportResult, error) {
	if len(bundle.Accounts) == 0 {
		return nil, errors.New("accounts is required")
	}

	result := &AccountImportResult{
		Errors: make([]AccountImportError, 0),
	}

	for idx, item := range bundle.Accounts {
		payload := AccountPayload{
			Name:                     item.Name,
			Remark:                   item.Remark,
			Type:                     item.Type,
			BaseURL:                  item.BaseURL,
			Sub2APIAuthMode:          item.Sub2APIAuthMode,
			Sub2APILoginBodyTemplate: item.Sub2APILoginBodyTemplate,
			NewAPIAuthMode:           item.NewAPIAuthMode,
			BearerToken:              item.BearerToken,
			RefreshToken:             item.RefreshToken,
			Sub2APIEmail:             item.Sub2APIEmail,
			Sub2APIPassword:          item.Sub2APIPassword,
			NewAPIUser:               item.NewAPIUser,
			LowBalanceThresholdUSD:   item.LowBalanceThresholdUSD,
			Enabled:                  item.Enabled,
		}

		existing, err := s.findImportTarget(item)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, AccountImportError{
				Index:   idx,
				ID:      item.ID,
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}

		if existing != nil {
			bearerToken, refreshToken, sub2apiEmail, sub2apiPassword, newAPIUser, err := s.DecryptCredentials(existing)
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, AccountImportError{
					Index:   idx,
					ID:      item.ID,
					Name:    item.Name,
					Message: err.Error(),
				})
				continue
			}
			if strings.TrimSpace(payload.BearerToken) == "" {
				payload.BearerToken = bearerToken
			}
			if payload.Type == models.UpstreamTypeSub2API && strings.TrimSpace(payload.RefreshToken) == "" {
				payload.RefreshToken = refreshToken
			}
			if payload.Type == models.UpstreamTypeSub2API && strings.TrimSpace(payload.Sub2APIEmail) == "" {
				payload.Sub2APIEmail = sub2apiEmail
			}
			if payload.Type == models.UpstreamTypeSub2API && strings.TrimSpace(payload.Sub2APIPassword) == "" {
				payload.Sub2APIPassword = sub2apiPassword
			}
			if payload.Type == models.UpstreamTypeSub2API && strings.TrimSpace(payload.Sub2APILoginBodyTemplate) == "" {
				payload.Sub2APILoginBodyTemplate = existing.Sub2APILoginBodyTemplate
			}
			if payload.Type == models.UpstreamTypeNewAPI && strings.TrimSpace(payload.NewAPIUser) == "" {
				payload.NewAPIUser = newAPIUser
			}
		}

		if existing != nil {
			if _, err := s.Update(existing.ID, payload); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, AccountImportError{
					Index:   idx,
					ID:      item.ID,
					Name:    item.Name,
					Message: err.Error(),
				})
				continue
			}
			result.Updated++
			continue
		}

		if _, err := s.Create(payload); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, AccountImportError{
				Index:   idx,
				ID:      item.ID,
				Name:    item.Name,
				Message: err.Error(),
			})
			continue
		}
		result.Created++
	}

	return result, nil
}

func (s *AccountService) DecryptCredentials(account *models.UpstreamAccount) (string, string, string, string, string, error) {
	bearer, err := s.cipher.Decrypt(account.BearerTokenEncrypted)
	if err != nil {
		return "", "", "", "", "", err
	}
	refreshToken := ""
	if account.RefreshTokenEncrypted != "" {
		refreshToken, err = s.cipher.Decrypt(account.RefreshTokenEncrypted)
		if err != nil {
			return "", "", "", "", "", err
		}
	}
	sub2apiEmail := ""
	if account.Sub2APIEmailEncrypted != "" {
		sub2apiEmail, err = s.cipher.Decrypt(account.Sub2APIEmailEncrypted)
		if err != nil {
			return "", "", "", "", "", err
		}
	}
	sub2apiPassword := ""
	if account.Sub2APIPasswordEncrypted != "" {
		sub2apiPassword, err = s.cipher.Decrypt(account.Sub2APIPasswordEncrypted)
		if err != nil {
			return "", "", "", "", "", err
		}
	}
	newAPIUser := ""
	if account.NewAPIUserEncrypted != "" {
		newAPIUser, err = s.cipher.Decrypt(account.NewAPIUserEncrypted)
		if err != nil {
			return "", "", "", "", "", err
		}
	}
	return bearer, refreshToken, sub2apiEmail, sub2apiPassword, newAPIUser, nil
}

type validationOptions struct {
	RequireCredentials bool
}

func validateAccountPayload(payload AccountPayload, opts validationOptions) error {
	if strings.TrimSpace(payload.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(payload.BaseURL) == "" {
		return errors.New("base_url is required")
	}
	if payload.Type != models.UpstreamTypeSub2API && payload.Type != models.UpstreamTypeNewAPI {
		return errors.New("type must be sub2api or newapi")
	}
	if payload.Type == models.UpstreamTypeSub2API {
		authMode := payload.Sub2APIAuthMode
		if authMode == "" {
			authMode = models.Sub2APIAuthModeRefreshToken
		}
		if authMode == models.Sub2APIAuthModeRefreshToken {
			if strings.TrimSpace(payload.RefreshToken) == "" {
				return errors.New("refresh_token is required for sub2api refresh_token mode")
			}
		} else if authMode == models.Sub2APIAuthModePassword {
			if strings.TrimSpace(payload.Sub2APIEmail) == "" {
				return errors.New("sub2api_email is required for sub2api password mode")
			}
			if strings.TrimSpace(payload.Sub2APIPassword) == "" {
				return errors.New("sub2api_password is required for sub2api password mode")
			}
			if err := upstream.ValidateSub2APILoginBodyTemplate(payload.Sub2APILoginBodyTemplate); err != nil {
				return err
			}
		} else {
			return errors.New("invalid sub2api_auth_mode")
		}
	} else if payload.Type == models.UpstreamTypeNewAPI {
		authMode := payload.NewAPIAuthMode
		if authMode == "" {
			authMode = models.NewAPIAuthModeManual
		}
		if authMode == models.NewAPIAuthModeManual {
			if opts.RequireCredentials && strings.TrimSpace(payload.BearerToken) == "" {
				return errors.New("bearer_token is required")
			}
			if opts.RequireCredentials && strings.TrimSpace(payload.NewAPIUser) == "" {
				return errors.New("new_api_user is required for newapi")
			}
		} else {
			return errors.New("invalid newapi_auth_mode")
		}
	}
	if payload.LowBalanceThresholdUSD < 0 {
		return errors.New("low_balance_threshold_usd must be >= 0")
	}
	return nil
}

func (s *AccountService) findImportTarget(item AccountTransferItem) (*models.UpstreamAccount, error) {
	name := strings.TrimSpace(item.Name)
	if name != "" && item.Type != "" {
		account, err := s.repo.FindAccountByNameAndType(name, item.Type)
		if err == nil {
			return account, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	if item.ID == 0 {
		return nil, nil
	}

	account, err := s.repo.GetAccount(item.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if name == "" || item.Type == "" {
		return account, nil
	}
	if strings.TrimSpace(account.Name) == name && account.Type == item.Type {
		return account, nil
	}

	return nil, nil
}

func (s *AccountService) UpdateRuntimeTokens(accountID uint, accessToken string, refreshToken string) error {
	if accountID == 0 {
		return nil
	}
	account, err := s.repo.GetAccount(accountID)
	if err != nil {
		return err
	}

	if strings.TrimSpace(accessToken) != "" {
		encryptedAccessToken, err := s.cipher.Encrypt(accessToken)
		if err != nil {
			return err
		}
		account.BearerTokenEncrypted = encryptedAccessToken
	}

	if strings.TrimSpace(refreshToken) != "" {
		encryptedRefreshToken, err := s.cipher.Encrypt(refreshToken)
		if err != nil {
			return err
		}
		account.RefreshTokenEncrypted = encryptedRefreshToken
	}

	return s.repo.UpdateAccount(account)
}

func (s *AccountService) UpdateRuntimeCredentials(accountID uint, accessToken string, refreshToken string) error {
	return s.UpdateRuntimeTokens(accountID, accessToken, refreshToken)
}

func normalizeSub2APIAuthMode(accountType models.UpstreamType, authMode models.Sub2APIAuthMode) models.Sub2APIAuthMode {
	if accountType != models.UpstreamTypeSub2API {
		return ""
	}
	if authMode == models.Sub2APIAuthModePassword {
		return models.Sub2APIAuthModePassword
	}
	return models.Sub2APIAuthModeRefreshToken
}

func normalizeNewAPIAuthMode(accountType models.UpstreamType, authMode models.NewAPIAuthMode) models.NewAPIAuthMode {
	if accountType != models.UpstreamTypeNewAPI {
		return ""
	}
	return models.NewAPIAuthModeManual
}
