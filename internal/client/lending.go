package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type lendingExpectedRaw struct {
	ExpectedAmountUsdOneMonth float64 `json:"expectedAmountUsdOneMonth"`
	ExpectedAmountUsdOneYear  float64 `json:"expectedAmountUsdOneYear"`
	Items                     []struct {
		Guid      string  `json:"guid"`
		StockName string  `json:"stockName"`
		Amount    float64 `json:"amount"`
	} `json:"items"`
}

type lendingTopRevenueRaw struct {
	Items []struct {
		UserName   string  `json:"userName"`
		Revenue    float64 `json:"revenue"`
		RevenueKRW float64 `json:"revenueKrw"`
	} `json:"items"`
}

// GetLendingExpected fetches projected share-lending (대주) income for the
// account: monthly/yearly USD totals plus a per-stock breakdown. WTS-only.
func (c *Client) GetLendingExpected(ctx context.Context) (domain.LendingExpected, error) {
	if err := c.requireSession(); err != nil {
		return domain.LendingExpected{}, err
	}
	var env quoteEnvelope[lendingExpectedRaw]
	endpoint := c.certBaseURL + "/api/v1/lending/revenue/account/expected"
	if err := c.getJSON(ctx, endpoint, &env); err != nil {
		return domain.LendingExpected{}, err
	}
	out := domain.LendingExpected{
		OneMonthUSD: env.Result.ExpectedAmountUsdOneMonth,
		OneYearUSD:  env.Result.ExpectedAmountUsdOneYear,
		FetchedAt:   time.Now(),
	}
	for _, it := range env.Result.Items {
		out.Stocks = append(out.Stocks, domain.LendingExpectedStock{
			ProductCode: it.Guid,
			Name:        it.StockName,
			AmountUSD:   it.Amount,
		})
	}
	return out, nil
}

// GetTopLendingRevenue returns the anonymized share-lending revenue ranking.
// size <= 0 keeps every row supplied by the server.
func (c *Client) GetTopLendingRevenue(ctx context.Context, size int) (domain.LendingRevenueRanking, error) {
	if err := c.requireSession(); err != nil {
		return domain.LendingRevenueRanking{}, err
	}
	var env quoteEnvelope[lendingTopRevenueRaw]
	endpoint := c.certBaseURL + "/api/v1/lending/revenue/account/top-revenue"
	if err := c.getJSON(ctx, endpoint, &env); err != nil {
		return domain.LendingRevenueRanking{}, err
	}
	out := domain.LendingRevenueRanking{FetchedAt: time.Now().UTC()}
	for i, item := range env.Result.Items {
		if size > 0 && i >= size {
			break
		}
		out.Items = append(out.Items, domain.LendingRevenueRank{
			Rank: i + 1, UserName: item.UserName,
			Revenue: item.Revenue, RevenueKRW: item.RevenueKRW,
		})
	}
	return out, nil
}
