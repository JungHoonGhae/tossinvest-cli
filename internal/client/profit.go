package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type dualCurrencyRaw struct {
	KRW float64  `json:"krw"`
	USD *float64 `json:"usd"`
}

func (r dualCurrencyRaw) toDomain() domain.DualCurrency {
	return domain.DualCurrency{KRW: r.KRW, USD: r.USD}
}

type profitByTypeRaw struct {
	Amount         dualCurrencyRaw `json:"amount"`
	EarningRate    dualCurrencyRaw `json:"earningRate"`
	PurchaseAmount dualCurrencyRaw `json:"purchaseAmount"`
}

func (r profitByTypeRaw) toDomain() domain.ProfitByType {
	return domain.ProfitByType{
		Amount:         r.Amount.toDomain(),
		EarningRate:    r.EarningRate.toDomain(),
		PurchaseAmount: r.PurchaseAmount.toDomain(),
	}
}

type profitOverviewRaw struct {
	TotalAssetAmount dualCurrencyRaw `json:"totalAssetAmount"`
	EarningAmount    dualCurrencyRaw `json:"earningAmount"`
	Sales            profitByTypeRaw `json:"sales"`
	Lending          profitByTypeRaw `json:"lending"`
	Dividend         profitByTypeRaw `json:"dividend"`
	Maturity         profitByTypeRaw `json:"maturity"`
	Interest         float64         `json:"interest"`
}

// GetProfitOverview fetches the account's cumulative profit breakdown across
// every category (매매손익·대여료·배당·만기·예탁금이자), each in KRW and USD.
// This is a cumulative realized-profit view, distinct from `account summary`
// (which reports current holdings valuation). WTS-only.
func (c *Client) GetProfitOverview(ctx context.Context) (domain.ProfitOverview, error) {
	if err := c.requireSession(); err != nil {
		return domain.ProfitOverview{}, err
	}
	var env quoteEnvelope[profitOverviewRaw]
	endpoint := c.certBaseURL + "/api/v1/profit/overview"
	// Empty body returns the full all-market, all-time breakdown.
	if err := c.postJSON(ctx, endpoint, []byte(`{}`), &env); err != nil {
		return domain.ProfitOverview{}, err
	}
	r := env.Result
	return domain.ProfitOverview{
		TotalAssetAmount: r.TotalAssetAmount.toDomain(),
		EarningAmount:    r.EarningAmount.toDomain(),
		Sales:            r.Sales.toDomain(),
		Lending:          r.Lending.toDomain(),
		Dividend:         r.Dividend.toDomain(),
		Maturity:         r.Maturity.toDomain(),
		Interest:         r.Interest,
		FetchedAt:        time.Now(),
	}, nil
}
