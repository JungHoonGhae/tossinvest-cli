package domain

import "time"

// AssetAmount is one portfolio value expressed in both KRW and USD.
type AssetAmount struct {
	KRW float64 `json:"krw"`
	USD float64 `json:"usd"`
}

// AssetRate is one portfolio return rate expressed on the KRW and USD bases.
type AssetRate struct {
	KRW float64 `json:"krw"`
	USD float64 `json:"usd"`
}

// AssetSnapshotScope identifies whether a valuation covers all accounts or
// one session-bound account.
type AssetSnapshotScope string

const (
	AssetSnapshotScopeAllAccounts AssetSnapshotScope = "all_accounts"
	AssetSnapshotScopeAccount     AssetSnapshotScope = "account"
)

// AssetSnapshotExtreme identifies the highest or lowest evaluated amount in a
// performance range.
type AssetSnapshotExtreme struct {
	BaseDate string      `json:"base_date"`
	Amount   AssetAmount `json:"amount"`
}

// AssetSnapshotPoint is one dated point in a portfolio performance series.
type AssetSnapshotPoint struct {
	BaseDate           string      `json:"base_date"`
	PrincipalAmount    AssetAmount `json:"principal_amount"`
	EvaluatedAmount    AssetAmount `json:"evaluated_amount"`
	ProfitLossAmount   AssetAmount `json:"profit_loss_amount"`
	ProfitLossRate     AssetRate   `json:"profit_loss_rate"`
	Realtime           bool        `json:"realtime"`
	EvaluationComplete bool        `json:"evaluation_complete"`
}

// AssetPerformance is the verified one-month daily historical portfolio
// valuation. Scope is all_accounts unless AccountScope identifies one
// session-bound account.
type AssetPerformance struct {
	Scope               AssetSnapshotScope   `json:"scope"`
	AccountScope        string               `json:"account_scope,omitempty"`
	Range               string               `json:"range"`
	StepUnit            string               `json:"step_unit"`
	HasKRStock          bool                 `json:"has_kr_stock"`
	HasKRStockInRange   bool                 `json:"has_kr_stock_in_range"`
	HasProduct          bool                 `json:"has_product"`
	HasProductInRange   bool                 `json:"has_product_in_range"`
	EvaluatedAmountDiff AssetAmount          `json:"evaluated_amount_diff"`
	MaxEvaluated        AssetSnapshotExtreme `json:"max_evaluated"`
	MinEvaluated        AssetSnapshotExtreme `json:"min_evaluated"`
	Points              []AssetSnapshotPoint `json:"points"`
	FetchedAt           time.Time            `json:"fetched_at"`
}

// AssetSnapshotPage is one cursor page of dated portfolio valuations. Toss
// can include the current realtime point in addition to the requested history
// page size, so Snapshots is not truncated locally.
type AssetSnapshotPage struct {
	Scope        AssetSnapshotScope   `json:"scope"`
	AccountScope string               `json:"account_scope,omitempty"`
	PageSize     int                  `json:"page_size"`
	Snapshots    []AssetSnapshotPoint `json:"snapshots"`
	NextCursor   string               `json:"next_cursor,omitempty"`
	HasNext      bool                 `json:"has_next"`
	FetchedAt    time.Time            `json:"fetched_at"`
}

// AssetSnapshotHolding is one security present in a dated valuation.
type AssetSnapshotHolding struct {
	ProductCode      string      `json:"product_code"`
	ISIN             string      `json:"isin,omitempty"`
	Symbol           string      `json:"symbol"`
	Name             string      `json:"name"`
	Quantity         float64     `json:"quantity"`
	PurchasePrice    AssetAmount `json:"purchase_price"`
	PurchaseAmount   AssetAmount `json:"purchase_amount"`
	EvaluatedAmount  AssetAmount `json:"evaluated_amount"`
	ProfitLossAmount AssetAmount `json:"profit_loss_amount"`
	ProfitLossRate   AssetRate   `json:"profit_loss_rate"`
	MarketDivision   string      `json:"market_division,omitempty"`
	LogoImageURL     string      `json:"logo_image_url,omitempty"`
	Type             string      `json:"type,omitempty"`
}

// AssetSnapshotMarket is a dated valuation section for Korean stocks, US
// stocks, US options, or bonds.
type AssetSnapshotMarket struct {
	Market           string                 `json:"market"`
	PrincipalAmount  AssetAmount            `json:"principal_amount"`
	EvaluatedAmount  AssetAmount            `json:"evaluated_amount"`
	ProfitLossAmount AssetAmount            `json:"profit_loss_amount"`
	ProfitLossRate   AssetRate              `json:"profit_loss_rate"`
	Holdings         []AssetSnapshotHolding `json:"holdings"`
}

// AssetSnapshotDetail is the complete portfolio valuation for one base date.
type AssetSnapshotDetail struct {
	Scope              AssetSnapshotScope    `json:"scope"`
	AccountScope       string                `json:"account_scope,omitempty"`
	BaseDate           string                `json:"base_date"`
	EvaluationComplete bool                  `json:"evaluation_complete"`
	PrincipalAmount    AssetAmount           `json:"principal_amount"`
	EvaluatedAmount    AssetAmount           `json:"evaluated_amount"`
	ProfitLossAmount   AssetAmount           `json:"profit_loss_amount"`
	ProfitLossRate     AssetRate             `json:"profit_loss_rate"`
	Markets            []AssetSnapshotMarket `json:"markets"`
	FetchedAt          time.Time             `json:"fetched_at"`
}
