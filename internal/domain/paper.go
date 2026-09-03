package domain

import (
	"encoding/json"

	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

// PaperStatus describes the isolated US-options simulation ledger. Values in
// this model are simulated and must never be combined with live AccountSummary.
type PaperStatus struct {
	Environment string                `json:"environment"`
	Product     string                `json:"product"`
	Balance     PaperCashBalance      `json:"balance"`
	Education   PaperEducationSummary `json:"education"`
}

type PaperCashBalance struct {
	Deposit             float64 `json:"deposit"`
	OrderableAmount     float64 `json:"orderable_amount"`
	WithdrawableAmount  float64 `json:"withdrawable_amount"`
	MarginAmount        float64 `json:"margin_amount"`
	UnsettledAmount     float64 `json:"unsettled_amount"`
	BuyExecutionAmount  float64 `json:"buy_execution_amount"`
	SellExecutionAmount float64 `json:"sell_execution_amount"`
}

type PaperEducationProgress struct {
	TotalSeconds     int  `json:"total_seconds"`
	RequiredSeconds  int  `json:"required_seconds"`
	RemainingSeconds int  `json:"remaining_seconds"`
	Completed        bool `json:"completed"`
}

type PaperEducationSummary struct {
	Lecture                    PaperEducationProgress `json:"lecture"`
	PaperTrading               PaperEducationProgress `json:"paper_trading"`
	AllCompleted               bool                   `json:"all_completed"`
	OverseasDerivativeEligible bool                   `json:"overseas_derivative_eligible"`
}

// PaperOrderPreview is the server-validated simulation preview. The short-lived
// order key returned by Toss is deliberately excluded from this public model.
type PaperOrderPreview struct {
	Environment      string                        `json:"environment"`
	Product          string                        `json:"product"`
	Intent           orderintent.OptionPlaceIntent `json:"intent"`
	PreparedQuantity int                           `json:"prepared_quantity"`
	AuthRequired     bool                          `json:"auth_required"`
}

type PaperMutationReceipt struct {
	Environment string `json:"environment"`
	Message     string `json:"message,omitempty"`
	OrderDate   string `json:"order_date,omitempty"`
	OrderNo     string `json:"order_no,omitempty"`
	OrderID     string `json:"order_id,omitempty"`
}

type PaperOrder struct {
	ID                 string          `json:"id"`
	OrderID            string          `json:"order_id,omitempty"`
	OrderNo            string          `json:"order_no"`
	OrderDate          string          `json:"order_date"`
	OrderedAt          string          `json:"ordered_at,omitempty"`
	StockCode          string          `json:"stock_code"`
	StockName          string          `json:"stock_name,omitempty"`
	TradeType          string          `json:"trade_type"`
	Status             string          `json:"status,omitempty"`
	Quantity           float64         `json:"quantity,omitempty"`
	PendingQuantity    float64         `json:"pending_quantity,omitempty"`
	OrderPrice         float64         `json:"order_price,omitempty"`
	OrderPriceKRW      float64         `json:"order_price_krw,omitempty"`
	OrderPriceUSD      float64         `json:"order_price_usd,omitempty"`
	ExecutedQuantity   float64         `json:"executed_quantity,omitempty"`
	AveragePriceKRW    float64         `json:"average_price_krw,omitempty"`
	AveragePriceUSD    float64         `json:"average_price_usd,omitempty"`
	IsAfterMarketOrder bool            `json:"is_after_market_order"`
	IsReservationOrder bool            `json:"is_reservation_order"`
	Raw                json.RawMessage `json:"raw,omitempty"`
}

type PaperBulkCancelReceipt struct {
	Environment    string `json:"environment"`
	RequestedCount int    `json:"requested_count"`
	FailedCount    int    `json:"failed_count"`
}
