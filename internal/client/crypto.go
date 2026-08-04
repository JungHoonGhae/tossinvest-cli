package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type cryptoPriceRaw struct {
	ProductCode string  `json:"productCode"`
	Base        float64 `json:"base"`
	Open        float64 `json:"open"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
	Value       float64 `json:"value"`
	ChangeType  string  `json:"changeType"`
	High52w     float64 `json:"high52w"`
	Low52w      float64 `json:"low52w"`
	USDPerKRW   float64 `json:"usdPerKrwExchangeRate"`
	Premium     float64 `json:"premium"`
	PremiumRate float64 `json:"premiumRate"`
}

// cryptoProductCode expands a short symbol to the code the API takes.
// A caller who already knows the full code (`VWAP.KRW-BTC`) gets it through
// untouched, so a pair Toss adds later works without a release here.
func cryptoProductCode(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if strings.HasPrefix(s, "VWAP.") {
		return s
	}
	return "VWAP.KRW-" + s
}

// cryptoSymbol is the inverse, for display.
func cryptoSymbol(productCode string) string {
	if _, sym, ok := strings.Cut(productCode, "-"); ok {
		return sym
	}
	return productCode
}

// GetCryptoPrices returns the KRW crypto tape for the given symbols.
// 공식 Open API 에 없는 web 전용 표면.
//
// Symbols may be short (`BTC`) or full product codes. The endpoint takes them
// comma-joined and returns one entry per code it knows — unknown codes are
// dropped silently rather than erroring, so the result can be shorter than the
// request.
func (c *Client) GetCryptoPrices(ctx context.Context, symbols []string) (domain.CryptoPrices, error) {
	if err := c.requireSession(); err != nil {
		return domain.CryptoPrices{}, err
	}
	if len(symbols) == 0 {
		return domain.CryptoPrices{}, fmt.Errorf("at least one symbol is required")
	}

	codes := make([]string, 0, len(symbols))
	for _, s := range symbols {
		if code := cryptoProductCode(s); code != "VWAP.KRW-" {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return domain.CryptoPrices{}, fmt.Errorf("at least one symbol is required")
	}

	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/crypto-prices")
	if err != nil {
		return domain.CryptoPrices{}, err
	}
	query := endpoint.Query()
	query.Set("productCodes", strings.Join(codes, ","))
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[[]cryptoPriceRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.CryptoPrices{}, err
	}

	out := domain.CryptoPrices{FetchedAt: time.Now().UTC()}
	for _, p := range envelope.Result {
		price := domain.CryptoPrice{
			ProductCode: p.ProductCode,
			Symbol:      cryptoSymbol(p.ProductCode),
			Base:        p.Base,
			Open:        p.Open,
			High:        p.High,
			Low:         p.Low,
			Close:       p.Close,
			ChangeType:  p.ChangeType,
			Volume:      p.Volume,
			Value:       p.Value,
			High52w:     p.High52w,
			Low52w:      p.Low52w,
			USDPerKRW:   p.USDPerKRW,
			Premium:     p.Premium,
			PremiumRate: p.PremiumRate,
		}
		// 서버는 등락폭·등락률을 안 준다. base 는 기준가(전일 종가)라 여기서 낸다.
		// base 가 0 이면 나누지 않는다 — NaN/Inf 가 출력까지 흘러가면 조용히 깨진다.
		if p.Base != 0 {
			price.Change = p.Close - p.Base
			price.ChangeRate = (p.Close - p.Base) / p.Base * 100
		}
		out.Prices = append(out.Prices, price)
	}
	return out, nil
}
