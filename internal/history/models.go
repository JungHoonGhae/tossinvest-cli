// Package history stores explicit, read-only WTS observations for offline use.
// It never supplies cached prices, balances, or order state to trading.
package history

import "github.com/JungHoonGhae/tossinvest-cli/internal/domain"

type Snapshot struct {
	ID               int64    `json:"id"`
	CollectedAt      string   `json:"collected_at"`
	Source           string   `json:"source"`
	From             string   `json:"from"`
	To               string   `json:"to"`
	Markets          []string `json:"markets"`
	Complete         bool     `json:"complete"`
	Warnings         []string `json:"warnings"`
	PositionCount    int      `json:"position_count"`
	TransactionCount int      `json:"transaction_count"`
}

type Data struct {
	Snapshot     Snapshot          `json:"snapshot"`
	Positions    []domain.Position `json:"positions"`
	Transactions []Transaction     `json:"transactions"`
}

type Holdings struct {
	Snapshot  Snapshot          `json:"snapshot"`
	Positions []domain.Position `json:"positions"`
}

// Transaction deliberately excludes raw upstream payloads and account IDs.
// Amounts retain their original currency; different currencies are never summed.
type Transaction struct {
	Key            string  `json:"key"`
	Date           string  `json:"date"`
	DateTime       string  `json:"datetime"`
	Market         string  `json:"market"`
	Currency       string  `json:"currency"`
	Category       string  `json:"category"`
	Type           string  `json:"type"`
	StockCode      string  `json:"stock_code"`
	StockName      string  `json:"stock_name"`
	Summary        string  `json:"summary"`
	Quantity       float64 `json:"quantity"`
	Amount         float64 `json:"amount"`
	AdjustedAmount float64 `json:"adjusted_amount"`
	Commission     float64 `json:"commission"`
	Tax            float64 `json:"tax"`
}

type Change struct {
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	ProductCode    string  `json:"product_code"`
	Status         string  `json:"status"`
	QuantityBefore float64 `json:"quantity_before"`
	QuantityAfter  float64 `json:"quantity_after"`
	QuantityDelta  float64 `json:"quantity_delta"`
	ValueDeltaKRW  float64 `json:"value_delta_krw"`
	ValueDeltaUSD  float64 `json:"value_delta_usd"`
	PnLDeltaKRW    float64 `json:"unrealized_pnl_delta_krw"`
}

type Comparison struct {
	Before  Snapshot `json:"before"`
	After   Snapshot `json:"after"`
	Changes []Change `json:"changes"`
	Note    string   `json:"note"`
}

type Query struct {
	SnapshotID int64
	Search     string
	From       string
	To         string
	Market     string
	Limit      int
	Offset     int
}

type Transactions struct {
	Snapshot   Snapshot      `json:"snapshot"`
	Items      []Transaction `json:"items"`
	HasMore    bool          `json:"has_more"`
	NextOffset int           `json:"next_offset"`
}
