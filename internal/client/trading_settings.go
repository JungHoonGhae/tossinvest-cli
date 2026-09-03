package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
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
func (c *Client) GetTradingSettings(ctx context.Context, accountKey string) (domain.TradingSettings, error) {
	if err := c.requireSession(); err != nil {
		return domain.TradingSettings{}, err
	}
	accountKey, err := c.resolveAccountKey(ctx, accountKey)
	if err != nil {
		return domain.TradingSettings{}, err
	}

	var simpleTrade quoteEnvelope[bool]
	var exchangeChoice quoteEnvelope[string]
	var atsNotification quoteEnvelope[bool]
	var optionTick quoteEnvelope[struct {
		Requested     bool `json:"requested"`
		Serviced      bool `json:"serviced"`
		ShouldCharged bool `json:"shouldCharged"`
	}]
	if err := runReadBatch(
		readTask{label: "simple trade setting", run: func() error {
			return c.getJSONWithAccountKey(ctx, c.certBaseURL+simpleTradeSettingPath, accountKey, &simpleTrade)
		}},
		readTask{label: "investor exchange choice", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+investorExchangeChoicePath, &exchangeChoice)
		}},
		readTask{label: "ATS notification setting", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+atsNotificationPath, &atsNotification)
		}},
		readTask{label: "option real-time tick setting", run: func() error {
			return c.getJSON(ctx, c.certBaseURL+optionRealTimeTickPath, &optionTick)
		}},
	); err != nil {
		return domain.TradingSettings{}, err
	}

	return domain.TradingSettings{
		AccountScope:           confirmation.Token("account:" + accountKey),
		SimpleTradeEnabled:     simpleTrade.Result,
		InvestorExchangeChoice: exchangeChoice.Result,
		ATSNotificationEnabled: atsNotification.Result,
		OptionRealTimeTick: domain.OptionRealTimeTickStatus{
			Requested:        optionTick.Result.Requested,
			Serviced:         optionTick.Result.Serviced,
			RawShouldCharged: optionTick.Result.ShouldCharged,
		},
		FetchedAt: time.Now().UTC(),
	}, nil
}
