package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type buyingPowerRaw struct {
	Buyable                          bool    `json:"buyable"`
	ReceivableCurrency               string  `json:"receivableCurrency"`
	KRWAmount                        float64 `json:"krwAmount"`
	USDAmount                        float64 `json:"usdAmount"`
	USDReceivableKrwEquivalentAmount float64 `json:"usdReceivableKrwEquivalentAmount"`
	KRWMoneyOutAbleAmount            float64 `json:"krwMoneyOutAbleAmount"`
	RequiredDepositAmount            float64 `json:"requiredDepositAmount"`
	RequiredExchangeAmount           float64 `json:"requiredExchangeAmount"`
}

// GetOrderFunding reports whether the account can buy right now and, when it
// cannot, how much must be deposited or exchanged first.
// 공식 Open API 에 없는 web 전용 표면.
//
// This is the missing-amount view. `account summary` reports what is already
// orderable; this one reports the gap, which is what blocks an order.
func (c *Client) GetOrderFunding(ctx context.Context) (domain.OrderFunding, error) {
	if err := c.requireSession(); err != nil {
		return domain.OrderFunding{}, err
	}
	key, err := c.primaryAccountKey(ctx)
	if err != nil {
		return domain.OrderFunding{}, err
	}

	var envelope quoteEnvelope[buyingPowerRaw]
	url := c.infoBaseURL + "/api/v2/trading/order/buy-control/required-deposit-amount"
	if err := c.getJSONWithAccountKey(ctx, url, key, &envelope); err != nil {
		return domain.OrderFunding{}, err
	}
	r := envelope.Result

	return domain.OrderFunding{
		Buyable:                r.Buyable,
		ReceivableCurrency:     r.ReceivableCurrency,
		KRWAmount:              r.KRWAmount,
		USDAmount:              r.USDAmount,
		USDReceivableKRWEquiv:  r.USDReceivableKrwEquivalentAmount,
		KRWWithdrawable:        r.KRWMoneyOutAbleAmount,
		RequiredDepositAmount:  r.RequiredDepositAmount,
		RequiredExchangeAmount: r.RequiredExchangeAmount,
		FetchedAt:              time.Now().UTC(),
	}, nil
}
