package domain

import (
	"encoding/json"
	"time"
)

type Account struct {
	ID          string   `json:"id"`
	DisplayName string   `json:"display_name"`
	Name        string   `json:"name,omitempty"`
	Type        string   `json:"type,omitempty"`
	Currency    string   `json:"currency,omitempty"`
	Markets     []string `json:"markets,omitempty"`
	Primary     bool     `json:"primary,omitempty"`
}

type AccountSummary struct {
	TotalAssetAmount      float64                         `json:"total_asset_amount"`
	EvaluatedProfitAmount float64                         `json:"evaluated_profit_amount"`
	ProfitRate            float64                         `json:"profit_rate"`
	OrderableAmountKRW    float64                         `json:"orderable_amount_krw"`
	OrderableAmountUSD    float64                         `json:"orderable_amount_usd"`
	WithdrawableKR        map[string]any                  `json:"withdrawable_kr,omitempty"`
	WithdrawableUS        map[string]any                  `json:"withdrawable_us,omitempty"`
	Markets               map[string]AccountMarketSummary `json:"markets,omitempty"`
}

type AccountMarketSummary struct {
	Market                string  `json:"market"`
	PendingBuyOrderAmount float64 `json:"pending_buy_order_amount"`
	EvaluatedAmount       float64 `json:"evaluated_amount"`
	PrincipalAmount       float64 `json:"principal_amount"`
	EvaluatedProfitAmount float64 `json:"evaluated_profit_amount"`
	ProfitRate            float64 `json:"profit_rate"`
	TotalAssetAmount      float64 `json:"total_asset_amount"`
	OrderableAmountKRW    float64 `json:"orderable_amount_krw"`
	OrderableAmountUSD    float64 `json:"orderable_amount_usd"`
}

type Position struct {
	ProductCode     string  `json:"product_code,omitempty"`
	Symbol          string  `json:"symbol"`
	Name            string  `json:"name,omitempty"`
	MarketType      string  `json:"market_type,omitempty"`
	MarketCode      string  `json:"market_code,omitempty"`
	Quantity        float64 `json:"quantity"`
	AveragePrice    float64 `json:"average_price,omitempty"`
	CurrentPrice    float64 `json:"current_price,omitempty"`
	MarketValue     float64 `json:"market_value,omitempty"`
	UnrealizedPnL   float64 `json:"unrealized_pnl,omitempty"`
	ProfitRate      float64 `json:"profit_rate,omitempty"`
	DailyProfitLoss float64 `json:"daily_profit_loss,omitempty"`
	DailyProfitRate float64 `json:"daily_profit_rate,omitempty"`

	AveragePriceUSD    float64 `json:"average_price_usd,omitempty"`
	CurrentPriceUSD    float64 `json:"current_price_usd,omitempty"`
	MarketValueUSD     float64 `json:"market_value_usd,omitempty"`
	UnrealizedPnLUSD   float64 `json:"unrealized_pnl_usd,omitempty"`
	ProfitRateUSD      float64 `json:"profit_rate_usd,omitempty"`
	DailyProfitLossUSD float64 `json:"daily_profit_loss_usd,omitempty"`
	DailyProfitRateUSD float64 `json:"daily_profit_rate_usd,omitempty"`
}

type Order struct {
	ID                    string          `json:"id"`
	ResolvedFromID        string          `json:"resolved_from_id,omitempty"`
	Symbol                string          `json:"symbol"`
	Name                  string          `json:"name,omitempty"`
	Market                string          `json:"market,omitempty"`
	Side                  string          `json:"side,omitempty"`
	Status                string          `json:"status,omitempty"`
	Quantity              float64         `json:"quantity,omitempty"`
	FilledQuantity        float64         `json:"filled_quantity,omitempty"`
	Price                 float64         `json:"price,omitempty"`
	AverageExecutionPrice float64         `json:"average_execution_price,omitempty"`
	OrderDate             string          `json:"order_date,omitempty"`
	SubmittedAt           *time.Time      `json:"submitted_at,omitempty"`
	Raw                   json.RawMessage `json:"raw,omitempty"`
}

type WatchlistItem struct {
	Group    string  `json:"group,omitempty"`
	Symbol   string  `json:"symbol"`
	Name     string  `json:"name,omitempty"`
	Currency string  `json:"currency,omitempty"`
	Base     float64 `json:"base,omitempty"`
	Last     float64 `json:"last,omitempty"`
}

// Trade is a single executed tick (체결) from the market-data feed.
type Trade struct {
	Time             string  `json:"time"`
	Price            float64 `json:"price"`
	Base             float64 `json:"base,omitempty"`
	Volume           float64 `json:"volume"`
	TradeType        string  `json:"trade_type,omitempty"` // BUY / SELL
	CumulativeVolume float64 `json:"cumulative_volume,omitempty"`
}

type TradeList struct {
	ProductCode string    `json:"product_code"`
	Symbol      string    `json:"symbol,omitempty"`
	Name        string    `json:"name,omitempty"`
	Trades      []Trade   `json:"trades"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// PriceLimits is the daily upper/lower price band (상/하한가).
type PriceLimits struct {
	ProductCode string  `json:"product_code"`
	Symbol      string  `json:"symbol,omitempty"`
	Name        string  `json:"name,omitempty"`
	Date        string  `json:"date,omitempty"`
	UpperLimit  float64 `json:"upper_limit"`
	LowerLimit  float64 `json:"lower_limit"`
}

// StockWarning is a buy-caution badge (매수 유의사항). The web feed's badge
// shape is dynamic, so non-core fields are preserved as raw JSON.
type StockWarning struct {
	Type  string          `json:"type,omitempty"`
	Title string          `json:"title,omitempty"`
	Text  string          `json:"text,omitempty"`
	Level string          `json:"level,omitempty"`
	Raw   json.RawMessage `json:"raw,omitempty"`
}

type StockWarnings struct {
	ProductCode string         `json:"product_code"`
	Symbol      string         `json:"symbol,omitempty"`
	Name        string         `json:"name,omitempty"`
	Warnings    []StockWarning `json:"warnings"`
	FetchedAt   time.Time      `json:"fetched_at"`
}

// ExchangeRate is one FX/index quote (e.g. USD/KRW, DXY).
type ExchangeRate struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Base  float64 `json:"base,omitempty"`
	Close float64 `json:"close"`
}

type ExchangeRates struct {
	Rates     []ExchangeRate `json:"rates"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// ScreenerPreset is a predefined stock screen (조건검색 프리셋: 가치주·배당주 등).
// 공식 API 에 없는 web 전용 표면.
type ScreenerPreset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ScreenerPresets struct {
	Presets   []ScreenerPreset `json:"presets"`
	FetchedAt time.Time        `json:"fetched_at"`
}

// ScreenedStock is one stock matching a screener run.
type ScreenedStock struct {
	ProductCode string  `json:"product_code"`
	Name        string  `json:"name"`
	Close       float64 `json:"close"`
	Change      float64 `json:"change,omitempty"`
	ChangeRate  float64 `json:"change_rate,omitempty"`
}

type ScreenerResult struct {
	PresetID   string          `json:"preset_id"`
	PresetName string          `json:"preset_name,omitempty"`
	Nation     string          `json:"nation"`
	TotalCount int             `json:"total_count"`
	Stocks     []ScreenedStock `json:"stocks"`
	FetchedAt  time.Time       `json:"fetched_at"`
}

// AISignal is one entry of Toss's AI market signal feed (토스증권 AI 시그널).
// 공식 API 에 없는 web 전용 표면 — hero(AI 연결)와 정합.
type AISignal struct {
	AssetName   string `json:"asset_name"`
	AssetType   string `json:"asset_type,omitempty"`
	Title       string `json:"title"`
	Keyword     string `json:"keyword,omitempty"`
	Fluctuation string `json:"fluctuation,omitempty"`
	StockCode   string `json:"stock_code,omitempty"`
}

type AISignals struct {
	Label     string     `json:"label,omitempty"`
	Signals   []AISignal `json:"signals"`
	FetchedAt time.Time  `json:"fetched_at"`
}

// TradingFlow is one day's investor-type net flow (수급 — 개인·외국인·기관 순매수).
// KRX 전용 · 공식 API 에 없는 web 전용 표면.
type TradingFlow struct {
	Date           string  `json:"date"`
	NetIndividuals float64 `json:"net_individuals"` // 개인 순매수 (주)
	NetForeigner   float64 `json:"net_foreigner"`   // 외국인 순매수
	NetInstitution float64 `json:"net_institution"` // 기관 순매수
}

type TradingFlows struct {
	ProductCode string        `json:"product_code"`
	Symbol      string        `json:"symbol,omitempty"`
	Name        string        `json:"name,omitempty"`
	Flows       []TradingFlow `json:"flows"`
	FetchedAt   time.Time     `json:"fetched_at"`
}

// MarketIndex is one market index quote (코스피·나스닥·VIX 등). 공식 API 에 없는
// 표면 — web 전용 해자.
type MarketIndex struct {
	Code       string  `json:"code,omitempty"`
	Name       string  `json:"name"`
	Nation     string  `json:"nation,omitempty"` // kr | us
	Latest     float64 `json:"latest"`
	Base       float64 `json:"base,omitempty"`
	Change     float64 `json:"change,omitempty"`
	ChangeRate float64 `json:"change_rate,omitempty"`
}

type MarketIndices struct {
	Indices   []MarketIndex `json:"indices"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// IndexQuote is a detailed quote for a single market index (지수 상세 시세).
type IndexQuote struct {
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Nation     string    `json:"nation,omitempty"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	Base       float64   `json:"base"`
	Change     float64   `json:"change"`
	ChangeRate float64   `json:"change_rate"`
	Volume     float64   `json:"volume,omitempty"`
	High52w    float64   `json:"high_52w,omitempty"`
	Low52w     float64   `json:"low_52w,omitempty"`
	FetchedAt  time.Time `json:"fetched_at"`
}

// RankedStock is one entry in the realtime popularity ranking (실시간 인기 순위).
// 공식 API 에 없는 discovery 표면 — web 전용 해자.
type RankedStock struct {
	Rank        int    `json:"rank"`
	ProductCode string `json:"product_code"`
	Symbol      string `json:"symbol,omitempty"`
	Name        string `json:"name,omitempty"`
	Market      string `json:"market,omitempty"`
}

type StockRanking struct {
	Stocks    []RankedStock `json:"stocks"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// MarketSession is one trading day's session times for a market.
type MarketSession struct {
	Date      string `json:"date,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
}

// TradingHours holds today's (and next business day's) KR and US session
// windows (장 운영 시간). NextKR/NextUS are useful when today is a holiday.
type TradingHours struct {
	KR        MarketSession `json:"kr"`
	US        MarketSession `json:"us"`
	NextKR    MarketSession `json:"next_kr,omitempty"`
	NextUS    MarketSession `json:"next_us,omitempty"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// WatchlistGroup is a watchlist folder (관심종목 폴더) with its items.
// 공식 API 에 없는 web 전용 표면 — 읽기 + 쓰기(폴더 CRUD, 종목 add/remove).
type WatchlistGroup struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type,omitempty"` // USER_MADE 등
	ItemCount int             `json:"item_count"`
	Items     []WatchlistItem `json:"items,omitempty"`
}

type Transaction struct {
	Type             string          `json:"type"`
	Category         string          `json:"category"`
	Code             string          `json:"code,omitempty"`
	DisplayName      string          `json:"display_name,omitempty"`
	DisplayType      string          `json:"display_type,omitempty"`
	Summary          string          `json:"summary,omitempty"`
	Market           string          `json:"market"`
	Currency         string          `json:"currency"`
	StockCode        string          `json:"stock_code,omitempty"`
	StockName        string          `json:"stock_name,omitempty"`
	Quantity         float64         `json:"quantity,omitempty"`
	Amount           float64         `json:"amount"`
	AdjustedAmount   float64         `json:"adjusted_amount"`
	CommissionAmount float64         `json:"commission_amount,omitempty"`
	TaxAmount        float64         `json:"tax_amount,omitempty"`
	BalanceAmount    float64         `json:"balance_amount,omitempty"`
	Date             string          `json:"date,omitempty"`
	DateTime         string          `json:"datetime,omitempty"`
	OrderDate        string          `json:"order_date,omitempty"`
	SettlementDate   string          `json:"settlement_date,omitempty"`
	TradeType        string          `json:"trade_type,omitempty"`
	ReferenceType    string          `json:"reference_type,omitempty"`
	ReferenceID      string          `json:"reference_id,omitempty"`
	SortKey          string          `json:"sort_key,omitempty"`
	Raw              json.RawMessage `json:"raw,omitempty"`
}

type TransactionPage struct {
	Market   string        `json:"market"`
	Items    []Transaction `json:"items"`
	LastPage bool          `json:"last_page"`
	Next     *PagingParam  `json:"next,omitempty"`
}

type PagingParam struct {
	Number  int    `json:"number,omitempty"`
	Size    int    `json:"size,omitempty"`
	Key     string `json:"key,omitempty"`
	Filters string `json:"filters,omitempty"`
	Type    string `json:"type,omitempty"`
}

type TransactionOverview struct {
	Market                  string                         `json:"market"`
	OrderableKRW            float64                        `json:"orderable_krw"`
	OrderableUSD            float64                        `json:"orderable_usd"`
	Withdrawable            []SettlementBucket             `json:"withdrawable,omitempty"`
	DisplayWithdrawable     []SettlementBucket             `json:"display_withdrawable,omitempty"`
	Deposit                 []SettlementBucket             `json:"deposit,omitempty"`
	EstimateSettlement      []SettlementEstimate           `json:"estimate_settlement,omitempty"`
	WithdrawableBottomSheet []WithdrawableBottomSheetEntry `json:"withdrawable_bottom_sheet,omitempty"`
}

type SettlementBucket struct {
	Date string  `json:"date,omitempty"`
	KRW  float64 `json:"krw,omitempty"`
	USD  float64 `json:"usd,omitempty"`
}

type SettlementEstimate struct {
	Date       string  `json:"date,omitempty"`
	BuyAmount  float64 `json:"buy_amount,omitempty"`
	SellAmount float64 `json:"sell_amount,omitempty"`
}

type WithdrawableBottomSheetEntry struct {
	Title string  `json:"title"`
	KRW   float64 `json:"krw,omitempty"`
	USD   float64 `json:"usd,omitempty"`
}

type Candle struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume float64   `json:"volume,omitempty"`
}

type Chart struct {
	ProductCode string    `json:"product_code"`
	Symbol      string    `json:"symbol,omitempty"`
	Name        string    `json:"name,omitempty"`
	Interval    string    `json:"interval"`
	Base        float64   `json:"base,omitempty"`
	Candles     []Candle  `json:"candles"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type Quote struct {
	ProductCode     string    `json:"product_code,omitempty"`
	Symbol          string    `json:"symbol"`
	Name            string    `json:"name,omitempty"`
	MarketCode      string    `json:"market_code,omitempty"`
	Market          string    `json:"market,omitempty"`
	Currency        string    `json:"currency,omitempty"`
	ReferencePrice  float64   `json:"reference_price,omitempty"`
	Last            float64   `json:"last,omitempty"`
	Change          float64   `json:"change,omitempty"`
	ChangeRate      float64   `json:"change_rate,omitempty"`
	Volume          float64   `json:"volume,omitempty"`
	Open            float64   `json:"open,omitempty"`
	High            float64   `json:"high,omitempty"`
	Low             float64   `json:"low,omitempty"`
	High52w         float64   `json:"high_52w,omitempty"`
	Low52w          float64   `json:"low_52w,omitempty"`
	MarketCap       float64   `json:"market_cap,omitempty"`
	TradingValue    float64   `json:"trading_value,omitempty"`    // 거래대금
	TradingStrength float64   `json:"trading_strength,omitempty"` // 체결강도 (%)
	PrevVolume      float64   `json:"prev_volume,omitempty"`
	UpperLimit      float64   `json:"upper_limit,omitempty"`
	LowerLimit      float64   `json:"lower_limit,omitempty"`
	Status          string    `json:"status,omitempty"`
	BadgeCount      int       `json:"badge_count,omitempty"`
	NoticeCount     int       `json:"notice_count,omitempty"`
	FetchedAt       time.Time `json:"fetched_at"`
}

// OrderBookLevel is a single price level (호가) with its resting volume.
type OrderBookLevel struct {
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

// OrderBook is the bid/ask depth ladder (호가) for a symbol. Offers are ask
// (매도) levels, Bids are bid (매수) levels, ordered best-first.
type OrderBook struct {
	ProductCode string           `json:"product_code"`
	Symbol      string           `json:"symbol"`
	Name        string           `json:"name"`
	Close       float64          `json:"close"`
	Offers      []OrderBookLevel `json:"offers"`
	Bids        []OrderBookLevel `json:"bids"`
	TotalOffer  float64          `json:"total_offer_volume"`
	TotalBid    float64          `json:"total_bid_volume"`
	FetchedAt   time.Time        `json:"fetched_at"`
}

// SellableQuantity is how many shares of a held symbol can be sold now.
type SellableQuantity struct {
	ProductCode string    `json:"product_code"`
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Quantity    float64   `json:"sellable_quantity"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// Commission is the commission/tax rate schedule applied to a symbol's trades.
type Commission struct {
	ProductCode    string    `json:"product_code"`
	Symbol         string    `json:"symbol"`
	Name           string    `json:"name"`
	CommissionRate float64   `json:"commission_rate"`
	TaxRate        float64   `json:"tax_rate"`
	FetchedAt      time.Time `json:"fetched_at"`
}

// InvestorRankedStock is a top net-buy stock for an investor type.
type InvestorRankedStock struct {
	Rank         int     `json:"rank"`
	ProductCode  string  `json:"product_code"`
	Name         string  `json:"name"`
	NetBuyAmount float64 `json:"net_buy_amount"`
	Base         float64 `json:"base"`
	Close        float64 `json:"close"`
}

// InvestorRanking is one investor type's net-buy ranking (외국인/개인/기관).
type InvestorRanking struct {
	InvestorType string                `json:"investor_type"`
	BasedAt      string                `json:"based_at"`
	Stocks       []InvestorRankedStock `json:"stocks"`
}

// InvestorRankings is the market-wide net-buy ranking by investor type.
type InvestorRankings struct {
	Rankings  []InvestorRanking `json:"rankings"`
	FetchedAt time.Time         `json:"fetched_at"`
}

// EarningCall is a single upcoming earnings-call event.
type EarningCall struct {
	EventID     int64  `json:"event_id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	LiveAt      string `json:"live_at"`
	CompanyCode string `json:"company_code"`
	CompanyName string `json:"company_name"`
	Category    string `json:"category,omitempty"`
}

// EarningCalls is the upcoming earnings-call calendar.
type EarningCalls struct {
	Events    []EarningCall `json:"events"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// DividendAmount is a dividend value in both KRW and USD.
type DividendAmount struct {
	KRW float64 `json:"krw"`
	USD float64 `json:"usd"`
}

// DividendSummary is a total/paid/estimated dividend breakdown. Tax and
// Commission are only populated for the payment-date view.
type DividendSummary struct {
	Total      DividendAmount  `json:"total"`
	Paid       DividendAmount  `json:"paid"`
	Estimated  DividendAmount  `json:"estimated"`
	Tax        *DividendAmount `json:"tax,omitempty"`
	Commission *DividendAmount `json:"commission,omitempty"`
}

// DividendRegion is a per-market (kr/us) dividend summary.
type DividendRegion struct {
	Region  string          `json:"region"`
	Summary DividendSummary `json:"summary"`
}

// DividendStock is a single holding's dividend within a month.
type DividendStock struct {
	ProductCode string         `json:"product_code"`
	Name        string         `json:"name"`
	Quantity    float64        `json:"quantity"`
	Amount      DividendAmount `json:"amount"`
}

// DividendMonth is one month's dividend schedule.
type DividendMonth struct {
	Month   int             `json:"month"`
	Summary DividendSummary `json:"summary"`
	Stocks  []DividendStock `json:"stocks,omitempty"`
}

// Dividends is an annual dividend report for an account.
type Dividends struct {
	Year          int              `json:"year"`
	ByPaymentDate bool             `json:"by_payment_date"`
	Summary       DividendSummary  `json:"summary"`
	Regions       []DividendRegion `json:"regions"`
	Monthly       []DividendMonth  `json:"monthly"`
	FetchedAt     time.Time        `json:"fetched_at"`
}

// LendingExpectedStock is one holding's projected share-lending income.
type LendingExpectedStock struct {
	ProductCode string  `json:"product_code"`
	Name        string  `json:"name"`
	AmountUSD   float64 `json:"amount_usd"`
}

// LendingExpected is the projected share-lending (대주) income for an account:
// monthly/yearly totals in USD plus a per-stock breakdown. Works even for
// accounts without an active lending agreement (returns zeros).
type LendingExpected struct {
	OneMonthUSD float64                `json:"one_month_usd"`
	OneYearUSD  float64                `json:"one_year_usd"`
	Stocks      []LendingExpectedStock `json:"stocks"`
	FetchedAt   time.Time              `json:"fetched_at"`
}

// CommunityUser is one ranked community profile. Fields vary by ranking type:
// Description for influencers, Profit* for return rankings, Following* for
// fastest-growing rankings.
type CommunityUser struct {
	Rank              int     `json:"rank"`
	Nickname          string  `json:"nickname"`
	UserProfileID     int64   `json:"user_profile_id"`
	Description       string  `json:"description,omitempty"`
	ProfitAmountKRW   float64 `json:"profit_amount_krw,omitempty"`
	ProfitRate        float64 `json:"profit_rate,omitempty"`
	FollowingCount    int     `json:"following_count,omitempty"`
	FollowingIncrease int     `json:"following_increase,omitempty"`
}

// CommunityRanking is a community leaderboard of one type.
type CommunityRanking struct {
	Type      string          `json:"type"`
	Users     []CommunityUser `json:"users"`
	FetchedAt time.Time       `json:"fetched_at"`
}

// BriefingNews is a single news headline backing a briefing theme.
type BriefingNews struct {
	Title     string `json:"title"`
	Agency    string `json:"agency"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

// BriefingItem is one themed briefing (수급 변동·실적 등) with its headlines.
type BriefingItem struct {
	CategoryType string         `json:"category_type"`
	Keywords     []string       `json:"keywords"`
	News         []BriefingNews `json:"news"`
}

// NewsBriefing is the personalized AI news briefing grouped by theme.
type NewsBriefing struct {
	CreatedAt string         `json:"created_at"`
	Items     []BriefingItem `json:"items"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// Sector is one industry (TICS) with its fluctuation rates and sub-industries.
type Sector struct {
	ID             int      `json:"id"`
	Title          string   `json:"title"`
	CompanyCount   int      `json:"company_count"`
	OneDayRate     float64  `json:"one_day_rate"`
	OneMonthRate   float64  `json:"one_month_rate"`
	ThreeMonthRate float64  `json:"three_month_rate"`
	OneYearRate    float64  `json:"one_year_rate"`
	SubSectors     []Sector `json:"sub_sectors,omitempty"`
}

// Sectors is the industry (TICS) tree with fluctuation rates.
type Sectors struct {
	Items     []Sector  `json:"items"`
	FetchedAt time.Time `json:"fetched_at"`
}

// ThemeRanking is one TICS theme/sector ranked by today's fluctuation.
type ThemeRanking struct {
	Ranking          int     `json:"ranking"`
	TicsID           string  `json:"tics_id"`
	Title            string  `json:"title"`
	ChangeRate       float64 `json:"change_rate"` // daily % change (e.g. 11.18 or -3.2)
	RiseCompanyCount int     `json:"rise_company_count"`
	TotalCount       int     `json:"total_count"`
	Summary          string  `json:"summary"` // e.g. "21개 중 13개 종목 상승"
}

// ThemeRankings is the TICS theme fluctuation ranking (오늘의 테마 등락 순위),
// sorted by ranking (1 = biggest gainer). Not in the official Open API.
type ThemeRankings struct {
	Name      string         `json:"name"`
	DateTime  string         `json:"date_time"`
	Items     []ThemeRanking `json:"items"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// BuyingPower is the cash-based buying power for a given currency.
// Endpoint: GET /api/v1/buying-power (official API).
// cashBuyingPower (string decimal) is parsed to float64.
// CashBuyingPower maps to WTS OrderableAmountKr.KRW or OrderableAmountUs.USD.
type BuyingPower struct {
	Currency        string  `json:"currency"`
	CashBuyingPower float64 `json:"cash_buying_power"`
}

// PrimeExchangeFee is a foreign-exchange fee comparison across three tiers:
// non-member, Prime member (list price), and the actual benefit-adjusted fee.
type PrimeExchangeFee struct {
	NonPrimeFee int `json:"non_prime_fee"`
	PrimeFee    int `json:"prime_fee"`
	BenefitFee  int `json:"benefit_fee"`
}

// PrimeInterestTier is a cash-balance interest-rate comparison (KRW or USD)
// across the same three tiers as PrimeExchangeFee.
type PrimeInterestTier struct {
	Status           string `json:"status"`
	NonPrimeInterest int    `json:"non_prime_interest"`
	PrimeInterest    int    `json:"prime_interest"`
	BenefitInterest  int    `json:"benefit_interest"`
}

// PrimeStatus is the account's Toss Prime membership state plus this month's
// fee/interest benefit comparison. Not in the official Open API — 공식
// Open API 에 없는 tossctl 고유 기능.
type PrimeStatus struct {
	IsMember        bool              `json:"is_member"`
	UserID          *string           `json:"user_id,omitempty"`
	PrimeType       *string           `json:"prime_type,omitempty"`
	BenefitsStartAt *string           `json:"benefits_start_at,omitempty"`
	BenefitsEndAt   *string           `json:"benefits_end_at,omitempty"`
	CycleNumber     *int              `json:"cycle_number,omitempty"`
	Month           string            `json:"month"`
	Exchange        PrimeExchangeFee  `json:"exchange"`
	InterestKRW     PrimeInterestTier `json:"interest_krw"`
	InterestUSD     PrimeInterestTier `json:"interest_usd"`
	BaseRate        float64           `json:"base_rate"`
	MonthlyTotalKRW float64           `json:"monthly_total_krw"`
	FetchedAt       time.Time         `json:"fetched_at"`
}

// RankingItem is one row of the official /rankings response.
type RankingItem struct {
	Rank          int     `json:"rank"`
	Symbol        string  `json:"symbol"`
	Currency      string  `json:"currency"`
	LastPrice     float64 `json:"last_price"`
	BasePrice     float64 `json:"base_price"`
	ChangeRate    float64 `json:"change_rate"`
	TradingVolume float64 `json:"trading_volume"`
	TradingAmount float64 `json:"trading_amount"`
}

// Ranking is the official stock ranking (거래대금/거래량/등락률 상위).
// Source: GET /api/v1/rankings (official Open API, key required).
type Ranking struct {
	Type          string        `json:"type"`
	MarketCountry string        `json:"market_country"`
	Duration      string        `json:"duration"`
	RankedAt      string        `json:"ranked_at,omitempty"`
	Items         []RankingItem `json:"items"`
	FetchedAt     time.Time     `json:"fetched_at"`
}

// MarketIndicatorPrice is one indicator's current price.
// Source: GET /api/v1/market-indicators/prices (official Open API, key required).
type MarketIndicatorPrice struct {
	Symbol    string  `json:"symbol"`
	LastPrice float64 `json:"last_price"`
	Timestamp string  `json:"timestamp,omitempty"`
}

// MarketIndicatorPrices is a batch of indicator current prices.
type MarketIndicatorPrices struct {
	Indicators []MarketIndicatorPrice `json:"indicators"`
	FetchedAt  time.Time              `json:"fetched_at"`
}

// MarketIndicatorCandle is one OHLCV candle for a market indicator.
type MarketIndicatorCandle struct {
	Timestamp string  `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// MarketIndicatorCandles is a page of candles for one indicator symbol.
// Source: GET /api/v1/market-indicators/{symbol}/candles (official Open API, key required).
type MarketIndicatorCandles struct {
	Symbol     string                  `json:"symbol"`
	Interval   string                  `json:"interval"`
	Candles    []MarketIndicatorCandle `json:"candles"`
	NextBefore string                  `json:"next_before,omitempty"`
	FetchedAt  time.Time               `json:"fetched_at"`
}

// InvestorTradingParty is one investor group's buy/sell/net amounts (KRW).
type InvestorTradingParty struct {
	BuyAmount  float64 `json:"buy_amount"`
	SellAmount float64 `json:"sell_amount"`
	NetAmount  float64 `json:"net_amount"`
}

// InvestorTradingRecord is one period's market-wide investor trading.
type InvestorTradingRecord struct {
	Date             string               `json:"date"`
	UpdatedAt        string               `json:"updated_at"`
	Individual       InvestorTradingParty `json:"individual"`
	Foreigner        InvestorTradingParty `json:"foreigner"`
	Institution      InvestorTradingParty `json:"institution"`
	OtherCorporation InvestorTradingParty `json:"other_corporation"`
}

// InvestorTrading is market-wide (KOSPI/KOSDAQ) investor trading over time.
// Source: GET /api/v1/market-indicators/{symbol}/investor-trading (official Open API, key required).
type InvestorTrading struct {
	Symbol    string                  `json:"symbol"`
	Interval  string                  `json:"interval"`
	Records   []InvestorTradingRecord `json:"records"`
	NextUntil string                  `json:"next_until,omitempty"`
	FetchedAt time.Time               `json:"fetched_at"`
}

// ConditionalOrderCondition is one watch leg of a conditional order.
type ConditionalOrderCondition struct {
	Type             string  `json:"type"` // STOP | PROFIT_RATE
	Status           string  `json:"status"`
	TriggerPrice     float64 `json:"trigger_price"`
	TargetProfitRate float64 `json:"target_profit_rate"`
	OrderPrice       float64 `json:"order_price"`
	TriggeredOrderID string  `json:"triggered_order_id,omitempty"`
}

// ConditionalOrder is one conditional order (group of watch legs).
type ConditionalOrder struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"` // SINGLE | OCO | OTO
	Status     string                     `json:"status"`
	Symbol     string                     `json:"symbol"`
	Market     string                     `json:"market"`
	Quantity   float64                    `json:"quantity"`
	OrderType  string                     `json:"order_type"` // LIMIT | MARKET
	ExpireDate string                     `json:"expire_date"`
	First      ConditionalOrderCondition  `json:"first"`
	Second     *ConditionalOrderCondition `json:"second,omitempty"`
	CreatedAt  string                     `json:"created_at"`
}

// ConditionalOrderList is a page of conditional orders.
type ConditionalOrderList struct {
	Orders     []ConditionalOrder `json:"orders"`
	NextCursor string             `json:"next_cursor,omitempty"`
	HasNext    bool               `json:"has_next"`
	FetchedAt  time.Time          `json:"fetched_at"`
}

// ConditionalOrderRef is the id returned when creating a conditional order.
type ConditionalOrderRef struct {
	ID            string `json:"id"`
	ClientOrderID string `json:"client_order_id,omitempty"`
}

// AccumulationPlan is one "stock accumulation" (주식모으기) recurring-buy
// plan — a scheduled automatic purchase of a fixed amount or quantity.
// IsPaused is the plan's active/disabled state. WTS-only.
type AccumulationPlan struct {
	ID                    int64   `json:"id"`
	Symbol                string  `json:"symbol"`
	StockCode             string  `json:"stock_code"`
	StockName             string  `json:"stock_name"`
	CountryCode           string  `json:"country_code"`
	Currency              string  `json:"currency"`
	PlanType              string  `json:"plan_type"` // AMOUNT | QUANTITY
	Iteration             string  `json:"iteration"` // e.g. DAILY, WEEKLY, MONTHLY
	IterateTarget         int     `json:"iterate_target"`
	InvestAmount          float64 `json:"invest_amount"`
	InvestQuantity        float64 `json:"invest_quantity"`
	TradeStatus           string  `json:"trade_status"` // e.g. PROGRESS
	IsPaused              bool    `json:"is_paused"`
	InvestStartDate       string  `json:"invest_start_date"`
	InvestEndDate         string  `json:"invest_end_date"`
	ProceededRound        int     `json:"proceeded_round"`
	SucceededRound        int     `json:"succeeded_round"`
	TotalExecutedAmount   float64 `json:"total_executed_amount"`
	TotalExecutedQuantity float64 `json:"total_executed_quantity"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

// AccumulationPlans is the account's full list of accumulation plans.
type AccumulationPlans struct {
	Plans     []AccumulationPlan `json:"plans"`
	FetchedAt time.Time          `json:"fetched_at"`
}

// DualCurrency holds a KRW and USD amount for the same figure. USD is a
// pointer because some fields (e.g. account-interest) have no USD value.
type DualCurrency struct {
	KRW float64  `json:"krw"`
	USD *float64 `json:"usd,omitempty"`
}

// ProfitByType is one profit category's realized figures: the earned amount,
// its rate (%), and the purchase basis — each in both currencies.
type ProfitByType struct {
	Amount         DualCurrency `json:"amount"`
	EarningRate    DualCurrency `json:"earning_rate"`
	PurchaseAmount DualCurrency `json:"purchase_amount"`
}

// ProfitOverview is the account's cumulative profit breakdown across every
// category (매매손익·대여료·배당·만기·예탁금이자). This is a separate,
// cumulative view from `account summary` (which reports current valuation).
type ProfitOverview struct {
	TotalAssetAmount DualCurrency `json:"total_asset_amount"`
	EarningAmount    DualCurrency `json:"earning_amount"`
	Sales            ProfitByType `json:"sales"`    // 매매손익
	Lending          ProfitByType `json:"lending"`  // 대여료
	Dividend         ProfitByType `json:"dividend"` // 배당
	Maturity         ProfitByType `json:"maturity"` // 만기
	Interest         float64      `json:"interest"` // 예탁금이자 (KRW only)
	FetchedAt        time.Time    `json:"fetched_at"`
}

// PeriodProfit is realized profit for ONE profit type over a date range —
// the period-scoped counterpart to ProfitOverview (which is all-time and covers
// every type at once). Range is "all" when no dates were given.
type PeriodProfit struct {
	Type           string       `json:"type"` // sales | dividend | lending | account-interest
	From           string       `json:"from,omitempty"`
	To             string       `json:"to,omitempty"`
	EarningAmount  DualCurrency `json:"earning_amount"`
	EarningRate    DualCurrency `json:"earning_rate"`
	PurchaseAmount DualCurrency `json:"purchase_amount"`
	FetchedAt      time.Time    `json:"fetched_at"`
}

// DailyProfitStock is one stock's realized profit on one date, as reported by
// the daily market breakdown.
type DailyProfitStock struct {
	Date        string       `json:"date"` // YYYY-MM-DD
	MarketType  string       `json:"market_type"`
	Symbol      string       `json:"symbol"`
	Name        string       `json:"name"`
	ProductCode string       `json:"product_code"`
	Quantity    float64      `json:"quantity"`
	ProfitLoss  DualCurrency `json:"profit_loss"`
	ProfitRate  float64      `json:"profit_rate"`
	SellAmount  DualCurrency `json:"sell_amount"`
	BuyAmount   DualCurrency `json:"buy_amount"`
}

// DailyProfit is the per-stock realized-profit breakdown over a date range,
// aggregated across every page the API returns.
//
// Currency is the BASIS the rates were computed against, not a filter: the same
// rows come back either way, but ProfitRate for a foreign holding differs
// because the KRW basis folds in FX movement and the USD basis does not.
type DailyProfit struct {
	From      string             `json:"from,omitempty"`
	To        string             `json:"to,omitempty"`
	Currency  string             `json:"currency"`
	Stocks    []DailyProfitStock `json:"stocks"`
	FetchedAt time.Time          `json:"fetched_at"`
}

// TransferIncomeStock is one stock's overseas transfer-income (양도소득) line
// for a tax year: quantities, sell/buy amounts, and realized profit/loss.
type TransferIncomeStock struct {
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	StockCode      string  `json:"stock_code"`
	SellQuantity   float64 `json:"sell_quantity"`
	SellAmount     float64 `json:"sell_amount"`
	BuyAmount      float64 `json:"buy_amount"`
	Expense        float64 `json:"expense"`
	ProfitLoss     float64 `json:"profit_loss"`
	SettlementDate string  `json:"settlement_date"`
	Settled        bool    `json:"settled"`
}

// OverseasTransferIncome is the account's overseas-stock transfer-income (해외
// 주식 양도소득) report for a tax year: a tax summary plus per-stock lines.
// Used for capital-gains tax filing. All amounts are in KRW.
type OverseasTransferIncome struct {
	Year              int                   `json:"year"`
	TaxRate           float64               `json:"tax_rate"`
	LocalTaxRate      float64               `json:"local_tax_rate"`
	BaseDeduction     float64               `json:"base_deduction"`
	TotalProfitLoss   float64               `json:"total_profit_loss"`
	TransferIncomeTax float64               `json:"transfer_income_tax"`
	LocalIncomeTax    float64               `json:"local_income_tax"`
	TotalTax          float64               `json:"total_tax"`
	Stocks            []TransferIncomeStock `json:"stocks"`
	FetchedAt         time.Time             `json:"fetched_at"`
}
