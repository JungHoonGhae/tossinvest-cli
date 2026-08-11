package official

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type apiListedStock struct {
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	IsinCode      string `json:"isinCode"`
	SecurityType  string `json:"securityType"`
	IsCommonShare bool   `json:"isCommonShare"`
}

// Markets the universe endpoint accepts. Kept here rather than validated
// server-side only, so a typo fails with the list instead of a bare 400.
var universeMarkets = []string{"KOSPI", "KOSDAQ", "NYSE", "NASDAQ", "AMEX", "KR_ETC", "US_ETC"}

// UniverseMarkets returns the accepted market codes (CLI help, MCP params).
func UniverseMarkets() []string { return append([]string(nil), universeMarkets...) }

// ListStocks returns a market's tradable universe, sorted by symbol.
//
// There is no pagination: the server answers with the whole filtered set in one
// response (thousands of rows). It is low-churn batch data refreshed daily, so
// callers that need it repeatedly should cache rather than re-request.
func (c *Client) ListStocks(ctx context.Context, market, status, securityType string, commonShareOnly bool) (domain.StockUniverse, error) {
	m := strings.ToUpper(strings.TrimSpace(market))
	if m == "" {
		return domain.StockUniverse{}, fmt.Errorf("market is required (one of: %s)", strings.Join(universeMarkets, ", "))
	}
	known := false
	for _, v := range universeMarkets {
		if v == m {
			known = true
			break
		}
	}
	if !known {
		return domain.StockUniverse{}, fmt.Errorf("unknown market %q (want one of: %s)", market, strings.Join(universeMarkets, ", "))
	}

	q := url.Values{}
	q.Set("market", m)
	if s := strings.ToUpper(strings.TrimSpace(status)); s != "" {
		q.Set("status", s)
	}
	if t := strings.ToUpper(strings.TrimSpace(securityType)); t != "" {
		q.Set("securityType", t)
	}
	// 기본값이 있는 필터는 켤 때만 보낸다 — false 를 실어 보내면 서버 기본값을
	// 덮어쓸 수 있고, 그 기본값이 무엇인지 스펙에 없다.
	if commonShareOnly {
		q.Set("commonShare", strconv.FormatBool(true))
	}

	var raw []apiListedStock
	if err := c.get(ctx, "/api/v1/stocks/all", q, &raw); err != nil {
		return domain.StockUniverse{}, err
	}

	out := domain.StockUniverse{Market: m, Status: q.Get("status"), FetchedAt: time.Now().UTC()}
	for _, s := range raw {
		out.Stocks = append(out.Stocks, domain.ListedStock{
			Symbol:       s.Symbol,
			Name:         s.Name,
			ISINCode:     s.IsinCode,
			SecurityType: s.SecurityType,
			CommonShare:  s.IsCommonShare,
		})
	}
	return out, nil
}
