package client

import (
	"context"
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
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/autotrade/open-banking/info/find", &env); err != nil {
		return domain.OpenBankingStatus{}, err
	}
	var creatable quoteEnvelope[bool]
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/autotrade/open-banking/creatable", &creatable); err != nil {
		return domain.OpenBankingStatus{}, err
	}
	var registration quoteEnvelope[bool]
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/autotrade/open-banking/need-registration", &registration); err != nil {
		return domain.OpenBankingStatus{}, err
	}

	out := domain.OpenBankingStatus{
		HolderName:              env.Result.Name,
		LinkedAccountCount:      len(env.Result.OpenBankingAccounts),
		RegistrableAccountCount: len(env.Result.RegistrableAccounts),
		SavingCount:             env.Result.SavingCount,
		ConnectionCreatable:     creatable.Result,
		RegistrationRequired:    registration.Result,
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

// GetNotificationSettings returns every WTS notification toggle. The wire's
// internal userId is intentionally omitted from the domain model.
func (c *Client) GetNotificationSettings(ctx context.Context) (domain.NotificationSettings, error) {
	if err := c.requireSession(); err != nil {
		return domain.NotificationSettings{}, err
	}
	var env quoteEnvelope[[]struct {
		ID        int64   `json:"id"`
		Type      *string `json:"type"`
		Enabled   bool    `json:"enabled"`
		CreatedAt string  `json:"createdAt"`
		UpdatedAt string  `json:"updatedAt"`
	}]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/user-alimies", &env); err != nil {
		return domain.NotificationSettings{}, err
	}

	out := domain.NotificationSettings{
		Settings:  make([]domain.NotificationSetting, 0, len(env.Result)),
		FetchedAt: time.Now().UTC(),
	}
	for _, item := range env.Result {
		setting := domain.NotificationSetting{
			ID:        item.ID,
			Enabled:   item.Enabled,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
		if item.Type != nil {
			setting.Type = *item.Type
		}
		out.Settings = append(out.Settings, setting)
	}
	return out, nil
}
