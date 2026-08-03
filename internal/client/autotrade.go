package client

import (
	"context"
	"strconv"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Automated trading rules (자동매매) — stop-loss / target-profit / OCO / OTO
// rules the user armed in Toss's own UI.
//
// GET /api/v3/trading/auto-trading/histories on the info host.
//
// This is read-only on purpose. Arming or cancelling a rule is a write against
// the account and tossctl deliberately does not expose those (same stance as
// `account detail`, which shows the 계좌관리 screen but none of its actions).

// autoTradeStatuses maps the wire status code to its enum name. The wire value
// is a bare number ("6"), which tells a reader nothing. The mapping was read
// out of Toss's web bundle, not inferred from observed values — inventing it
// would risk showing an armed rule as finished.
//
// The bundle also carries a Korean label map, but it collapses EXPIRED,
// DELETED and COMPLETED all to "켬" (on) because it drives a toggle switch, not
// a list. Using it here would report a deleted rule as active, so the enum
// names are surfaced instead.
var autoTradeStatuses = map[string]string{
	"1":  "READY",
	"2":  "PAUSED",
	"3":  "FORCE_PAUSED",
	"4":  "PROGRESSED",
	"5":  "ORDERED",
	"6":  "EXPIRED",
	"7":  "DELETED",
	"8":  "COMPLETED",
	"10": "ORDERING",
	"11": "RETRY",
	"12": "HOLDING",
	"19": "PARTIAL_FILLED",
}

type autoTradeRaw struct {
	ID             int64  `json:"id"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	ProductCode    string `json:"productCode"`
	ProductName    string `json:"productName"`
	MarketDivision string `json:"marketDivision"`
	CreatedAt      string `json:"createdAt"`
	Settings       []struct {
		Quantity struct {
			Amount   float64 `json:"amount"`
			AllTrade bool    `json:"allTrade"`
		} `json:"quantity"`
		Target struct {
			Price        float64 `json:"price"`
			CurrencyType string  `json:"currencyType"`
		} `json:"target"`
		Order struct {
			Price     float64 `json:"price"`
			TradeType string  `json:"tradeType"`
		} `json:"order"`
	} `json:"settings"`
}

type autoTradePageRaw struct {
	Body     []autoTradeRaw `json:"body"`
	LastPage bool           `json:"lastPage"`
}

// ListAutoTrades returns the account's automated-trading rules. WTS-only.
func (c *Client) ListAutoTrades(ctx context.Context) (domain.AutoTradeList, error) {
	if err := c.requireSession(); err != nil {
		return domain.AutoTradeList{}, err
	}

	var env quoteEnvelope[autoTradePageRaw]
	url := c.infoBaseURL + "/api/v3/trading/auto-trading/histories"
	if err := c.getJSON(ctx, url, &env); err != nil {
		return domain.AutoTradeList{}, err
	}

	out := domain.AutoTradeList{
		Items:     make([]domain.AutoTrade, 0, len(env.Result.Body)),
		HasNext:   !env.Result.LastPage,
		FetchedAt: time.Now(),
	}
	for _, r := range env.Result.Body {
		item := domain.AutoTrade{
			ID:         r.ID,
			Type:       r.Type,
			Status:     autoTradeStatuses[r.Status],
			StatusCode: r.Status,
			Symbol:     r.ProductCode,
			Name:       r.ProductName,
			Market:     r.MarketDivision,
			CreatedAt:  r.CreatedAt,
		}
		if item.Status == "" {
			// An unmapped code is still worth showing — Toss adding a state
			// must not make the rule disappear from the list.
			item.Status = "UNKNOWN(" + strconv.Quote(r.Status) + ")"
		}
		if len(r.Settings) > 0 {
			s := r.Settings[0]
			item.Quantity = s.Quantity.Amount
			item.AllQuantity = s.Quantity.AllTrade
			item.TriggerPrice = s.Target.Price
			item.OrderPrice = s.Order.Price
			item.Currency = s.Target.CurrencyType
			item.TradeType = s.Order.TradeType
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}
