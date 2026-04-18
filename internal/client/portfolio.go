package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/junghoonkye/tossinvest-cli/internal/domain"
)

type assetSectionsEnvelope struct {
	Result struct {
		Sections []struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		} `json:"sections"`
	} `json:"result"`
}

type sortedOverviewData struct {
	Products []struct {
		MarketType string `json:"marketType"`
		Items      []struct {
			StockCode    string  `json:"stockCode"`
			StockSymbol  *string `json:"stockSymbol"`
			StockName    string  `json:"stockName"`
			Quantity     float64 `json:"quantity"`
			CurrentPrice struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"currentPrice"`
			PurchasePrice struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"purchasePrice"`
			EvaluatedAmount struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"evaluatedAmount"`
			ProfitLossAmount struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"profitLossAmount"`
			ProfitLossRate struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"profitLossRate"`
			DailyProfitLossAmount struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"dailyProfitLossAmount"`
			DailyProfitLossRate struct {
				KRW *float64 `json:"krw"`
				USD *float64 `json:"usd"`
			} `json:"dailyProfitLossRate"`
			MarketCode string `json:"marketCode"`
		} `json:"items"`
	} `json:"products"`
}

func (c *Client) ListPositions(ctx context.Context) ([]domain.Position, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}

	var envelope assetSectionsEnvelope
	if err := c.postJSON(ctx, c.certBaseURL+"/api/v2/dashboard/asset/sections/all", json.RawMessage("{}"), &envelope); err != nil {
		return nil, err
	}

	var overview sortedOverviewData
	found := false
	for _, section := range envelope.Result.Sections {
		if section.Type != "SORTED_OVERVIEW" {
			continue
		}
		if err := json.Unmarshal(section.Data, &overview); err != nil {
			return nil, err
		}
		found = true
		break
	}
	if !found {
		return nil, fmt.Errorf("SORTED_OVERVIEW section not found")
	}

	positions := []domain.Position{}
	for _, product := range overview.Products {
		for _, item := range product.Items {
			symbol := item.StockCode
			if item.StockSymbol != nil && *item.StockSymbol != "" {
				symbol = *item.StockSymbol
			}

			positions = append(positions, domain.Position{
				ProductCode:     item.StockCode,
				Symbol:          symbol,
				Name:            item.StockName,
				MarketType:      product.MarketType,
				MarketCode:      item.MarketCode,
				Quantity:        item.Quantity,
				AveragePrice:    coalesceMoney(item.PurchasePrice.KRW, item.PurchasePrice.USD),
				CurrentPrice:    coalesceMoney(item.CurrentPrice.KRW, item.CurrentPrice.USD),
				MarketValue:     coalesceMoney(item.EvaluatedAmount.KRW, item.EvaluatedAmount.USD),
				UnrealizedPnL:   coalesceMoney(item.ProfitLossAmount.KRW, item.ProfitLossAmount.USD),
				ProfitRate:      coalesceMoney(item.ProfitLossRate.KRW, item.ProfitLossRate.USD),
				DailyProfitLoss: coalesceMoney(item.DailyProfitLossAmount.KRW, item.DailyProfitLossAmount.USD),
				DailyProfitRate: coalesceMoney(item.DailyProfitLossRate.KRW, item.DailyProfitLossRate.USD),

				AveragePriceUSD:    derefFloat(item.PurchasePrice.USD),
				CurrentPriceUSD:    derefFloat(item.CurrentPrice.USD),
				MarketValueUSD:     derefFloat(item.EvaluatedAmount.USD),
				UnrealizedPnLUSD:   derefFloat(item.ProfitLossAmount.USD),
				ProfitRateUSD:      derefFloat(item.ProfitLossRate.USD),
				DailyProfitLossUSD: derefFloat(item.DailyProfitLossAmount.USD),
				DailyProfitRateUSD: derefFloat(item.DailyProfitLossRate.USD),
			})
		}
	}

	return positions, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, body json.RawMessage, target any) error {
	req, err := httpNewRequestWithBody(ctx, endpoint, body)
	if err != nil {
		return err
	}
	c.applySession(req)
	if err := c.prepareCertRequest(ctx, endpoint, req); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newStatusError(resp.StatusCode, endpoint, data)
	}

	return json.Unmarshal(data, target)
}

func httpNewRequestWithBody(ctx context.Context, endpoint string, body []byte) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
}

func coalesceMoney(values ...*float64) float64 {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return 0
}

func derefFloat(p *float64) float64 {
	if p != nil {
		return *p
	}
	return 0
}
