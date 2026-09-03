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

// AccountOverview is the mobile account switcher's all-account rollup. It is
// deliberately separate from AccountSummary: this view spans every account
// (including minor accounts), while AccountSummary describes the currently
// selected account in detail.
type AccountOverview struct {
	Accounts         []AccountOverviewItem `json:"accounts"`
	MinorAccounts    []AccountOverviewItem `json:"minor_accounts"`
	TotalAssetAmount int64                 `json:"total_asset_amount"`
}

type AccountOverviewItem struct {
	AccountName       string `json:"account_name"`
	AccountNo         string `json:"account_no"`
	PendingOrderCount int    `json:"pending_order_count"`
	TotalAssetAmount  int64  `json:"total_asset_amount"`
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
	// Filters is the preset's raw condition array, exactly as the screener API
	// accepts it. It is carried through so a caller can copy a preset, adjust a
	// threshold, and feed it back via `market screener --filter` — the filter
	// vocabulary itself lives in Toss's JS bundle (Korean ids) and is not
	// otherwise discoverable.
	Filters json.RawMessage `json:"filters,omitempty"`
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

// TradingSession is one named window within a business day (pre-market,
// regular, after-hours, and — US only — the day market).
//
// SinglePriceAuction* are KR-only: the 단일가 auction windows the KRX runs at
// the edges of a session. They stay empty for US days rather than being faked,
// so an empty value means "this market has no such auction", not "unknown".
type TradingSession struct {
	Name                    string `json:"name"`
	Start                   string `json:"start,omitempty"`
	End                     string `json:"end,omitempty"`
	SinglePriceAuctionStart string `json:"single_price_auction_start,omitempty"`
	SinglePriceAuctionEnd   string `json:"single_price_auction_end,omitempty"`
}

// BusinessDay is one calendar day and the sessions it runs.
//
// Holiday is what the two markets express differently — KR nulls the whole
// `integrated` object, US leaves every session empty — normalized to one flag
// so a caller does not have to know which market it asked about.
type BusinessDay struct {
	Date     string           `json:"date"`
	Holiday  bool             `json:"holiday"`
	Sessions []TradingSession `json:"sessions,omitempty"`
}

// TradingCalendar is the official API's previous/today/next business-day view
// for one market.
//
// Distinct from MarketCalendar, which is the WTS month of scheduled market
// *events* (earnings, holidays, economic releases). This one answers "when does
// trading actually open and close", and is the only path to session times for
// users who hold an official key but no web session.
type TradingCalendar struct {
	Country   string      `json:"country"`
	Previous  BusinessDay `json:"previous"`
	Today     BusinessDay `json:"today"`
	Next      BusinessDay `json:"next"`
	FetchedAt time.Time   `json:"fetched_at"`
}

// MarketHaltEvent is one market-wide trading interruption switch and whether it
// is firing right now. Toss exposes the same four facts twice — as a fixed
// `haltStatus` object with hardcoded kospi/kosdaq keys, and as this list. The
// list is the one that survives Toss adding a market or a halt type, so it is
// the only source read.
//
// Market is Toss's code ("KSP" 코스피, "KSQ" 코스닥); MarketName is the readable
// form. Type is normalized to a lowercase alias, falling back to the raw server
// string when unrecognized so a newly shipped halt type stays visible instead of
// silently disappearing.
type MarketHaltEvent struct {
	Market     string `json:"market"`
	MarketName string `json:"market_name"`
	Type       string `json:"type"`
	Activated  bool   `json:"activated"`
}

// MarketHalt is the current 서킷브레이커·사이드카 state across KR markets.
type MarketHalt struct {
	Events    []MarketHaltEvent `json:"events"`
	FetchedAt time.Time         `json:"fetched_at"`
}

// Halted reports whether any switch is currently firing.
func (m MarketHalt) Halted() bool {
	for _, e := range m.Events {
		if e.Activated {
			return true
		}
	}
	return false
}

// IndexAnomaly is one index Toss has badged as moving unusually, with the AI
// signal it attached. ZScore is how far the move sits from the index's own
// recent distribution — the number behind the badge.
//
// Category and Direction are passed through as the server's own strings: Toss
// does not publish these enums, and guessing a mapping would silently mislabel
// a value it adds later.
type IndexAnomaly struct {
	IndexCode   string  `json:"index_code"`
	DisplayName string  `json:"display_name"`
	Category    string  `json:"category"`
	Direction   string  `json:"direction"`
	IsAnomaly   bool    `json:"is_anomaly"`
	ChangeRate  float64 `json:"change_rate"`
	SignalTitle string  `json:"signal_title,omitempty"`
	SignalID    string  `json:"signal_id,omitempty"`
	Keyword     string  `json:"keyword,omitempty"`
	ZScore      float64 `json:"zscore"`
}

// IndexAnomalies is the badged-index set for the current session.
type IndexAnomalies struct {
	Indices   []IndexAnomaly `json:"indices"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// ChartBatch is the answer to one multi-symbol chart request: the charts that
// came back, plus the symbols the server had no data for.
//
// The two travel together because the server omits unknown codes instead of
// erroring — a caller handed only the charts cannot tell "this symbol has no
// data" from "I never asked for it". Keeping Missing in the same value means
// every output format carries it, rather than it existing only in whichever
// renderer remembered to print a warning.
type ChartBatch struct {
	Charts    []Chart              `json:"charts"`
	Missing   []string             `json:"missing,omitempty"`
	Sequence  []BatchSequenceEntry `json:"-"`
	FetchedAt time.Time            `json:"fetched_at"`
}

// BatchSequenceEntry retains one caller input position across backends that
// return found and missing items in separate shapes. It is output metadata,
// not part of the JSON contract; machine JSON uses the explicit items and
// missing fields while row-oriented formats use this to preserve interleaving.
type BatchSequenceEntry struct {
	Symbol  string
	Missing bool
}

// StockReason is one line of Toss's AI explanation for why a stock is moving.
// The batch endpoint returns only this sentence; GetStockReasoning fetches the
// full card (summary, direction, related stocks) for a single symbol.
//
// Symbol is echoed back from the request so a caller can align results with
// what it asked for — the server omits codes it has no reasoning for, so the
// response is shorter than the request and not positionally aligned.
type StockReason struct {
	Symbol      string `json:"symbol"`
	ProductCode string `json:"product_code"`
	Description string `json:"description"`
}

// StockReasons is the batch AI-reasoning result.
type StockReasons struct {
	Reasons   []StockReason        `json:"reasons"`
	Missing   []string             `json:"missing,omitempty"`
	Sequence  []BatchSequenceEntry `json:"-"`
	FetchedAt time.Time            `json:"fetched_at"`
}

// OptionSession is one US-options business day. PreMarket/AfterMarket are
// nil-able in the feed — options have no extended session the way equities do,
// so they stay empty rather than being faked to match the regular window.
type OptionSession struct {
	Date             string `json:"date"`
	Start            string `json:"start,omitempty"`
	End              string `json:"end,omitempty"`
	PreMarketStart   string `json:"pre_market_start,omitempty"`
	PreMarketEnd     string `json:"pre_market_end,omitempty"`
	AfterMarketStart string `json:"after_market_start,omitempty"`
	AfterMarketEnd   string `json:"after_market_end,omitempty"`
}

// OptionTradingHours is the US-options session window for the previous,
// current, and next business day. Distinct from TradingHours, which covers
// equities — the two can diverge around holidays.
type OptionTradingHours struct {
	Previous  OptionSession `json:"previous"`
	Today     OptionSession `json:"today"`
	Next      OptionSession `json:"next"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// OrderFunding answers "can I buy right now, and if not, how much do I need to
// deposit or exchange". Distinct from AccountSummary's orderable amounts and
// from BuyingPower (official API), both of which report what is already
// available rather than what is missing.
type OrderFunding struct {
	Buyable                bool      `json:"buyable"`
	ReceivableCurrency     string    `json:"receivable_currency,omitempty"`
	KRWAmount              float64   `json:"krw_amount"`
	USDAmount              float64   `json:"usd_amount"`
	USDReceivableKRWEquiv  float64   `json:"usd_receivable_krw_equivalent"`
	KRWWithdrawable        float64   `json:"krw_withdrawable"`
	RequiredDepositAmount  float64   `json:"required_deposit_amount"`
	RequiredExchangeAmount float64   `json:"required_exchange_amount"`
	FetchedAt              time.Time `json:"fetched_at"`
}

// CommunityBoard is one Toss community lounge (라운지). Rules is the board's
// posting-rule list as the server sends it.
type CommunityBoard struct {
	ID            int64    `json:"id"`
	SubjectType   string   `json:"subject_type,omitempty"`
	SubjectID     string   `json:"subject_id,omitempty"`
	Title         string   `json:"title"`
	About         string   `json:"about,omitempty"`
	Rules         []string `json:"rules,omitempty"`
	FollowerCount int      `json:"follower_count"`
	CommentCount  int      `json:"comment_count"`
	IsMember      bool     `json:"is_member"`
	IsManager     bool     `json:"is_manager"`
	CreatedAt     string   `json:"created_at,omitempty"`
}

type CommunityBoards struct {
	Boards    []CommunityBoard `json:"boards"`
	FetchedAt time.Time        `json:"fetched_at"`
}

// WatchlistGroup is a watchlist folder (관심종목 폴더) with its items.
// 공식 API 에 없는 web 전용 표면 — 읽기 + 쓰기(폴더 CRUD, 종목 add/remove).
type WatchlistGroup struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Ordering  int             `json:"ordering,omitempty"`
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

// KoreanMarketDetail contains KRX/NXT availability flags returned only for
// Korean stock metadata. NXTTradingSuspended is nil when NXT is unsupported.
type KoreanMarketDetail struct {
	LiquidationTrading  bool  `json:"liquidation_trading"`
	NXTSupported        bool  `json:"nxt_supported"`
	KRXTradingSuspended bool  `json:"krx_trading_suspended"`
	NXTTradingSuspended *bool `json:"nxt_trading_suspended"`
}

// StockMetadata is the official Open API's batch stock-description record.
// Decimal and date fields stay as strings so identifiers and large share
// counts are never rounded by machine-readable output.
type StockMetadata struct {
	Symbol             string              `json:"symbol"`
	Name               string              `json:"name"`
	EnglishName        string              `json:"english_name"`
	ISINCode           string              `json:"isin_code"`
	MarketCode         string              `json:"market_code"`
	SecurityType       string              `json:"security_type"`
	CommonShare        bool                `json:"common_share"`
	Status             string              `json:"status"`
	Currency           string              `json:"currency"`
	SharesOutstanding  string              `json:"shares_outstanding"`
	ListDate           *string             `json:"list_date"`
	DelistDate         *string             `json:"delist_date"`
	LeverageFactor     *string             `json:"leverage_factor"`
	KoreanMarketDetail *KoreanMarketDetail `json:"korean_market_detail"`
	FetchedAt          time.Time           `json:"fetched_at"`
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

// EarningCallDetail is the full event payload behind an earnings-call
// calendar row. Media URLs are nil until Toss publishes the corresponding
// recording, transcript, or slide deck.
type EarningCallDetail struct {
	EventID                      int64     `json:"event_id"`
	MarketCountry                string    `json:"market_country"`
	Category                     string    `json:"category"`
	DefaultSummarizationCategory string    `json:"default_summarization_category"`
	Status                       string    `json:"status"`
	Title                        string    `json:"title"`
	LiveAt                       string    `json:"live_at"`
	WentLiveAt                   *string   `json:"went_live_at"`
	AudioURL                     *string   `json:"audio_url"`
	TranscriptURL                *string   `json:"transcript_url"`
	SlideFileURL                 *string   `json:"slide_file_url"`
	CompanyCode                  string    `json:"company_code"`
	CompanyName                  string    `json:"company_name"`
	CompanyLogoImageURL          string    `json:"company_logo_image_url,omitempty"`
	RepresentativeStockSymbol    string    `json:"representative_stock_symbol,omitempty"`
	RepresentativeStockGUID      string    `json:"representative_stock_guid,omitempty"`
	RepresentativeStockCode      string    `json:"representative_stock_code,omitempty"`
	ReportID                     string    `json:"report_id,omitempty"`
	ReportItem                   string    `json:"report_item,omitempty"`
	MTSLandingPath               string    `json:"mts_landing_path,omitempty"`
	ConsensusGapRate             *float64  `json:"consensus_gap_rate"`
	IsGapRateVisible             bool      `json:"is_gap_rate_visible"`
	StockChangeRate              *float64  `json:"stock_change_rate"`
	FetchedAt                    time.Time `json:"fetched_at"`
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

// InterestPayment is one deposit-interest payment. Amount is pre-tax and
// PaymentAmount is what actually landed; StartDate/EndDate bound the period
// the interest was accrued over, which is not the month it was paid in.
// Estimated marks a payment Toss has projected but not yet made.
type InterestPayment struct {
	Date          string  `json:"date"`
	Amount        float64 `json:"amount"`
	Tax           float64 `json:"tax"`
	PaymentAmount float64 `json:"payment_amount"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	Estimated     bool    `json:"estimated"`
}

// InterestMonth is one month's deposit-interest total plus its payments.
type InterestMonth struct {
	Month    int               `json:"month"`
	Total    float64           `json:"total"`
	Payments []InterestPayment `json:"payments,omitempty"`
}

// AccountInterest is an annual deposit-interest (예탁금 이용료) report.
// Amounts are KRW only — Toss pays this on the won cash balance.
// Distinct from `profit summary --type account-interest`, which reports one
// period total; this is the per-payment breakdown.
type AccountInterest struct {
	Year           int             `json:"year"`
	Total          float64         `json:"total"`
	Monthly        []InterestMonth `json:"monthly"`
	AvailableYears []int           `json:"available_years,omitempty"`
	FetchedAt      time.Time       `json:"fetched_at"`
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

// LendingRevenueRank is one anonymized account in Toss Securities' current
// share-lending revenue ranking. Revenue preserves the upstream base amount;
// RevenueKRW is the explicit won conversion returned beside it.
type LendingRevenueRank struct {
	Rank       int     `json:"rank"`
	UserName   string  `json:"user_name"`
	Revenue    float64 `json:"revenue"`
	RevenueKRW float64 `json:"revenue_krw"`
}

type LendingRevenueRanking struct {
	Items     []LendingRevenueRank `json:"items"`
	FetchedAt time.Time            `json:"fetched_at"`
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
	ID         string `json:"id,omitempty"`
	Title      string `json:"title"`
	Agency     string `json:"agency"`
	Source     string `json:"source"`
	FaviconURL string `json:"favicon_url,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// BriefingItem is one themed briefing (수급 변동·실적 등) with its headlines.
type BriefingItem struct {
	CategoryType      string         `json:"category_type"`
	Keywords          []string       `json:"keywords"`
	News              []BriefingNews `json:"news"`
	Section           string         `json:"section,omitempty"`
	SignalID          string         `json:"signal_id,omitempty"`
	TraceID           string         `json:"trace_id,omitempty"`
	CreatedAt         string         `json:"created_at,omitempty"`
	AssetCode         string         `json:"asset_code,omitempty"`
	AssetName         string         `json:"asset_name,omitempty"`
	AssetLogoImageURL string         `json:"asset_logo_image_url,omitempty"`
	AssetType         string         `json:"asset_type,omitempty"`
	InvestmentType    string         `json:"investment_type,omitempty"`
	ProfitLossRate    float64        `json:"profit_loss_rate,omitempty"`
	ReasoningTitle    string         `json:"reasoning_title,omitempty"`
	SignalDirection   int            `json:"signal_direction,omitempty"`
	RelatedStocks     []RelatedStock `json:"related_stocks,omitempty"`
}

// NewsBriefing is the personalized AI news briefing grouped by theme.
type NewsBriefing struct {
	CreatedAt string         `json:"created_at"`
	Scope     string         `json:"scope,omitempty"`
	Items     []BriefingItem `json:"items"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// MarketKeyEarning is a company result highlighted in Toss's current key-event
// digest. Numeric result fields are pointers because future announcements are
// present before their actual values exist.
type MarketKeyEarning struct {
	AnnounceAt               string   `json:"announce_at"`
	MarketStatus             string   `json:"market_status,omitempty"`
	MarketStatusText         string   `json:"market_status_text,omitempty"`
	CompanyCode              string   `json:"company_code"`
	CompanyName              string   `json:"company_name"`
	CountryIcon              string   `json:"country_icon,omitempty"`
	LogoImageURL             string   `json:"logo_image_url,omitempty"`
	EPS                      *float64 `json:"eps,omitempty"`
	EPSDisplay               *string  `json:"eps_display,omitempty"`
	EPSEstimate              *float64 `json:"eps_estimate,omitempty"`
	EPSEstimateDisplay       *string  `json:"eps_estimate_display,omitempty"`
	EPSSurprise              *float64 `json:"eps_surprise,omitempty"`
	EPSSurpriseDisplay       *string  `json:"eps_surprise_display,omitempty"`
	Sales                    *float64 `json:"sales,omitempty"`
	SalesDisplay             *string  `json:"sales_display,omitempty"`
	SalesEstimate            *float64 `json:"sales_estimate,omitempty"`
	SalesEstimateDisplay     *string  `json:"sales_estimate_display,omitempty"`
	SalesSurprise            *float64 `json:"sales_surprise,omitempty"`
	SalesSurpriseDisplay     *string  `json:"sales_surprise_display,omitempty"`
	OperatingProfit          *float64 `json:"operating_profit,omitempty"`
	OperatingProfitDisplay   *string  `json:"operating_profit_display,omitempty"`
	OperatingEstimate        *float64 `json:"operating_profit_estimate,omitempty"`
	OperatingEstimateDisplay *string  `json:"operating_profit_estimate_display,omitempty"`
	OperatingSurprise        *float64 `json:"operating_profit_surprise,omitempty"`
	OperatingSurpriseDisplay *string  `json:"operating_profit_surprise_display,omitempty"`
	LegacyReportID           *string  `json:"legacy_report_id,omitempty"`
	ReportID                 string   `json:"report_id,omitempty"`
	ReportItem               string   `json:"report_item,omitempty"`
	LandingURL               string   `json:"landing_url,omitempty"`
}

// MarketKeyIndicator is an economic release highlighted in the same digest.
type MarketKeyIndicator struct {
	AnnounceAt  string   `json:"announce_at"`
	Title       string   `json:"title"`
	Actual      *float64 `json:"actual,omitempty"`
	Forecast    *float64 `json:"forecast,omitempty"`
	Historical  *float64 `json:"historical,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	UnitPrefix  string   `json:"unit_prefix,omitempty"`
	DisplayUnit string   `json:"display_unit,omitempty"`
	RIC         string   `json:"ric,omitempty"`
}

// MarketKeyEvents is the current curated digest of earnings and economic data.
type MarketKeyEvents struct {
	Earnings   []MarketKeyEarning   `json:"earnings"`
	Indicators []MarketKeyIndicator `json:"indicators"`
	FetchedAt  time.Time            `json:"fetched_at"`
}

// OpenBankingAccount is the single bank account currently connected to Toss's
// stock-accumulation funding flow.
type OpenBankingAccount struct {
	AccountNo     string `json:"account_no"`
	BankCode      string `json:"bank_code"`
	OpenBankingID int64  `json:"open_banking_id"`
}

// OpenBankingStatus intentionally carries only fields observed with stable
// schemas. The two account lists were empty during verification, so only their
// counts are exposed rather than inventing item models.
type OpenBankingStatus struct {
	HolderName              string              `json:"holder_name,omitempty"`
	ConnectedAccount        *OpenBankingAccount `json:"connected_account,omitempty"`
	LinkedAccountCount      int                 `json:"linked_account_count"`
	RegistrableAccountCount int                 `json:"registrable_account_count"`
	SavingCount             int                 `json:"saving_count"`
	ConnectionCreatable     bool                `json:"connection_creatable"`
	RegistrationRequired    bool                `json:"registration_required"`
	FetchedAt               time.Time           `json:"fetched_at"`
}

// NotificationSetting is one WTS notification preference. The internal user
// id from the wire response is deliberately not retained.
type NotificationSetting struct {
	ID        int64  `json:"id"`
	Type      string `json:"type,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type NotificationSettings struct {
	Settings  []NotificationSetting `json:"settings"`
	FetchedAt time.Time             `json:"fetched_at"`
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
	Cumulative      PrimeCumulative   `json:"cumulative"`
	FetchedAt       time.Time         `json:"fetched_at"`
}

// PrimeCumulative is the benefit total since joining Prime, as opposed to the
// surrounding PrimeStatus figures which are all current-month.
type PrimeCumulative struct {
	Exchange    float64 `json:"exchange"`
	InterestKRW float64 `json:"interest_krw"`
	InterestUSD float64 `json:"interest_usd"`
	TotalKRW    float64 `json:"total_krw"`
}

// CommissionTier is the commission schedule for one trading surface. Korean
// and US equities charge a rate on notional (RatePercent); US options charge a
// flat fee per contract (PerContract) and leave the rate at zero.
type CommissionTier struct {
	RatePercent    float64 `json:"rate_percent,omitempty"`
	PerContract    float64 `json:"per_contract,omitempty"`
	HasReduction   bool    `json:"has_reduction"`
	ReductionEndAt string  `json:"reduction_end_at,omitempty"`
}

// CommissionSchedule is the account's own commission rates per surface —
// distinct from `quote commission`, which reports the rate and tax for a
// single symbol. Sourced from the v2 endpoint: v1 returns a null US-options
// tier even on accounts that have one.
type CommissionSchedule struct {
	Korea     CommissionTier  `json:"korea"`
	US        CommissionTier  `json:"us"`
	USOptions *CommissionTier `json:"us_options,omitempty"`
	FetchedAt time.Time       `json:"fetched_at"`
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

// OrderList is a page of orders. Mirrors ConditionalOrderList: the cursor and
// HasNext must reach the caller, or a multi-page result looks like the whole
// result. `GET /api/v1/orders` began paginating status=CLOSED in official spec
// v1.2.5 (it used to reject that filter outright), so this is now a live path.
type OrderList struct {
	Orders     []Order   `json:"orders"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasNext    bool      `json:"has_next"`
	FetchedAt  time.Time `json:"fetched_at"`
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

// NewsRelatedStock is a stock a news item is about, with how it is moving —
// the reason this feed is more useful than a bare headline list.
type NewsRelatedStock struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Market      string  `json:"market,omitempty"`
	Fluctuation float64 `json:"fluctuation"` // percent
}

// NewsItem is one article in the market news feed.
type NewsItem struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Summary   string             `json:"summary,omitempty"`
	Source    string             `json:"source,omitempty"`
	Type      string             `json:"type,omitempty"`
	Nation    string             `json:"nation,omitempty"`
	CreatedAt string             `json:"created_at,omitempty"`
	Stocks    []NewsRelatedStock `json:"stocks,omitempty"`
}

// MarketNews is one scope of the news feed. Title is the server's own label for
// the scope (e.g. "모든 주요 뉴스"), carried through rather than hardcoded so a
// rename upstream shows up instead of silently diverging.
type MarketNews struct {
	Type      string     `json:"type"`
	Title     string     `json:"title,omitempty"`
	Items     []NewsItem `json:"items"`
	FetchedAt time.Time  `json:"fetched_at"`
}

// CalendarEvent is one dated entry on the market calendar: an economic
// release, an earnings announcement, or a market holiday.
type CalendarEvent struct {
	Date  string `json:"date"`
	Title string `json:"title"`
	// Kind is a stable alias for the server's group, so callers can filter
	// without hardcoding Toss's enum: economic | earnings_kr | earnings_us |
	// holiday | other. Group keeps the raw value — a new group Toss adds shows
	// up as "other" here but is still identifiable there.
	Kind  string `json:"kind"`
	Group string `json:"group,omitempty"`
	// Note is the server's one-line gloss ("미국 제조업 경기 상황을 빠르게
	// 파악할 수 있어요", or the holiday's actual name).
	Note string `json:"note,omitempty"`
	// Symbol is the stock an earnings entry belongs to, lifted from the
	// event's landing URL. Empty for economic releases and holidays.
	Symbol string `json:"symbol,omitempty"`
	// LiveAt is when an earnings call streams (RFC3339), when Toss has one.
	LiveAt string `json:"live_at,omitempty"`
	// Indicator carries the forecast/actual numbers for economic releases.
	// This is the part a bare calendar lacks: knowing CPI prints tomorrow is
	// less useful than knowing the street expects 54.0 against 53.3 last time.
	Indicator *CalendarIndicator `json:"indicator,omitempty"`
}

// CalendarIndicator is an economic release's expected and realised values.
// All three are pointers because "not published yet" and "zero" are different
// facts, and collapsing them would misreport a print of 0.0.
type CalendarIndicator struct {
	Unit       string   `json:"unit,omitempty"`
	Forecast   *float64 `json:"forecast,omitempty"`
	Actual     *float64 `json:"actual,omitempty"`
	Historical *float64 `json:"historical,omitempty"`
}

// MarketCalendar is one month of scheduled market events plus Toss's AI take
// on the current week.
type MarketCalendar struct {
	// Month is the requested YYYY-MM.
	Month  string          `json:"month"`
	Events []CalendarEvent `json:"events"`
	// Summary/SummaryDetail are the weekly AI note. They describe the current
	// week regardless of which month was asked for, so they are omitted when
	// the caller navigates away from the present.
	Summary       string    `json:"summary,omitempty"`
	SummaryDetail string    `json:"summary_detail,omitempty"`
	Warnings      []string  `json:"warnings,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
}

// AutoTrade is one automated-trading rule the user set in the Toss app or web
// (stop-loss, target-profit, OCO, OTO). tossctl cannot create or change them —
// this is the read that lets you see what is armed on your account, which was
// otherwise only visible inside Toss's own UI.
type AutoTrade struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	// Status is the server's lifecycle code translated to its own enum name
	// (READY, ORDERED, EXPIRED, COMPLETED, …). The wire value is a bare number
	// ("6"), meaningless on its own; the mapping was read out of Toss's web
	// bundle rather than guessed. StatusCode keeps the raw value so a code Toss
	// adds later is still reportable.
	Status     string `json:"status"`
	StatusCode string `json:"status_code,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	Name       string `json:"name,omitempty"`
	Market     string `json:"market,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	// Quantity is how much the rule trades; AllQuantity means "everything held".
	Quantity    float64 `json:"quantity,omitempty"`
	AllQuantity bool    `json:"all_quantity,omitempty"`
	// TriggerPrice is the watch price that arms the rule, OrderPrice the price
	// it then orders at. Currency says which unit both are in.
	TriggerPrice float64 `json:"trigger_price,omitempty"`
	OrderPrice   float64 `json:"order_price,omitempty"`
	Currency     string  `json:"currency,omitempty"`
	TradeType    string  `json:"trade_type,omitempty"`
}

// AutoTradeList is a page of automated-trading rules.
type AutoTradeList struct {
	Items     []AutoTrade `json:"items"`
	HasNext   bool        `json:"has_next"`
	FetchedAt time.Time   `json:"fetched_at"`
}

// MarketIssue is one ranked market topic: a cluster of news the feed groups
// under a single theme, with its position and which way it is moving.
//
// Distinct from both existing surfaces: `market news` is a flat headline list,
// `market briefing` groups articles by AI category. This one ranks the topics
// themselves, so it answers "what is the market talking about most right now".
type MarketIssue struct {
	Rank int `json:"rank"`
	// RankStatus is the server's movement flag (UP / DOWN / …), passed through.
	RankStatus string `json:"rank_status,omitempty"`
	// Topic is the short key phrase ("CXMT 메모리 증설"); Title is the fuller
	// framing ("글로벌 D램 공급 확대"). Both come from the server.
	Topic    string `json:"topic"`
	Title    string `json:"title,omitempty"`
	Category string `json:"category,omitempty"`
	// SourceCount is the server's total; Sources carries the ones it sent,
	// which can be fewer.
	SourceCount int           `json:"source_count,omitempty"`
	Sources     []IssueSource `json:"sources,omitempty"`
}

// IssueSource is one article backing an issue topic.
type IssueSource struct {
	Name      string `json:"name,omitempty"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at,omitempty"`
}

// MarketIssues is the ranked topic board.
type MarketIssues struct {
	Issues    []MarketIssue `json:"issues"`
	UpdatedAt string        `json:"updated_at,omitempty"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// WithdrawableByDay is cash available now (D+0) and as settlement completes.
type WithdrawableByDay struct {
	Day0 float64 `json:"day0"`
	Day1 float64 `json:"day1"`
	Day2 float64 `json:"day2"`
}

// WithdrawalLimits are the caps Toss applies to outgoing transfers.
type WithdrawalLimits struct {
	PerTransaction float64 `json:"per_transaction"`
	PerDay         float64 `json:"per_day"`
	UsedToday      float64 `json:"used_today"`
}

// MarginStatus reports whether 미수거래 (buying on receivable credit) is open,
// per market. Message carries the server's explanation when it is not.
type MarginStatus struct {
	Receivable bool   `json:"receivable"`
	Message    string `json:"message,omitempty"`
}

// AccountDetail mirrors the web's 계좌관리 screen: what the account is, what can
// be withdrawn, and whether credit trading is open. Read-only — the screen's
// other half (closing the account, changing the PIN, moving money) is
// deliberately out of scope.
//
// Warnings records sections that could not be fetched. Only the identity part
// is required; the rest degrades rather than failing the whole command, since a
// missing margin endpoint should not hide the account number.
type AccountDetail struct {
	Number       string `json:"number"`
	Name         string `json:"name,omitempty"`
	Status       string `json:"status,omitempty"`
	OpenedAt     string `json:"opened_at,omitempty"`
	LastTradedAt string `json:"last_traded_at,omitempty"`

	Withdrawable       *WithdrawableByDay `json:"withdrawable,omitempty"`
	WithdrawalLimits   *WithdrawalLimits  `json:"withdrawal_limits,omitempty"`
	FullWithdrawalOn   string             `json:"full_withdrawal_on,omitempty"`
	TransferRestricted *bool              `json:"transfer_restricted,omitempty"`

	MarginKR           *MarginStatus `json:"margin_kr,omitempty"`
	MarginUS           *MarginStatus `json:"margin_us,omitempty"`
	DifferentialMargin *bool         `json:"differential_margin,omitempty"`

	USDividendOption *USDividendOption `json:"us_dividend_option,omitempty"`

	// TradePurpose explains a transfer restriction that TransferRestricted only
	// flags as a bool. Status/RejectReasonType are the server's own codes.
	TradePurpose *TradePurposeVerification `json:"trade_purpose,omitempty"`

	Warnings  []string  `json:"warnings,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// TradePurposeVerification is the trade-purpose review (거래목적 확인) state
// behind a transfer-limit restriction. Codes are kept verbatim — Toss ships no
// web mapping for them, so translating would be a guess. RejectReason is the
// server's own free text and is empty when nothing was rejected.
type TradePurposeVerification struct {
	Purpose          string `json:"purpose,omitempty"`
	Status           string `json:"status,omitempty"`
	RejectReasonType string `json:"reject_reason_type,omitempty"`
	RejectReason     string `json:"reject_reason,omitempty"`
	DocumentType     string `json:"document_type,omitempty"`
	OpenedAt         string `json:"opened_at,omitempty"`
}

// USDividendOption is how US dividends land in this account: as cash, or
// reinvested into more shares. It is an account-level setting with no web
// screen (the Toss app is the only place to change it), which is exactly why
// reading it here is worth something — the choice changes both the tax event
// and the share count, and there is otherwise no way to see it from a desktop.
type USDividendOption struct {
	// GiveType is the server enum: "CASH" (paid out) or "STOCK" (reinvested).
	// Passed through rather than translated — Toss may add values, and a
	// mistranslated money setting is worse than a raw one.
	GiveType string `json:"give_type"`
	// UpdatedAt is when the setting last changed ("" if never).
	UpdatedAt string `json:"updated_at,omitempty"`
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

// RIAQuarterProfitLoss is one period's weighted profit/loss. The RIA
// deduction weights periods differently, so the weight is carried rather than
// folded into the amount. Quarter is the server's own label and is not always
// a quarter — live responses mix "Q1"/"Q2" with "H2" (a half-year), so treat
// it as an opaque period label, never parse it.
type RIAQuarterProfitLoss struct {
	Quarter            string  `json:"quarter"`
	Weight             float64 `json:"weight"`
	TotalProfitLoss    float64 `json:"total_profit_loss"`
	WeightedProfitLoss float64 `json:"weighted_profit_loss"`
}

// RIADeduction is the RIA-account portion of the transfer-income deduction.
type RIADeduction struct {
	DeductionRate                   float64                `json:"deduction_rate"`
	NormalAccountOverseasBuyAmount  float64                `json:"normal_account_overseas_buy_amount"`
	NormalAccountOverseasSellAmount float64                `json:"normal_account_overseas_sell_amount"`
	RIAAccountOverseasSellAmount    float64                `json:"ria_account_overseas_sell_amount"`
	PreAdjustmentDeduction          float64                `json:"pre_adjustment_deduction"`
	TotalAmount                     float64                `json:"total_amount"`
	QuarterlyProfitLoss             []RIAQuarterProfitLoss `json:"quarterly_profit_loss,omitempty"`
}

// RIALimit is the account's RIA sell-limit state.
type RIALimit struct {
	TotalLimit              float64 `json:"total_limit"`
	RemainingLimit          float64 `json:"remaining_limit"`
	OverseasStockSellAmount float64 `json:"overseas_stock_sell_amount"`
	SettlementDate          string  `json:"settlement_date,omitempty"`
	Settled                 bool    `json:"settled"`
}

// RIAReport is the RIA (해외주식 양도세 절세 계좌) tax-saving projection:
// estimated tax with and without the RIA deduction, and the deduction's
// components. Complements `tax overseas`, which reports the plain filing
// figures with no RIA concept.
type RIAReport struct {
	EstimatedTransferIncomeTax float64 `json:"estimated_transfer_income_tax"`
	EstimatedTaxSaving         float64 `json:"estimated_tax_saving"`
	FinalTransferIncomeTax     float64 `json:"final_transfer_income_tax"`

	TotalTransferIncomeAmount   float64      `json:"total_transfer_income_amount"`
	NormalAccountTransferIncome float64      `json:"normal_account_transfer_income"`
	RIAAccountTransferIncome    float64      `json:"ria_account_transfer_income"`
	BaseDeduction               float64      `json:"base_deduction"`
	Deduction                   RIADeduction `json:"deduction"`
	ProfitAfterDeduction        float64      `json:"profit_after_deduction"`
	TransferIncomeTaxRate       float64      `json:"transfer_income_tax_rate"`
	TransferIncomeTax           float64      `json:"transfer_income_tax"`
	LocalTaxRate                float64      `json:"local_tax_rate"`
	LocalTax                    float64      `json:"local_tax"`

	// MaxTaxSaving is the best saving still reachable this year.
	// ZeroReasonCode is the server's raw reason when it is zero (e.g.
	// NO_PROFITABLE_STOCKS) — kept verbatim because Toss ships no web
	// mapping for it and guessing a translation would be worse than the code.
	MaxTaxSaving   *float64 `json:"max_tax_saving,omitempty"`
	ZeroReasonCode string   `json:"zero_reason_code,omitempty"`

	Limit     *RIALimit `json:"limit,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// CryptoPrice is one crypto pair's snapshot from the WTS crypto tape.
//
// Toss quotes crypto against KRW under codes shaped `VWAP.KRW-BTC` — a
// volume-weighted average across the exchanges it aggregates, not a single
// venue's tape. Symbol is the short form (`BTC`) for display; ProductCode is
// what the API takes.
//
// Premium is the "김치 프리미엄": how far the KRW price sits from the global
// USD price converted at USDPerKRW. It is negative when Korea trades below
// the global market, so it is kept signed rather than normalised.
type CryptoPrice struct {
	ProductCode string  `json:"product_code"`
	Symbol      string  `json:"symbol"`
	Base        float64 `json:"base,omitempty"` // 기준가 (전일 종가)
	Open        float64 `json:"open,omitempty"`
	High        float64 `json:"high,omitempty"`
	Low         float64 `json:"low,omitempty"`
	Close       float64 `json:"close,omitempty"` // 현재가
	Change      float64 `json:"change,omitempty"`
	ChangeRate  float64 `json:"change_rate,omitempty"` // 퍼센트 (1.5 = 1.5%)
	ChangeType  string  `json:"change_type,omitempty"`
	Volume      float64 `json:"volume,omitempty"`
	Value       float64 `json:"value,omitempty"` // 거래대금 (KRW)
	High52w     float64 `json:"high_52w,omitempty"`
	Low52w      float64 `json:"low_52w,omitempty"`

	USDPerKRW   float64 `json:"usd_per_krw,omitempty"`
	Premium     float64 `json:"premium,omitempty"`      // KRW
	PremiumRate float64 `json:"premium_rate,omitempty"` // 퍼센트, 부호 유지
}

type CryptoPrices struct {
	Prices    []CryptoPrice `json:"prices"`
	FetchedAt time.Time     `json:"fetched_at"`
}

// StockReasoning is Toss's AI explanation of why a stock moved today
// ("왜 올랐을까?"). Direction is the server's own sign: positive for a rise,
// negative for a fall.
type StockReasoning struct {
	Symbol       string         `json:"symbol"`
	ProductCode  string         `json:"product_code"`
	Title        string         `json:"title,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Direction    int            `json:"direction,omitempty"`
	Keyword      string         `json:"keyword,omitempty"`
	SignalID     string         `json:"signal_id,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	RelatedStock []RelatedStock `json:"related_stocks,omitempty"`
	FetchedAt    time.Time      `json:"fetched_at"`
}

// RelatedStock is a stock the reasoning cites as connected to the move.
// InvestmentTypeValue is the server's display string for InvestmentType; both
// are kept verbatim because Toss ships no public mapping for the enum.
type RelatedStock struct {
	ProductCode         string `json:"product_code"`
	Name                string `json:"name,omitempty"`
	Symbol              string `json:"symbol,omitempty"`
	Market              string `json:"market,omitempty"`
	InvestmentType      string `json:"investment_type,omitempty"`
	InvestmentTypeValue string `json:"investment_type_value,omitempty"`
	CompanyCode         string `json:"company_code,omitempty"`
	CompanyName         string `json:"company_name,omitempty"`
	LogoImageURL        string `json:"logo_image_url,omitempty"`
	Status              string `json:"status,omitempty"`
	CommonShare         string `json:"common_share,omitempty"`
	Display             string `json:"display,omitempty"`
}

// StockSignals are the per-stock signal cards Toss shows on a stock page —
// distinct from the market-wide AI signals behind `market signals`.
type StockSignals struct {
	Symbol      string        `json:"symbol"`
	ProductCode string        `json:"product_code"`
	Signals     []StockSignal `json:"signals"`
	FetchedAt   time.Time     `json:"fetched_at"`
}

type StockSignal struct {
	Label    string `json:"label,omitempty"` // 호재 / 악재 등 (서버 원문)
	Info     string `json:"info,omitempty"`
	SignalID int64  `json:"signal_id,omitempty"`
	DateTime string `json:"datetime,omitempty"`
}

// MarginNotice is the receivable (미수금) / forced-liquidation warning state
// for one currency.
//
// Every timestamp is nil in the healthy case — the account owes nothing. They
// are pointers rather than zero times so "no deadline" never renders as an
// epoch date, which would read as an overdue account.
type MarginNotice struct {
	Currency           string    `json:"currency"`
	NoticeType         string    `json:"notice_type,omitempty"` // 서버 원문 (NONE 등)
	ReceivableAmount   float64   `json:"receivable_amount"`
	DeadlineAt         *string   `json:"deadline_at,omitempty"`
	ForcedLiquidatedAt *string   `json:"forced_liquidated_at,omitempty"`
	SuspensionStart    *string   `json:"suspension_start_date,omitempty"`
	SuspensionEnd      *string   `json:"suspension_end_date,omitempty"`
	FetchedAt          time.Time `json:"fetched_at"`
}

// SearchResults are unified search hits — stocks today, with bond and community
// fields present but unset on this surface.
type SearchResults struct {
	Query     string      `json:"query"`
	Results   []SearchHit `json:"results"`
	FetchedAt time.Time   `json:"fetched_at"`
}

type SearchHit struct {
	Keyword     string `json:"keyword"`
	SubKeyword  string `json:"sub_keyword,omitempty"`
	ProductCode string `json:"product_code,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	Market      string `json:"market,omitempty"`
}

// OptionExpiry is one listed expiration for an underlying.
//
// DisplayLiquidation is Toss's own display string ("거래 종료" and the like) and
// is kept verbatim: it encodes states the timestamps alone don't distinguish.
type OptionExpiry struct {
	MaturityDate        string `json:"maturity_date"`
	MaturityDateTime    string `json:"maturity_datetime,omitempty"`
	LiquidationDateTime string `json:"liquidation_datetime,omitempty"`
	DisplayLiquidation  string `json:"display_liquidation,omitempty"`
	CorporateActionName string `json:"corporate_action_name,omitempty"`
}

type OptionExpiries struct {
	Symbol      string         `json:"symbol"`
	ProductCode string         `json:"product_code"`
	Expiries    []OptionExpiry `json:"expiries"`
	FetchedAt   time.Time      `json:"fetched_at"`
}

// OptionChainRow is one strike, with the call and put side by side.
// OpenInterest is contracts outstanding — the chain carries no prices.
type OptionChainRow struct {
	StrikePrice      float64 `json:"strike_price"`
	CallGUID         string  `json:"call_guid,omitempty"`
	PutGUID          string  `json:"put_guid,omitempty"`
	CallOpenInterest int     `json:"call_open_interest"`
	PutOpenInterest  int     `json:"put_open_interest"`
}

type OptionChain struct {
	Symbol       string           `json:"symbol"`
	ProductCode  string           `json:"product_code"`
	MaturityDate string           `json:"maturity_date"`
	Rows         []OptionChainRow `json:"rows"`
	FetchedAt    time.Time        `json:"fetched_at"`
}

// ScreenerFilterRange is the observed value span of one screener filter, plus
// the date the underlying data is based on.
//
// This is what makes a filter usable: `market screener` accepts a filter id but
// nothing said what values are in bounds. Min/Max come from the live universe,
// so they move day to day — BasedAt says which day.
//
// Unavailable marks filters the server refused because they need extra
// conditions (a period, typically) that this surface can't express. The reason
// is the server's own code, kept verbatim.
type ScreenerFilterRange struct {
	FilterID    string   `json:"filter_id"`
	Nation      string   `json:"nation"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	BasedAt     string   `json:"based_at,omitempty"`
	Unavailable string   `json:"unavailable_reason,omitempty"`
}

type ScreenerFilterRanges struct {
	Nation    string                `json:"nation"`
	Filters   []ScreenerFilterRange `json:"filters"`
	FetchedAt time.Time             `json:"fetched_at"`
}

// --- 종목 수급 (공식 Open API 1.2.13 신규) ---------------------------------
//
// 다섯 표면(투자자별·공매도·신용·대차·프로그램)이 같은 모양이다: 일별 시계열 +
// `NextUntil` 커서. 그래서 SupplySeries 하나가 다섯을 다 담고, Kind 로 갈린다.
//
// **nullable 을 포인터로 유지하는 게 이 도메인의 핵심이다.** 당일 잠정 기록에는
// 개인 잠정치·외국인 보유·CFD 잔고가 아직 안 실린다. 0 으로 채우면 "순매수 0" 과
// "아직 집계 안 됨" 이 구분되지 않는데, 수급에서 그 둘은 정반대 신호다.

// SupplyKind identifies which supply series a record belongs to.
type SupplyKind string

const (
	SupplyInvestor SupplyKind = "investor"
	SupplyShort    SupplyKind = "short"
	SupplyCredit   SupplyKind = "credit"
	SupplyLending  SupplyKind = "lending"
	SupplyProgram  SupplyKind = "program"
)

// TradingVolume is a buy/sell/net triple, the unit every investor-side figure
// is reported in.
type TradingVolume struct {
	Buy    float64 `json:"buy"`
	Sell   float64 `json:"sell"`
	NetBuy float64 `json:"net_buy"`
}

// InstitutionBreakdown splits the institution total into its seven reported
// sub-categories.
// 각 분류가 매수/매도/순매수 삼중이다 — 합계와 같은 단위다. 스펙은 이 필드들을
// allOf → InvestorTradingVolume 로 적어두는데, 얕게 읽으면 스칼라로 착각한다.
type InstitutionBreakdown struct {
	FinancialInvestment       *TradingVolume `json:"financial_investment,omitempty"`        // 금융투자
	Insurance                 *TradingVolume `json:"insurance,omitempty"`                   // 보험
	Trust                     *TradingVolume `json:"trust,omitempty"`                       // 투신
	Bank                      *TradingVolume `json:"bank,omitempty"`                        // 은행
	OtherFinancialInstitution *TradingVolume `json:"other_financial_institution,omitempty"` // 기타금융
	PensionFund               *TradingVolume `json:"pension_fund,omitempty"`                // 연기금
	PrivateEquityFund         *TradingVolume `json:"private_equity_fund,omitempty"`         // 사모펀드
}

// ForeignerHolding is the foreign ownership snapshot for a date.
type ForeignerHolding struct {
	HoldingQuantity float64 `json:"holding_quantity"`
	HoldingRate     float64 `json:"holding_rate"`
	LimitQuantity   float64 `json:"limit_quantity"`
}

// CFDBalance is the contract-for-difference balance on both sides.
type CFDBalance struct {
	BuyBalanceQuantity  float64 `json:"buy_balance_quantity"`
	BuyBalanceRate      float64 `json:"buy_balance_rate"`
	SellBalanceQuantity float64 `json:"sell_balance_quantity"`
	SellBalanceRate     float64 `json:"sell_balance_rate"`
}

// CreditDetail is one side of the credit series (융자 or 대주).
type CreditDetail struct {
	NewQuantity     float64 `json:"new_quantity"`
	ReturnQuantity  float64 `json:"return_quantity"`
	BalanceQuantity float64 `json:"balance_quantity"`
	BalanceRate     float64 `json:"balance_rate"`
	TradingRate     float64 `json:"trading_rate"`
}

// SupplyRecord is one day of one supply series. Only the fields belonging to
// the series' Kind are set; the rest stay nil.
type SupplyRecord struct {
	Date      string `json:"date"`
	UpdatedAt string `json:"updated_at,omitempty"`

	// investor
	Individual       *TradingVolume        `json:"individual,omitempty"`
	Foreigner        *TradingVolume        `json:"foreigner,omitempty"`
	Institution      *TradingVolume        `json:"institution,omitempty"`
	InstitutionSplit *InstitutionBreakdown `json:"institution_breakdown,omitempty"`
	OtherCorporation *TradingVolume        `json:"other_corporation,omitempty"`
	ForeignerHolding *ForeignerHolding     `json:"foreigner_holding,omitempty"`
	CFD              *CFDBalance           `json:"cfd,omitempty"`

	// short selling
	ShortVolume     *float64 `json:"short_volume,omitempty"`
	ShortAmount     *float64 `json:"short_amount,omitempty"`
	ShortVolumeRate *float64 `json:"short_volume_rate,omitempty"`
	ShortAmountRate *float64 `json:"short_amount_rate,omitempty"`

	// credit
	MarginLoan *CreditDetail `json:"margin_loan,omitempty"` // 신용융자
	StockLoan  *CreditDetail `json:"stock_loan,omitempty"`  // 대주

	// securities lending
	LendingExecution  *float64 `json:"lending_execution,omitempty"`
	LendingRepayment  *float64 `json:"lending_repayment,omitempty"`
	LendingBalanceQty *float64 `json:"lending_balance_quantity,omitempty"`
	LendingBalanceAmt *float64 `json:"lending_balance_amount,omitempty"`

	// program
	Arbitrage    *TradingVolume `json:"arbitrage,omitempty"`
	NonArbitrage *TradingVolume `json:"non_arbitrage,omitempty"`
}

// SupplySeries is one symbol's supply history for a single Kind.
//
// NextUntil is the server's cursor for the next (older) page; empty means the
// series ended.
type SupplySeries struct {
	Symbol    string         `json:"symbol"`
	Kind      SupplyKind     `json:"kind"`
	Records   []SupplyRecord `json:"records"`
	NextUntil string         `json:"next_until,omitempty"`
	FetchedAt time.Time      `json:"fetched_at"`
}

// ListedStock is one entry of a market's tradable universe.
//
// Deliberately thin — the API returns thousands of these at once (NASDAQ ~2,800)
// and calls this a universe-construction surface. Names and prices are fetched
// per-symbol afterwards; carrying them here would just inflate every row.
type ListedStock struct {
	Symbol       string `json:"symbol"`
	Name         string `json:"name,omitempty"`
	ISINCode     string `json:"isin_code,omitempty"`
	SecurityType string `json:"security_type,omitempty"`
	CommonShare  bool   `json:"common_share"`
}

// StockUniverse is one market's listed stocks, already sorted by symbol on the
// server. Order is preserved.
type StockUniverse struct {
	Market    string        `json:"market"`
	Status    string        `json:"status,omitempty"`
	Stocks    []ListedStock `json:"stocks"`
	FetchedAt time.Time     `json:"fetched_at"`
}
