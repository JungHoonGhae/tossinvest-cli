package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const (
	hiddenHoldingsListPath = "/api/v2/hidden-stocks"
	hideHoldingPath        = "/api/v1/my-assets/hidden-stocks/hide"
	showHoldingPath        = "/api/v1/my-assets/hidden-stocks/show"
)

type hiddenHoldingsEnvelope struct {
	Result struct {
		HiddenStocks []struct {
			StockCode        string  `json:"stockCode"`
			StockName        string  `json:"stockName"`
			Type             string  `json:"type"`
			LogoImageURL     string  `json:"logoImageUrl"`
			TradableQuantity float64 `json:"tradableQuantity"`
		} `json:"hiddenStocks"`
	} `json:"result"`
}

// ListHiddenHoldings returns account-scoped holdings hidden in the Securities
// portfolio. An empty account key resolves to the primary Securities account.
func (c *Client) ListHiddenHoldings(ctx context.Context, accountKey string) (domain.HiddenHoldings, error) {
	if err := c.requireSession(); err != nil {
		return domain.HiddenHoldings{}, err
	}
	var err error
	accountKey, err = c.resolveAccountKey(ctx, accountKey)
	if err != nil {
		return domain.HiddenHoldings{}, err
	}
	var envelope hiddenHoldingsEnvelope
	if err := c.getJSONWithAccountKey(ctx, c.certBaseURL+hiddenHoldingsListPath, accountKey, &envelope); err != nil {
		return domain.HiddenHoldings{}, err
	}
	holdings := make([]domain.HiddenHolding, 0, len(envelope.Result.HiddenStocks))
	for _, item := range envelope.Result.HiddenStocks {
		holdings = append(holdings, domain.HiddenHolding{
			ProductCode:      item.StockCode,
			Name:             item.StockName,
			Type:             item.Type,
			LogoImageURL:     item.LogoImageURL,
			TradableQuantity: item.TradableQuantity,
		})
	}
	return domain.HiddenHoldings{AccountKey: accountKey, Holdings: holdings}, nil
}

// HideHolding hides one holding in the account's Securities portfolio.
func (c *Client) HideHolding(ctx context.Context, accountKey, productCode string) error {
	return c.setHoldingHidden(ctx, accountKey, productCode, hideHoldingPath)
}

// ShowHolding restores one hidden holding to the account's Securities portfolio.
func (c *Client) ShowHolding(ctx context.Context, accountKey, productCode string) error {
	return c.setHoldingHidden(ctx, accountKey, productCode, showHoldingPath)
}

func (c *Client) setHoldingHidden(ctx context.Context, accountKey, productCode, path string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	accountKey = strings.TrimSpace(accountKey)
	productCode = strings.TrimSpace(productCode)
	if accountKey == "" {
		return fmt.Errorf("account key is required")
	}
	if productCode == "" {
		return fmt.Errorf("product code is required")
	}
	body, err := json.Marshal(map[string]string{"stockCode": productCode})
	if err != nil {
		return err
	}
	return c.mutateJSONWithAccountKey(ctx, http.MethodPost, c.apiBaseURL+path, body, accountKey)
}
