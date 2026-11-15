package official

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// TransactionTypes are the transactionTypes filter values GET /api/v1/transactions
// accepts (spec 1.2.28). The server rejects any other value with 400, so the
// filter is checked before the request. Responses may still carry values outside
// this list (the spec requires tolerating unknown enums) and pass them through.
var TransactionTypes = []string{
	"BUY", "SELL",
	"SECURITIES_TRANSFER_IN", "SECURITIES_TRANSFER_OUT",
	"DEPOSIT", "WITHDRAWAL",
	"EXCHANGE_IN", "EXCHANGE_OUT",
	"DIVIDEND", "INTEREST", "FEE",
}

const maxTransactionLimit = 100

// TransactionsFilter selects one page of the official transaction ledger.
// Zero values are omitted, which the API reads as "no bound". From and To are
// settlement dates (KST, inclusive), not trade dates; an unsettled trade uses its
// expected settlement date, so a To of today drops trades settling later.
// Changing any filter invalidates Cursor: start again from the first page.
type TransactionsFilter struct {
	Types  []string // OR filter over TransactionTypes; empty = every type
	From   string   // YYYY-MM-DD
	To     string   // YYYY-MM-DD
	Cursor string   // NextCursor from the previous page, passed through untouched
	Limit  int      // rows per page, 1–100; 0 = API default (20)
}

// apiTransaction mirrors the Transaction schema. Every field is required by the
// spec; the pointers are the nullable ones, kept nil instead of "".
type apiTransaction struct {
	TransactionType string  `json:"transactionType"`
	IsCancellation  bool    `json:"isCancellation"`
	Settled         bool    `json:"settled"`
	SettlementDate  string  `json:"settlementDate"`
	TransactionTime *string `json:"transactionTime"`
	MarketCountry   string  `json:"marketCountry"`
	Currency        string  `json:"currency"`
	AssetClass      *string `json:"assetClass"`
	Symbol          *string `json:"symbol"`
	ISINCode        *string `json:"isinCode"`
	Quantity        *string `json:"quantity"`
	FilledPrice     *string `json:"filledPrice"`
	Amount          *string `json:"amount"`
	Commission      string  `json:"commission"`
	Tax             string  `json:"tax"`
	NetAmount       *string `json:"netAmount"`
	ExchangeRate    *string `json:"exchangeRate"`
}

// Transactions fetches one page of the account's transaction ledger: KR and US
// trades, cash movements, FX, dividends, interest, fees and securities
// transfers in a single list, unsettled entries first. It returns a page, not a
// bare slice: a page can be empty while HasNext is true, so only HasNext ends
// paging. Requires the X-Tossinvest-Account header (see WithAccountSeq).
func (c *Client) Transactions(ctx context.Context, filter TransactionsFilter) (domain.LedgerTransactionList, error) {
	q, err := filter.query()
	if err != nil {
		return domain.LedgerTransactionList{}, err
	}
	var raw struct {
		Transactions []apiTransaction `json:"transactions"`
		NextCursor   *string          `json:"nextCursor"`
		HasNext      bool             `json:"hasNext"`
	}
	if err := c.getAcct(ctx, "/api/v1/transactions", q, &raw); err != nil {
		return domain.LedgerTransactionList{}, err
	}
	out := domain.LedgerTransactionList{
		Transactions: make([]domain.LedgerTransaction, 0, len(raw.Transactions)),
		HasNext:      raw.HasNext,
		FetchedAt:    time.Now().UTC(),
	}
	if raw.NextCursor != nil {
		out.NextCursor = *raw.NextCursor
	}
	for _, t := range raw.Transactions {
		out.Transactions = append(out.Transactions, domain.LedgerTransaction{
			TransactionType: t.TransactionType,
			IsCancellation:  t.IsCancellation,
			Settled:         t.Settled,
			SettlementDate:  t.SettlementDate,
			TransactionTime: t.TransactionTime,
			MarketCountry:   t.MarketCountry,
			Currency:        t.Currency,
			AssetClass:      t.AssetClass,
			Symbol:          t.Symbol,
			ISINCode:        t.ISINCode,
			Quantity:        t.Quantity,
			FilledPrice:     t.FilledPrice,
			Amount:          t.Amount,
			Commission:      t.Commission,
			Tax:             t.Tax,
			NetAmount:       t.NetAmount,
			ExchangeRate:    t.ExchangeRate,
		})
	}
	return out, nil
}

// query validates the filter and encodes it. Every check here mirrors a 400 the
// spec documents, so a bad flag fails locally instead of spending a request from
// the TRANSACTION rate-limit group.
func (f TransactionsFilter) query() (url.Values, error) {
	q := url.Values{}
	if len(f.Types) > 0 {
		types := make([]string, 0, len(f.Types))
		for _, t := range f.Types {
			v := strings.ToUpper(strings.TrimSpace(t))
			if v == "" {
				return nil, fmt.Errorf("transaction types must not contain an empty value")
			}
			if !slices.Contains(TransactionTypes, v) {
				return nil, fmt.Errorf("unsupported transaction type %q (allowed: %s)", t, strings.Join(TransactionTypes, ", "))
			}
			types = append(types, v)
		}
		// style=form, explode=false: one comma-separated value.
		q.Set("transactionTypes", strings.Join(types, ","))
	}
	from, err := parseSettlementDate("from", f.From)
	if err != nil {
		return nil, err
	}
	to, err := parseSettlementDate("to", f.To)
	if err != nil {
		return nil, err
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return nil, fmt.Errorf("from (%s) must not be after to (%s)", f.From, f.To)
	}
	if f.From != "" {
		q.Set("from", f.From)
	}
	if f.To != "" {
		q.Set("to", f.To)
	}
	if f.Cursor != "" {
		q.Set("cursor", f.Cursor)
	}
	if f.Limit < 0 || f.Limit > maxTransactionLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxTransactionLimit, f.Limit)
	}
	if f.Limit > 0 {
		q.Set("limit", strconv.Itoa(f.Limit))
	}
	return q, nil
}

func parseSettlementDate(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD, got %q", name, value)
	}
	return t, nil
}
