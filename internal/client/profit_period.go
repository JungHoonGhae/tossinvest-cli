package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Period-scoped realized profit — the counterpart to GetProfitOverview, which
// is all-time. Both live on wts-cert-api.
//
// The API's `rangeType` is deliberately NOT surfaced to callers. Measured
// against the live service: for identical dates, "day"/"week"/"month"/"year"
// return byte-identical responses, and only "all" differs (it ignores the
// dates). So the parameter is effectively a two-state flag — whole history vs
// an explicit range — and that is what From/To express here. An unrecognised
// value makes the server return 500 rather than a validation error, which is
// the other reason we keep it under our control.

// ProfitTypes are the values profit/type/overview accepts. The server returns
// 400 for anything else, so callers are validated before the request.
var ProfitTypes = []string{"sales", "dividend", "lending", "account-interest"}

// ProfitCurrencies are the values the daily breakdown accepts (uppercase only;
// "krw" and "ALL" are both rejected with 400).
//
// This is the BASIS the returned rates are computed against, NOT a filter.
// Measured live: both values return the identical row set — same dates, same
// symbols, same quantities — but ProfitRate differs on 6 of 8 sampled rows,
// because a KRW basis folds FX movement into a foreign holding's return and a
// USD basis does not. Querying both and merging would therefore duplicate every
// row, which is why exactly one call is made.
var ProfitCurrencies = []string{"KRW", "USD"}

// DefaultProfitCurrency is the basis used when the caller does not choose one.
const DefaultProfitCurrency = "KRW"

// maxDailyProfitPages caps pagination so a looping server response cannot spin
// forever, mirroring maxTransferIncomePages.
const maxDailyProfitPages = 50

const dailyProfitPageSize = 100

type periodProfitRaw struct {
	EarningAmount  dualCurrencyRaw `json:"earningAmount"`
	EarningRate    dualCurrencyRaw `json:"earningRate"`
	PurchaseAmount dualCurrencyRaw `json:"purchaseAmount"`
}

// rangeBody builds the request body shared by the period endpoints. Empty
// from/to means the whole history.
func rangeBody(extra map[string]any, from, to string) ([]byte, error) {
	body := map[string]any{"rangeType": "all"}
	if from != "" || to != "" {
		// Any non-"all" value selects date mode; "month" is what the web app
		// sends, so we match it rather than inventing one.
		body["rangeType"] = "month"
		body["startDate"] = from
		body["endDate"] = to
	}
	for k, v := range extra {
		body[k] = v
	}
	return json.Marshal(body)
}

// GetPeriodProfit fetches realized profit for one profit type. from/to are
// YYYYMMDD; pass both empty for the whole history. WTS-only.
func (c *Client) GetPeriodProfit(ctx context.Context, profitType, from, to string) (domain.PeriodProfit, error) {
	if err := c.requireSession(); err != nil {
		return domain.PeriodProfit{}, err
	}
	body, err := rangeBody(map[string]any{"profitType": profitType}, from, to)
	if err != nil {
		return domain.PeriodProfit{}, err
	}
	var env quoteEnvelope[periodProfitRaw]
	endpoint := c.certBaseURL + "/api/v1/profit/type/overview"
	if err := c.postJSON(ctx, endpoint, body, &env); err != nil {
		return domain.PeriodProfit{}, err
	}
	r := env.Result
	return domain.PeriodProfit{
		Type:           profitType,
		From:           from,
		To:             to,
		EarningAmount:  r.EarningAmount.toDomain(),
		EarningRate:    r.EarningRate.toDomain(),
		PurchaseAmount: r.PurchaseAmount.toDomain(),
		FetchedAt:      time.Now(),
	}, nil
}

type dailyProfitItemRaw struct {
	MarketType       string          `json:"marketType"`
	ProductCode      string          `json:"productCode"`
	Symbol           string          `json:"symbol"`
	ProductName      string          `json:"productName"`
	BaseDate         string          `json:"baseDate"`
	Quantity         float64         `json:"quantity"`
	ProfitLossAmount dualCurrencyRaw `json:"profitLossAmount"`
	ProfitRate       float64         `json:"profitRate"`
	SellAmount       dualCurrencyRaw `json:"sellAmount"`
	BuyAmount        dualCurrencyRaw `json:"buyAmount"`
}

type dailyProfitPageRaw struct {
	Content []dailyProfitItemRaw `json:"content"`
	Last    bool                 `json:"last"`
}

// GetDailyProfit fetches the per-stock realized-profit breakdown over a date
// range, aggregating every page. currency is the basis for the returned rates
// (see ProfitCurrencies); empty means DefaultProfitCurrency. WTS-only.
func (c *Client) GetDailyProfit(ctx context.Context, from, to, currency string) (domain.DailyProfit, error) {
	if err := c.requireSession(); err != nil {
		return domain.DailyProfit{}, err
	}
	if currency == "" {
		currency = DefaultProfitCurrency
	}
	cur := currency
	out := domain.DailyProfit{From: from, To: to, Currency: currency, FetchedAt: time.Now()}
	{
		for page := 0; page < maxDailyProfitPages; page++ {
			body, err := rangeBody(map[string]any{
				"currency": cur,
				"page":     page,
				"size":     dailyProfitPageSize,
			}, from, to)
			if err != nil {
				return domain.DailyProfit{}, err
			}
			var env quoteEnvelope[dailyProfitPageRaw]
			endpoint := c.certBaseURL + "/api/v1/profit/wts/daily/market"
			if err := c.postJSON(ctx, endpoint, body, &env); err != nil {
				return domain.DailyProfit{}, fmt.Errorf("daily profit (%s, page %d): %w", cur, page, err)
			}
			for _, it := range env.Result.Content {
				out.Stocks = append(out.Stocks, domain.DailyProfitStock{
					Date:        formatBaseDate(it.BaseDate),
					MarketType:  it.MarketType,
					Symbol:      it.Symbol,
					Name:        it.ProductName,
					ProductCode: it.ProductCode,
					Quantity:    it.Quantity,
					ProfitLoss:  it.ProfitLossAmount.toDomain(),
					ProfitRate:  it.ProfitRate,
					SellAmount:  it.SellAmount.toDomain(),
					BuyAmount:   it.BuyAmount.toDomain(),
				})
			}
			if env.Result.Last || len(env.Result.Content) == 0 {
				break
			}
		}
	}
	return out, nil
}

// formatBaseDate normalises the row date to YYYY-MM-DD.
//
// The daily endpoint does NOT echo the YYYYMMDD it was queried with — it
// returns a display-formatted "YY.M.D" (e.g. "26.7.15"), with month and day
// unpadded. That is presentation, not data, so we convert it back. The
// YYYYMMDD branch stays because sibling endpoints do use it and costs nothing.
//
// Anything of an unrecognised shape is passed through untouched: a silent
// wrong date is worse than an obviously odd one, and the contract test pins
// both known forms.
func formatBaseDate(s string) string {
	if len(s) == 8 && !strings.Contains(s, ".") {
		return s[:4] + "-" + s[4:6] + "-" + s[6:]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return s
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	d, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return s
	}
	if y < 100 { // two-digit year, as the web app sends
		y += 2000
	}
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return s
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

// ParseProfitRange turns the YYYY-MM-DD dates a human (or an agent) supplies
// into the YYYYMMDD the API wants, enforcing the rules the server states badly:
// a future endDate is answered with an opaque 400, and giving only one side of
// the range silently changes the meaning of the query.
//
// It lives here rather than in the CLI so the cobra commands and the MCP
// operations validate identically — the same policy split across two surfaces
// is what let the conditional-order gate drift (see issue #111).
func ParseProfitRange(from, to string) (string, string, error) {
	parse := func(name, v string) (string, error) {
		if v == "" {
			return "", nil
		}
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return "", fmt.Errorf("%s: %q 는 날짜가 아닙니다 — YYYY-MM-DD 형식으로 주세요", name, v)
		}
		if t.After(time.Now()) {
			return "", fmt.Errorf("%s: %s 는 미래입니다 — 실현손익은 오늘까지만 조회됩니다", name, v)
		}
		return t.Format("20060102"), nil
	}
	f, err := parse("from", from)
	if err != nil {
		return "", "", err
	}
	t, err := parse("to", to)
	if err != nil {
		return "", "", err
	}
	if (f == "") != (t == "") {
		return "", "", fmt.Errorf("from 과 to 는 함께 주세요 (둘 다 없으면 전체 기간)")
	}
	if f != "" && f > t {
		return "", "", fmt.Errorf("from(%s) 이 to(%s) 보다 뒤입니다", from, to)
	}
	return f, t, nil
}
