package client

import (
	"context"
	"fmt"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type transferIncomeRaw struct {
	PagingParam struct {
		Number int `json:"number"`
		Size   int `json:"size"`
	} `json:"pagingParam"`
	Body struct {
		Summary struct {
			TransferIncomeTaxRate float64 `json:"transferIncomeTaxRate"`
			LocalIncomeTaxRate    float64 `json:"localIncomeTaxRate"`
			BaseDeduction         float64 `json:"baseDeduction"`
			TotalProfitLossAmount float64 `json:"totalProfitLossAmount"`
			TransferIncomeTax     float64 `json:"transferIncomeTax"`
			LocalIncomeTax        float64 `json:"localIncomeTax"`
			TotalTax              float64 `json:"totalTax"`
		} `json:"transferIncomeSummary"`
		Items []struct {
			StockCode    string  `json:"stockCode"`
			StockSymbol  string  `json:"stockSymbol"`
			StockName    string  `json:"stockName"`
			SellQuantity float64 `json:"sellQuantity"`
			SellAmount   float64 `json:"sellAmount"`
			BuyAmount    float64 `json:"buyAmount"`
			Expense      float64 `json:"expense"`
			ProfitLoss   float64 `json:"profitLossAmount"`
			SettleDate   string  `json:"finalSettlementKorDate"`
			Settled      bool    `json:"settled"`
		} `json:"items"`
	} `json:"body"`
	LastPage bool `json:"lastPage"`
}

// maxTransferIncomePages caps pagination so a runaway/looping server response
// can't spin forever. A tax year rarely exceeds a few hundred sell lines.
const maxTransferIncomePages = 50

// GetOverseasTransferIncome fetches the overseas-stock transfer-income (해외
// 주식 양도소득) report for a tax year: a tax summary plus every sold stock's
// profit/loss line, paged through and aggregated. Used for capital-gains tax
// filing. All amounts are KRW. WTS-only.
func (c *Client) GetOverseasTransferIncome(ctx context.Context, year int) (domain.OverseasTransferIncome, error) {
	if err := c.requireSession(); err != nil {
		return domain.OverseasTransferIncome{}, err
	}
	out := domain.OverseasTransferIncome{Year: year, FetchedAt: time.Now()}
	summarySet := false
	for page := 1; page <= maxTransferIncomePages; page++ {
		var env quoteEnvelope[transferIncomeRaw]
		endpoint := fmt.Sprintf("%s/api/v1/my-assets/transfer-income/overseas?year=%d&number=%d",
			c.apiBaseURL, year, page)
		if err := c.getJSON(ctx, endpoint, &env); err != nil {
			return domain.OverseasTransferIncome{}, err
		}
		r := env.Result
		if !summarySet {
			s := r.Body.Summary
			out.TaxRate = s.TransferIncomeTaxRate
			out.LocalTaxRate = s.LocalIncomeTaxRate
			out.BaseDeduction = s.BaseDeduction
			out.TotalProfitLoss = s.TotalProfitLossAmount
			out.TransferIncomeTax = s.TransferIncomeTax
			out.LocalIncomeTax = s.LocalIncomeTax
			out.TotalTax = s.TotalTax
			summarySet = true
		}
		for _, it := range r.Body.Items {
			out.Stocks = append(out.Stocks, domain.TransferIncomeStock{
				Symbol:         it.StockSymbol,
				Name:           it.StockName,
				StockCode:      it.StockCode,
				SellQuantity:   it.SellQuantity,
				SellAmount:     it.SellAmount,
				BuyAmount:      it.BuyAmount,
				Expense:        it.Expense,
				ProfitLoss:     it.ProfitLoss,
				SettlementDate: it.SettleDate,
				Settled:        it.Settled,
			})
		}
		if r.LastPage || len(r.Body.Items) == 0 {
			break
		}
	}
	return out, nil
}
