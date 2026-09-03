package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type priceAlertsEnvelope struct {
	Result []struct {
		TargetPrice float64 `json:"targetPrice"`
		Currency    string  `json:"currency"`
	} `json:"result"`
}

// ResolveProductCode exposes the client's established symbol/name resolver to
// higher-level services without exposing any endpoint details.
func (c *Client) ResolveProductCode(ctx context.Context, symbol string) (string, error) {
	return c.resolveProductCode(ctx, symbol)
}

// ListPriceAlerts returns the target-price alerts for a canonical product code.
func (c *Client) ListPriceAlerts(ctx context.Context, productCode string) (domain.PriceAlerts, error) {
	if err := c.requireSession(); err != nil {
		return domain.PriceAlerts{}, err
	}
	productCode = strings.TrimSpace(productCode)
	if productCode == "" {
		return domain.PriceAlerts{}, fmt.Errorf("product code is required")
	}
	var envelope priceAlertsEnvelope
	endpoint := fmt.Sprintf("%s/api/v1/user-price-alimy/%s", c.apiBaseURL, url.PathEscape(productCode))
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.PriceAlerts{}, err
	}
	alerts := make([]domain.PriceAlert, 0, len(envelope.Result))
	for _, item := range envelope.Result {
		alerts = append(alerts, domain.PriceAlert{
			TargetPrice: item.TargetPrice,
			Currency:    strings.ToUpper(strings.TrimSpace(item.Currency)),
		})
	}
	return domain.PriceAlerts{ProductCode: productCode, Alerts: alerts}, nil
}

// AddPriceAlert creates one target-price alert. Safety preview and confirmation
// are owned by pricealert.Service; this method only implements the WTS contract.
func (c *Client) AddPriceAlert(ctx context.Context, productCode string, targetPrice float64, currency string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"targetPrice": targetPrice,
		"currency":    strings.ToUpper(strings.TrimSpace(currency)),
	})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/v1/user-price-alimy/%s", c.apiBaseURL, url.PathEscape(strings.TrimSpace(productCode)))
	return c.mutateJSON(ctx, http.MethodPost, endpoint, body, nil)
}

// DeletePriceAlert deletes the exact currency/target-price tuple.
func (c *Client) DeletePriceAlert(ctx context.Context, productCode string, targetPrice float64, currency string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	endpoint := fmt.Sprintf(
		"%s%s/%s/%s/%s",
		c.apiBaseURL,
		"/api/v1/user-price-alimy",
		url.PathEscape(strings.TrimSpace(productCode)),
		url.PathEscape(strings.ToUpper(strings.TrimSpace(currency))),
		url.PathEscape(strconv.FormatFloat(targetPrice, 'f', -1, 64)),
	)
	return c.mutateJSON(ctx, http.MethodDelete, endpoint, nil, nil)
}
