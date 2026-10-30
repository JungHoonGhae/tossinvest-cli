package domain

import "time"

// Official Open API sector taxonomy (GET /api/v1/sectors/*, spec 1.2.24).
//
// Distinct from the WTS TICS types (Sector/Sectors/SectorDetail): ids are
// strings here, the tree is flat with parent links, and every money field is a
// KRW-converted string the server may null out when it has no data. Nullable
// fields stay pointers so "0" and "not available" never collapse into one.

// SectorNode is one taxonomy entry. Depth 0 = 대분류, 1 = 중분류, 2 = 소분류.
type SectorNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Depth    int    `json:"depth"`
	ParentID string `json:"parent_id,omitempty"`
}

// SectorCatalog is the full taxonomy, server-sorted by depth then numeric id.
type SectorCatalog struct {
	Sectors   []SectorNode `json:"sectors"`
	FetchedAt time.Time    `json:"fetched_at"`
}

// SectorReturn is one period's change rate as a decimal ratio (0.0123 = 1.23%).
// ChangeRate is nil when the market has no constituents for that period.
type SectorReturn struct {
	Duration   string   `json:"duration"`
	ChangeRate *float64 `json:"change_rate"`
}

// SectorProfile is one sector's identity, summary, and per-period returns for
// one market. Returns always holds the six durations in 1d → 1y order.
type SectorProfile struct {
	SectorNode
	Summary   string         `json:"summary,omitempty"`
	Market    string         `json:"market"`
	Returns   []SectorReturn `json:"returns"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// SectorRankingEntry is one ranked sector. ChangeRate is a decimal ratio and
// always present; TradingAmount and MarketCap are KRW and nil without data.
type SectorRankingEntry struct {
	Rank          int      `json:"rank"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ChangeRate    *float64 `json:"change_rate"`
	TradingAmount *float64 `json:"trading_amount"`
	MarketCap     *float64 `json:"market_cap"`
	Currency      string   `json:"currency"`
}

// SectorRankings is the top-100 ranking for one type/market/duration. Only
// depth 0–1 sectors are ranked; an empty Items is a valid "no data" answer.
type SectorRankings struct {
	Type      string               `json:"type"`
	Market    string               `json:"market"`
	Duration  string               `json:"duration"`
	Items     []SectorRankingEntry `json:"items"`
	FetchedAt time.Time            `json:"fetched_at"`
}

// SectorConstituent is one common stock in a sector (sub-sectors included),
// server-sorted by market cap descending. Volume/TradingAmount are today's
// running totals and nil for symbols without a quote.
type SectorConstituent struct {
	Symbol        string   `json:"symbol"`
	Name          string   `json:"name"`
	MarketCap     *float64 `json:"market_cap"`
	TradingAmount *float64 `json:"trading_amount"`
	Volume        *float64 `json:"volume"`
	Currency      string   `json:"currency"`
}

// SectorConstituents lists one sector's stocks for one market.
type SectorConstituents struct {
	SectorID  string              `json:"sector_id"`
	Market    string              `json:"market"`
	Stocks    []SectorConstituent `json:"stocks"`
	FetchedAt time.Time           `json:"fetched_at"`
}

// SectorETFEntry is one ETF classified under a sector (symbol and name only).
type SectorETFEntry struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

// SectorETFs lists one sector's ETFs (sub-sectors excluded) for one market.
type SectorETFs struct {
	SectorID  string           `json:"sector_id"`
	Market    string           `json:"market"`
	ETFs      []SectorETFEntry `json:"etfs"`
	FetchedAt time.Time        `json:"fetched_at"`
}
