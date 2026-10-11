package official

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Official sector taxonomy (spec 1.2.24, tag "Sector"). Token-only: no
// account header. All five share one marketCountry enum and one 404 code
// (sector-not-found), so the enum checks live here rather than per command.

var (
	sectorMarkets      = []string{"KR", "US"}
	sectorRankingTypes = []string{"TOP_GAINERS", "MARKET_TRADING_AMOUNT"}
	sectorDurations    = []string{"1d", "1w", "1mo", "3mo", "6mo", "1y"}
)

// SectorMarkets, SectorRankingTypes and SectorDurations expose the accepted
// enums for CLI help and MCP parameter descriptions.
func SectorMarkets() []string      { return append([]string(nil), sectorMarkets...) }
func SectorRankingTypes() []string { return append([]string(nil), sectorRankingTypes...) }
func SectorDurations() []string    { return append([]string(nil), sectorDurations...) }

func pickEnum(name, got string, allowed []string, fold func(string) string) (string, error) {
	v := fold(strings.TrimSpace(got))
	for _, a := range allowed {
		if a == v {
			return v, nil
		}
	}
	return "", fmt.Errorf("unknown %s %q (want one of: %s)", name, got, strings.Join(allowed, ", "))
}

func sectorMarketQuery(market string) (url.Values, string, error) {
	if strings.TrimSpace(market) == "" {
		market = "KR"
	}
	m, err := pickEnum("market", market, sectorMarkets, strings.ToUpper)
	if err != nil {
		return nil, "", err
	}
	q := url.Values{}
	q.Set("marketCountry", m)
	return q, m, nil
}

type apiSector struct {
	SectorID       string  `json:"sectorId"`
	Name           string  `json:"name"`
	Depth          int     `json:"depth"`
	ParentSectorID *string `json:"parentSectorId"`
}

func adaptSectorNode(s apiSector) domain.SectorNode {
	n := domain.SectorNode{ID: s.SectorID, Name: s.Name, Depth: s.Depth}
	if s.ParentSectorID != nil {
		n.ParentID = *s.ParentSectorID
	}
	return n
}

// Sectors returns the whole taxonomy. Low-churn: the spec recommends fetching
// once a day and caching.
func (c *Client) Sectors(ctx context.Context) (domain.SectorCatalog, error) {
	var raw struct {
		Sectors []apiSector `json:"sectors"`
	}
	if err := c.get(ctx, "/api/v1/sectors", nil, &raw); err != nil {
		return domain.SectorCatalog{}, err
	}
	out := domain.SectorCatalog{FetchedAt: time.Now().UTC()}
	for _, s := range raw.Sectors {
		out.Sectors = append(out.Sectors, adaptSectorNode(s))
	}
	return out, nil
}

type apiSectorChangeRate struct {
	Duration   string  `json:"duration"`
	ChangeRate *string `json:"changeRate"`
}

// Sector returns one sector's profile with per-period returns for market.
func (c *Client) Sector(ctx context.Context, id, market string) (domain.SectorProfile, error) {
	q, m, err := sectorMarketQuery(market)
	if err != nil {
		return domain.SectorProfile{}, err
	}
	var raw struct {
		apiSector
		Summary     *string `json:"summary"`
		Performance struct {
			ChangeRates []apiSectorChangeRate `json:"changeRates"`
		} `json:"performance"`
	}
	if err := c.get(ctx, "/api/v1/sectors/"+url.PathEscape(strings.TrimSpace(id)), q, &raw); err != nil {
		return domain.SectorProfile{}, err
	}
	out := domain.SectorProfile{SectorNode: adaptSectorNode(raw.apiSector), Market: m, FetchedAt: time.Now().UTC()}
	if raw.Summary != nil {
		out.Summary = *raw.Summary
	}
	for _, r := range raw.Performance.ChangeRates {
		out.Returns = append(out.Returns, domain.SectorReturn{Duration: r.Duration, ChangeRate: decimalPtr(r.ChangeRate)})
	}
	return out, nil
}

// SectorRankings returns the top-100 sectors for rankingType/market/duration.
func (c *Client) SectorRankings(ctx context.Context, rankingType, market, duration string) (domain.SectorRankings, error) {
	q, m, err := sectorMarketQuery(market)
	if err != nil {
		return domain.SectorRankings{}, err
	}
	t, err := pickEnum("ranking type", rankingType, sectorRankingTypes, strings.ToUpper)
	if err != nil {
		return domain.SectorRankings{}, err
	}
	if strings.TrimSpace(duration) == "" {
		duration = "1d"
	}
	d, err := pickEnum("duration", duration, sectorDurations, strings.ToLower)
	if err != nil {
		return domain.SectorRankings{}, err
	}
	q.Set("type", t)
	q.Set("duration", d)
	var raw struct {
		Rankings []struct {
			Rank                 int     `json:"rank"`
			SectorID             string  `json:"sectorId"`
			Name                 string  `json:"name"`
			ChangeRate           *string `json:"changeRate"`
			TradingAmount        *string `json:"tradingAmount"`
			MarketCapitalization *string `json:"marketCapitalization"`
			Currency             string  `json:"currency"`
		} `json:"rankings"`
	}
	if err := c.get(ctx, "/api/v1/sectors/rankings", q, &raw); err != nil {
		return domain.SectorRankings{}, err
	}
	out := domain.SectorRankings{Type: t, Market: m, Duration: d, FetchedAt: time.Now().UTC()}
	for _, r := range raw.Rankings {
		out.Items = append(out.Items, domain.SectorRankingEntry{
			Rank: r.Rank, ID: r.SectorID, Name: r.Name, Currency: r.Currency,
			ChangeRate:    decimalPtr(r.ChangeRate),
			TradingAmount: decimalPtr(r.TradingAmount),
			MarketCap:     decimalPtr(r.MarketCapitalization),
		})
	}
	return out, nil
}

// SectorStocks returns a sector's common stocks (sub-sectors included) for market.
func (c *Client) SectorStocks(ctx context.Context, id, market string) (domain.SectorConstituents, error) {
	q, m, err := sectorMarketQuery(market)
	if err != nil {
		return domain.SectorConstituents{}, err
	}
	id = strings.TrimSpace(id)
	var raw struct {
		Stocks []struct {
			Symbol               string  `json:"symbol"`
			Name                 string  `json:"name"`
			MarketCapitalization *string `json:"marketCapitalization"`
			TradingAmount        *string `json:"tradingAmount"`
			Volume               *string `json:"volume"`
			Currency             string  `json:"currency"`
		} `json:"stocks"`
	}
	if err := c.get(ctx, "/api/v1/sectors/"+url.PathEscape(id)+"/stocks", q, &raw); err != nil {
		return domain.SectorConstituents{}, err
	}
	out := domain.SectorConstituents{SectorID: id, Market: m, FetchedAt: time.Now().UTC()}
	for _, s := range raw.Stocks {
		out.Stocks = append(out.Stocks, domain.SectorConstituent{
			Symbol: s.Symbol, Name: s.Name, Currency: s.Currency,
			MarketCap:     decimalPtr(s.MarketCapitalization),
			TradingAmount: decimalPtr(s.TradingAmount),
			Volume:        decimalPtr(s.Volume),
		})
	}
	return out, nil
}

// SectorETFs returns a sector's ETFs (sub-sectors excluded) for market.
func (c *Client) SectorETFs(ctx context.Context, id, market string) (domain.SectorETFs, error) {
	q, m, err := sectorMarketQuery(market)
	if err != nil {
		return domain.SectorETFs{}, err
	}
	id = strings.TrimSpace(id)
	var raw struct {
		ETFs []struct {
			Symbol string `json:"symbol"`
			Name   string `json:"name"`
		} `json:"etfs"`
	}
	if err := c.get(ctx, "/api/v1/sectors/"+url.PathEscape(id)+"/etfs", q, &raw); err != nil {
		return domain.SectorETFs{}, err
	}
	out := domain.SectorETFs{SectorID: id, Market: m, FetchedAt: time.Now().UTC()}
	for _, e := range raw.ETFs {
		out.ETFs = append(out.ETFs, domain.SectorETFEntry{Symbol: e.Symbol, Name: e.Name})
	}
	return out, nil
}
