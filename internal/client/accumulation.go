package client

import (
	"context"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type accumulationPlanRaw struct {
	ID                    int64   `json:"id"`
	Symbol                string  `json:"symbol"`
	StockCode             string  `json:"stockCode"`
	StockName             string  `json:"stockName"`
	CountryCode           string  `json:"countryCode"`
	Currency              string  `json:"currency"`
	PlanType              string  `json:"planType"`
	Iteration             string  `json:"iteration"`
	IterateTarget         int     `json:"iterateTarget"`
	InvestAmt             float64 `json:"investAmt"`
	InvestQty             float64 `json:"investQty"`
	TradeStatus           string  `json:"tradeStatus"`
	IsPaused              bool    `json:"isPaused"`
	InvestStartDate       string  `json:"investStartDate"`
	InvestEndDate         string  `json:"investEndDate"`
	ProceededRound        int     `json:"proceededRound"`
	SucceededRound        int     `json:"succeededRound"`
	TotalExecutedAmount   float64 `json:"totalExecutedAmount"`
	TotalExecutedQuantity float64 `json:"totalExecutedQuantity"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
}

func (r accumulationPlanRaw) toDomain() domain.AccumulationPlan {
	return domain.AccumulationPlan{
		ID:                    r.ID,
		Symbol:                r.Symbol,
		StockCode:             r.StockCode,
		StockName:             r.StockName,
		CountryCode:           r.CountryCode,
		Currency:              r.Currency,
		PlanType:              r.PlanType,
		Iteration:             r.Iteration,
		IterateTarget:         r.IterateTarget,
		InvestAmount:          r.InvestAmt,
		InvestQuantity:        r.InvestQty,
		TradeStatus:           r.TradeStatus,
		IsPaused:              r.IsPaused,
		InvestStartDate:       r.InvestStartDate,
		InvestEndDate:         r.InvestEndDate,
		ProceededRound:        r.ProceededRound,
		SucceededRound:        r.SucceededRound,
		TotalExecutedAmount:   r.TotalExecutedAmount,
		TotalExecutedQuantity: r.TotalExecutedQuantity,
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}
}

// ListAccumulationPlans fetches every "stock accumulation" (주식모으기)
// recurring-buy plan on the account, across all stocks and statuses
// (including paused). WTS-only.
func (c *Client) ListAccumulationPlans(ctx context.Context) (domain.AccumulationPlans, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccumulationPlans{}, err
	}
	var env quoteEnvelope[[]accumulationPlanRaw]
	endpoint := c.apiBaseURL + "/api/v2/autotrade/plan/find"
	if err := c.getJSON(ctx, endpoint, &env); err != nil {
		return domain.AccumulationPlans{}, err
	}
	out := domain.AccumulationPlans{FetchedAt: time.Now()}
	for _, raw := range env.Result {
		out.Plans = append(out.Plans, raw.toDomain())
	}
	return out, nil
}

// GetAccumulationPlansByStock fetches the accumulation plan(s) for a single
// stock (accepts a plain ticker like "005930"/"AAPL" or a Toss product code
// like "A005930" — resolved the same way `quote get` resolves symbols) — a
// stock can have plan history, so this returns a list. WTS-only.
func (c *Client) GetAccumulationPlansByStock(ctx context.Context, symbol string) (domain.AccumulationPlans, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccumulationPlans{}, err
	}
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.AccumulationPlans{}, err
	}
	var env quoteEnvelope[[]accumulationPlanRaw]
	endpoint := c.certBaseURL + "/api/v1/growth/autotrade/plan/stock/" + productCode
	if err := c.getJSON(ctx, endpoint, &env); err != nil {
		return domain.AccumulationPlans{}, err
	}
	out := domain.AccumulationPlans{FetchedAt: time.Now()}
	for _, raw := range env.Result {
		out.Plans = append(out.Plans, raw.toDomain())
	}
	return out, nil
}
