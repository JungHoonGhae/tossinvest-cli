package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// GetOpenBankingStatus returns the read-only connection state used by stock
// accumulation funding. The two account arrays had no stable observed item
// schema, so their counts are retained without guessing at fields.
func (c *Client) GetOpenBankingStatus(ctx context.Context) (domain.OpenBankingStatus, error) {
	if err := c.requireSession(); err != nil {
		return domain.OpenBankingStatus{}, err
	}
	var env quoteEnvelope[struct {
		Name             string `json:"name"`
		ConnectedAccount *struct {
			AccountNo     string `json:"accountNo"`
			BankCode      string `json:"bankCode"`
			OpenBankingID int64  `json:"openBankingId"`
		} `json:"connectedOpenBankingAccount"`
		OpenBankingAccounts []struct{} `json:"openBankingAccounts"`
		RegistrableAccounts []struct{} `json:"registrableAccounts"`
		SavingCount         int        `json:"savingCount"`
	}]
	var creatable quoteEnvelope[bool]
	var registration quoteEnvelope[bool]
	var autoTrading quoteEnvelope[struct {
		ConnectedAccountBankCode string `json:"connectedAccountBankCode"`
		Registered               bool   `json:"isRegistered"`
	}]
	if err := runReadBatch(
		readTask{label: "open banking connection info", run: func() error {
			return c.getJSON(ctx, c.apiBaseURL+"/api/v1/autotrade/open-banking/info/find", &env)
		}},
		readTask{label: "open banking connection eligibility", run: func() error {
			return c.getJSON(ctx, c.apiBaseURL+"/api/v1/autotrade/open-banking/creatable", &creatable)
		}},
		readTask{label: "open banking registration requirement", run: func() error {
			return c.getJSON(ctx, c.apiBaseURL+"/api/v1/autotrade/open-banking/need-registration", &registration)
		}},
		readTask{label: "auto-trading open banking registration", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+"/api/v1/trading/open-banking/auto-trading", &autoTrading)
		}},
	); err != nil {
		return domain.OpenBankingStatus{}, err
	}

	out := domain.OpenBankingStatus{
		HolderName:              env.Result.Name,
		LinkedAccountCount:      len(env.Result.OpenBankingAccounts),
		RegistrableAccountCount: len(env.Result.RegistrableAccounts),
		SavingCount:             env.Result.SavingCount,
		ConnectionCreatable:     creatable.Result,
		RegistrationRequired:    registration.Result,
		AutoTradingRegistered:   autoTrading.Result.Registered,
		AutoTradingBankCode:     autoTrading.Result.ConnectedAccountBankCode,
		FetchedAt:               time.Now().UTC(),
	}
	if item := env.Result.ConnectedAccount; item != nil {
		out.ConnectedAccount = &domain.OpenBankingAccount{
			AccountNo:     item.AccountNo,
			BankCode:      item.BankCode,
			OpenBankingID: item.OpenBankingID,
		}
	}
	return out, nil
}

// GetNotificationSettings returns every WTS notification toggle. Toss emits
// one legitimate untyped row in the current contract, which is retained with
// an empty Type. The wire's internal userId is intentionally omitted.
func (c *Client) GetNotificationSettings(ctx context.Context) (domain.NotificationSettings, error) {
	if err := c.requireSession(); err != nil {
		return domain.NotificationSettings{}, err
	}
	var env quoteEnvelope[[]struct {
		ID        int64   `json:"id"`
		Type      *string `json:"type"`
		Enabled   *bool   `json:"enabled"`
		CreatedAt string  `json:"createdAt"`
		UpdatedAt string  `json:"updatedAt"`
	}]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/user-alimies", &env); err != nil {
		return domain.NotificationSettings{}, err
	}
	if env.Result == nil {
		return domain.NotificationSettings{}, fmt.Errorf("notification settings response missing result array")
	}

	out := domain.NotificationSettings{
		Settings:  make([]domain.NotificationSetting, 0, len(env.Result)),
		FetchedAt: time.Now().UTC(),
	}
	for index, item := range env.Result {
		if item.Enabled == nil {
			return domain.NotificationSettings{}, fmt.Errorf("notification settings result[%d] missing enabled boolean", index)
		}
		settingType := ""
		if item.Type != nil {
			settingType = strings.TrimSpace(*item.Type)
		}
		setting := domain.NotificationSetting{
			ID:        item.ID,
			Type:      settingType,
			Enabled:   *item.Enabled,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		out.Settings = append(out.Settings, setting)
	}
	return out, nil
}
