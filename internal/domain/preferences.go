package domain

// PriceAlert is one server-side stock target-price alert.
type PriceAlert struct {
	TargetPrice float64 `json:"target_price"`
	Currency    string  `json:"currency"`
}

// PriceAlerts groups every target-price alert for one canonical product code.
type PriceAlerts struct {
	ProductCode string       `json:"product_code"`
	Alerts      []PriceAlert `json:"alerts"`
}

// HiddenHolding is one portfolio holding hidden from the Toss Securities asset view.
type HiddenHolding struct {
	ProductCode      string  `json:"product_code"`
	Name             string  `json:"name,omitempty"`
	Type             string  `json:"type,omitempty"`
	LogoImageURL     string  `json:"logo_image_url,omitempty"`
	TradableQuantity float64 `json:"tradable_quantity,omitempty"`
}

// HiddenHoldings contains the hidden holdings for one account. AccountKey is
// intentionally excluded from serialization because it is an internal account
// identifier; callers can use AccountScope when a stable display value is needed.
type HiddenHoldings struct {
	AccountKey   string          `json:"-"`
	AccountScope string          `json:"account_scope"`
	Holdings     []HiddenHolding `json:"holdings"`
}
