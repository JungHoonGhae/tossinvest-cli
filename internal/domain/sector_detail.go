package domain

import "time"

// SectorPrice is the native and KRW-normalized price payload used by TICS
// constituents. Nullable fields remain pointers because some US ETF values do
// not have a KRW counterpart in the upstream response.
type SectorPrice struct {
	Base      *float64 `json:"base,omitempty"`
	BaseKRW   *float64 `json:"base_krw,omitempty"`
	Close     *float64 `json:"close,omitempty"`
	CloseKRW  *float64 `json:"close_krw,omitempty"`
	PriceType string   `json:"price_type,omitempty"`
}

type SectorStock struct {
	Rank            int         `json:"rank"`
	ProductCode     string      `json:"product_code"`
	Name            string      `json:"name"`
	LogoImageURL    string      `json:"logo_image_url,omitempty"`
	AnalystOpinion  string      `json:"analyst_opinion,omitempty"`
	ChangeRate      float64     `json:"change_rate"`
	MarketCapKRW    float64     `json:"market_cap_krw"`
	MarketCapUSD    float64     `json:"market_cap_usd"`
	TradingValueKRW float64     `json:"trading_value_krw"`
	TradingValueUSD float64     `json:"trading_value_usd"`
	Volume          float64     `json:"volume"`
	Price           SectorPrice `json:"price"`
}

type SectorTopHolding struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type SectorETF struct {
	Rank            int               `json:"rank"`
	ProductCode     string            `json:"product_code"`
	Symbol          string            `json:"symbol"`
	Name            string            `json:"name"`
	DetailName      string            `json:"detail_name,omitempty"`
	LogoImageURL    string            `json:"logo_image_url,omitempty"`
	ChangeRate      float64           `json:"change_rate"`
	ExpenseRatio    float64           `json:"expense_ratio"`
	LeverageFactor  float64           `json:"leverage_factor"`
	TopHolding      *SectorTopHolding `json:"top_holding,omitempty"`
	TradingValueKRW float64           `json:"trading_value_krw"`
	TradingValueUSD float64           `json:"trading_value_usd"`
	Price           SectorPrice       `json:"price"`
}

type SectorNews struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary,omitempty"`
	Source    string   `json:"source,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	ImageURLs []string `json:"image_urls,omitempty"`
}

// SectorDetail combines the independently served TICS overview, constituent
// stocks, related ETFs, and news into one stable CLI/MCP response.
type SectorDetail struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Summary      string `json:"summary"`
	Description  string `json:"description"`
	Depth        int    `json:"depth"`
	CompanyCount int    `json:"company_count"`
	ETFCount     int    `json:"etf_count"`
	// Total counts can exceed the arrays when WTS returns its default first page.
	StockTotalCount int           `json:"stock_total_count"`
	ETFTotalCount   int           `json:"etf_total_count"`
	NewsTotalCount  int           `json:"news_total_count"`
	Stocks          []SectorStock `json:"stocks"`
	ETFs            []SectorETF   `json:"etfs"`
	News            []SectorNews  `json:"news"`
	FetchedAt       time.Time     `json:"fetched_at"`
}
