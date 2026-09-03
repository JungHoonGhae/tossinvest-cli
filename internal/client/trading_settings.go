package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const (
	simpleTradeSettingPath     = "/api/v1/trading/settings/simple-trade"
	investorExchangeChoicePath = "/api/v2/trading/settings/investor-exchange-choice-type"
	atsNotificationPath        = "/api/v1/users/settings/me/ats-notification"
	optionRealTimeTickPath     = "/api/v1/member-subscriptions/get-option-real-time-tick"
)

// GetTradingSettings assembles the read-only WTS Securities trading settings.
// Only simple-trade is account-scoped; the other three contracts are user-wide.
func (c *Client) GetTradingSettings(ctx context.Context) (domain.TradingSettings, error) {
	if err := c.requireSession(); err != nil {
		return domain.TradingSettings{}, err
	}
	accountKey, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.TradingSettings{}, err
	}

	var simpleTrade quoteEnvelope[bool]
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+simpleTradeSettingPath, accountKey, &simpleTrade); err != nil {
		return domain.TradingSettings{}, fmt.Errorf("simple trade setting: %w", err)
	}
	var exchangeChoice quoteEnvelope[string]
	if err := c.getJSON(ctx, c.certBaseURL+investorExchangeChoicePath, &exchangeChoice); err != nil {
		return domain.TradingSettings{}, fmt.Errorf("investor exchange choice: %w", err)
	}
	var atsNotification quoteEnvelope[bool]
	if err := c.getJSON(ctx, c.certBaseURL+atsNotificationPath, &atsNotification); err != nil {
		return domain.TradingSettings{}, fmt.Errorf("ATS notification setting: %w", err)
	}
	var optionTick quoteEnvelope[struct {
		Requested     bool `json:"requested"`
		Serviced      bool `json:"serviced"`
		ShouldCharged bool `json:"shouldCharged"`
	}]
	if err := c.getJSON(ctx, c.certBaseURL+optionRealTimeTickPath, &optionTick); err != nil {
		return domain.TradingSettings{}, fmt.Errorf("option real-time tick setting: %w", err)
	}

	return domain.TradingSettings{
		SimpleTradeEnabled:     simpleTrade.Result,
		InvestorExchangeChoice: exchangeChoice.Result,
		ATSNotificationEnabled: atsNotification.Result,
		OptionRealTimeTick: domain.OptionRealTimeTickStatus{
			Requested:     optionTick.Result.Requested,
			Serviced:      optionTick.Result.Serviced,
			ShouldCharged: optionTick.Result.ShouldCharged,
		},
		FetchedAt: time.Now().UTC(),
	}, nil
}
