package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type chartCandleRaw struct {
	DT     string  `json:"dt"`
	Base   float64 `json:"base"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type chartResult struct {
	Code     string           `json:"code"`
	Exchange string           `json:"exchange"`
	Candles  []chartCandleRaw `json:"candles"`
}

func (c *Client) GetChart(ctx context.Context, symbol, interval string, count int) (domain.Chart, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.Chart{}, err
	}

	info, err := c.getStockInfo(ctx, productCode)
	if err != nil {
		return domain.Chart{}, err
	}

	rangeParam, err := normalizeChartInterval(interval)
	if err != nil {
		return domain.Chart{}, err
	}

	if count <= 0 {
		count = 30
	}

	sType := deriveSecurityType(productCode)
	endpoint, err := url.Parse(fmt.Sprintf("%s/api/v1/c-chart/%s/%s/%s", c.infoBaseURL, sType, productCode, rangeParam))
	if err != nil {
		return domain.Chart{}, err
	}
	query := endpoint.Query()
	query.Set("count", strconv.Itoa(count))
	query.Set("useAdjustedRate", "true")
	query.Set("session", "main")
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[chartResult]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.Chart{}, err
	}

	chart := domain.Chart{
		ProductCode: envelope.Result.Code,
		Symbol:      info.Symbol,
		Name:        info.Name,
		Interval:    interval,
		FetchedAt:   time.Now().UTC(),
	}
	if chart.ProductCode == "" {
		chart.ProductCode = productCode
	}

	chart.Candles = make([]domain.Candle, 0, len(envelope.Result.Candles))
	for _, raw := range envelope.Result.Candles {
		t, parseErr := time.Parse(time.RFC3339, raw.DT)
		if parseErr != nil {
			t = time.Time{}
		}
		chart.Candles = append(chart.Candles, domain.Candle{
			Time:   t,
			Open:   raw.Open,
			High:   raw.High,
			Low:    raw.Low,
			Close:  raw.Close,
			Volume: raw.Volume,
		})
		if chart.Base == 0 && raw.Base != 0 {
			chart.Base = raw.Base
		}
	}

	for i, j := 0, len(chart.Candles)-1; i < j; i, j = i+1, j-1 {
		chart.Candles[i], chart.Candles[j] = chart.Candles[j], chart.Candles[i]
	}

	return chart, nil
}

func deriveSecurityType(productCode string) string {
	// The chart path is segmented by security type (us-s stocks, us-o options,
	// us-b bonds, us-i indices — the set the web bundle uses). Option contracts
	// fall through to us-s otherwise and the server answers 400.
	if strings.HasPrefix(productCode, "OPT_") {
		return "us-o"
	}
	if len(productCode) >= 7 && productCode[0] == 'A' {
		for i := 1; i < len(productCode); i++ {
			if productCode[i] < '0' || productCode[i] > '9' {
				return "us-s"
			}
		}
		return "kr-s"
	}
	return "us-s"
}

func normalizeChartInterval(interval string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(interval))
	if s == "" {
		return "", fmt.Errorf("interval is required")
	}
	s = strings.TrimSuffix(s, "min")
	s = strings.TrimSuffix(s, "m")
	n, err := strconv.Atoi(s)
	if err != nil {
		n = 0
	}
	switch n {
	case 1, 3, 5, 10, 15, 30, 60:
		return fmt.Sprintf("min:%d", n), nil
	default:
		return "", fmt.Errorf("unsupported interval %q; expected one of: 1m, 3m, 5m, 10m, 15m, 30m, 60m", interval)
	}
}
