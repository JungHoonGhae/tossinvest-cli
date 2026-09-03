package domain

import "time"

// PortfolioFolderSummary is the common valuation rollup returned for the
// complete grouped portfolio and for each individual folder.
type PortfolioFolderSummary struct {
	PrincipalAmount          AssetAmount `json:"principal_amount"`
	EvaluatedAmount          AssetAmount `json:"evaluated_amount"`
	EvaluatedAmountAfterFees AssetAmount `json:"evaluated_amount_after_fees"`
	ProfitLossAmount         AssetAmount `json:"profit_loss_amount"`
	ProfitLossAfterFees      AssetAmount `json:"profit_loss_after_fees"`
	DailyProfitLossAmount    AssetAmount `json:"daily_profit_loss_amount"`
	ProfitLossRate           AssetRate   `json:"profit_loss_rate"`
	ProfitLossRateAfterFees  AssetRate   `json:"profit_loss_rate_after_fees"`
	DailyProfitLossRate      AssetRate   `json:"daily_profit_loss_rate"`
}

// PortfolioFolderItem is one holding in the user-defined portfolio grouping.
// Internal folder/item keys are deliberately omitted from this read surface.
type PortfolioFolderItem struct {
	Key               string  `json:"-"`
	ProductCode       string  `json:"product_code"`
	ISIN              string  `json:"isin,omitempty"`
	Symbol            string  `json:"symbol"`
	Name              string  `json:"name"`
	Quantity          float64 `json:"quantity"`
	TradableQuantity  float64 `json:"tradable_quantity"`
	UnsettledQuantity float64 `json:"unsettled_quantity"`

	CurrentPrice             AssetAmount `json:"current_price"`
	BasePrice                AssetAmount `json:"base_price"`
	PurchasePrice            AssetAmount `json:"purchase_price"`
	PurchaseAmount           AssetAmount `json:"purchase_amount"`
	EvaluatedAmount          AssetAmount `json:"evaluated_amount"`
	EvaluatedAmountAfterFees AssetAmount `json:"evaluated_amount_after_fees"`
	ProfitLossAmount         AssetAmount `json:"profit_loss_amount"`
	ProfitLossAfterFees      AssetAmount `json:"profit_loss_after_fees"`
	DailyProfitLossAmount    AssetAmount `json:"daily_profit_loss_amount"`
	ProfitLossRate           AssetRate   `json:"profit_loss_rate"`
	ProfitLossRateAfterFees  AssetRate   `json:"profit_loss_rate_after_fees"`
	DailyProfitLossRate      AssetRate   `json:"daily_profit_loss_rate"`
	Commission               AssetAmount `json:"commission"`
	CommissionRate           float64     `json:"commission_rate"`
	BuyCommission            AssetAmount `json:"buy_commission"`
	SellCommission           AssetAmount `json:"sell_commission"`

	MarketCode        string `json:"market_code,omitempty"`
	MarketDivision    string `json:"market_division,omitempty"`
	Type              string `json:"type,omitempty"`
	ShareHoldingsType string `json:"share_holdings_type,omitempty"`
	Delisting         bool   `json:"delisting"`
	Unlisting         bool   `json:"unlisting"`
	Archiving         bool   `json:"archiving"`
	ErrorPricing      bool   `json:"error_pricing"`
	NXTSupported      bool   `json:"nxt_supported"`
}

// PortfolioFolder is one server-defined default or user-defined holding group.
type PortfolioFolder struct {
	PortfolioFolderSummary
	Key        string                `json:"-"`
	Name       string                `json:"name"`
	Type       string                `json:"type"`
	DetailType string                `json:"detail_type,omitempty"`
	Default    bool                  `json:"default"`
	Items      []PortfolioFolderItem `json:"items"`
}

type PortfolioHiddenSummary struct {
	Count  int     `json:"count"`
	All    bool    `json:"all"`
	Amount float64 `json:"amount"`
}

// PortfolioFolders is the FOLDER_OVERVIEW_V2 read model for one Securities
// account. AccountScope is session-bound and does not expose the account key.
type PortfolioFolders struct {
	PortfolioFolderSummary
	SectionType  string                 `json:"section_type"`
	AccountScope string                 `json:"account_scope"`
	Folders      []PortfolioFolder      `json:"folders"`
	Hidden       PortfolioHiddenSummary `json:"hidden"`
	FetchedAt    time.Time              `json:"fetched_at"`
}
