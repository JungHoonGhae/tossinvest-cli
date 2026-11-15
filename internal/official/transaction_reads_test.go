package official

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

// Dummy ledger rows (not real account data) covering the shapes the 1.2.28
// spec documents: an unsettled KR buy with null price/time/rate, a settled US
// option sell, a bond deposit identified only by ISIN, a cash withdrawal with no
// instrument, a KR deposit cancellation, a securities transfer with null
// amounts, and types/asset classes newer than the spec.
const ledgerPage = `{"result":{"transactions":[
	{"transactionType":"BUY","isCancellation":false,"settled":false,"settlementDate":"2026-09-03",
	 "transactionTime":null,"marketCountry":"KR","currency":"KRW","assetClass":"STOCK","symbol":"000001",
	 "isinCode":"KR0000000001","quantity":"10","filledPrice":null,"amount":"1234567","commission":"185",
	 "tax":"0","netAmount":"1234752","exchangeRate":null},
	{"transactionType":"SELL","isCancellation":false,"settled":true,"settlementDate":"2026-09-02",
	 "transactionTime":"2026-09-02T06:30:00+09:00","marketCountry":"US","currency":"USD","assetClass":"OPTION",
	 "symbol":"DUMY  260918C00100000","isinCode":null,"quantity":"2","filledPrice":"1.235","amount":"247.00",
	 "commission":"1.30","tax":"0.01","netAmount":"245.69","exchangeRate":"1350.10"},
	{"transactionType":"DEPOSIT","isCancellation":false,"settled":true,"settlementDate":"2026-09-01",
	 "transactionTime":"2026-09-01T06:30:00+09:00","marketCountry":"US","currency":"USD","assetClass":"BOND",
	 "symbol":null,"isinCode":"US0000000002","quantity":null,"filledPrice":null,"amount":"100.00",
	 "commission":"0","tax":"0","netAmount":"100.00","exchangeRate":"1350.10"},
	{"transactionType":"WITHDRAWAL","isCancellation":false,"settled":true,"settlementDate":"2026-08-31",
	 "transactionTime":"2026-08-31T10:00:00+09:00","marketCountry":"KR","currency":"KRW","assetClass":null,
	 "symbol":null,"isinCode":null,"quantity":null,"filledPrice":null,"amount":"50000","commission":"0",
	 "tax":"0","netAmount":"50000","exchangeRate":null},
	{"transactionType":"DEPOSIT","isCancellation":true,"settled":true,"settlementDate":"2026-08-30",
	 "transactionTime":"2026-08-30T11:00:00+09:00","marketCountry":"KR","currency":"KRW","assetClass":null,
	 "symbol":null,"isinCode":null,"quantity":null,"filledPrice":null,"amount":"20000","commission":"0",
	 "tax":"0","netAmount":"20000","exchangeRate":null},
	{"transactionType":"SECURITIES_TRANSFER_IN","isCancellation":false,"settled":true,"settlementDate":"2026-08-29",
	 "transactionTime":"2026-08-29T09:00:00+09:00","marketCountry":"KR","currency":"KRW","assetClass":"STOCK",
	 "symbol":"000003","isinCode":"KR0000000003","quantity":"5","filledPrice":null,"amount":null,"commission":"0",
	 "tax":"0","netAmount":null,"exchangeRate":null},
	{"transactionType":"FUTURE_KIND","isCancellation":false,"settled":true,"settlementDate":"2026-08-28",
	 "transactionTime":"2026-08-28T09:00:00+09:00","marketCountry":"KR","currency":"KRW","assetClass":"FUND",
	 "symbol":null,"isinCode":null,"quantity":null,"filledPrice":null,"amount":"1","commission":"0",
	 "tax":"0","netAmount":"1","exchangeRate":null}
],"nextCursor":"dummyCursor_2-x","hasNext":true}}`

func newLedgerTestClient(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) (*Client, *int) {
	t.Helper()
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/transactions":
			requests++
			handle(w, r)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(7),
	)
	return c, &requests
}

func TestTransactionsDecodesLedgerContract(t *testing.T) {
	var gotQuery url.Values
	var rawQuery string
	c, _ := newLedgerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Tossinvest-Account"); got != "7" {
			t.Errorf("account header = %q, want 7", got)
		}
		gotQuery, rawQuery = r.URL.Query(), r.URL.RawQuery
		_, _ = w.Write([]byte(ledgerPage))
	})

	page, err := c.Transactions(context.Background(), TransactionsFilter{
		Types: []string{"buy", " SELL "}, From: "2026-08-01", To: "2026-09-30", Cursor: "prev_Cursor-1", Limit: 50,
	})
	if err != nil {
		t.Fatalf("Transactions: %v", err)
	}
	// explode=false: one comma-joined value, normalized to the enum spelling.
	if got := gotQuery["transactionTypes"]; len(got) != 1 || got[0] != "BUY,SELL" {
		t.Errorf("transactionTypes = %q (raw %q), want one BUY,SELL value", got, rawQuery)
	}
	for k, want := range map[string]string{"from": "2026-08-01", "to": "2026-09-30", "cursor": "prev_Cursor-1", "limit": "50"} {
		if got := gotQuery.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	if !page.HasNext || page.NextCursor != "dummyCursor_2-x" || len(page.Transactions) != 7 {
		t.Fatalf("page = has_next %v cursor %q rows %d", page.HasNext, page.NextCursor, len(page.Transactions))
	}
	buy := page.Transactions[0]
	if buy.Settled || buy.TransactionTime != nil || buy.FilledPrice != nil || buy.ExchangeRate != nil {
		t.Errorf("unsettled nulls must stay nil: %+v", buy)
	}
	if buy.NetAmount == nil || *buy.NetAmount != "1234752" || buy.Commission != "185" {
		t.Errorf("unsettled amounts: %+v", buy)
	}
	option := page.Transactions[1]
	if *option.AssetClass != "OPTION" || *option.Symbol != "DUMY  260918C00100000" || option.ISINCode != nil {
		t.Errorf("option identifiers: %+v", option)
	}
	// Decimal strings survive verbatim: no float rounding or trailing-zero loss.
	if *option.FilledPrice != "1.235" || *option.Amount != "247.00" || *option.NetAmount != "245.69" ||
		option.Commission != "1.30" || option.Tax != "0.01" || *option.ExchangeRate != "1350.10" {
		t.Errorf("decimal strings must not be reformatted: %+v", option)
	}
	bond := page.Transactions[2]
	if bond.Symbol != nil || bond.ISINCode == nil || *bond.ISINCode != "US0000000002" {
		t.Errorf("bond is identified by ISIN only: %+v", bond)
	}
	cash := page.Transactions[3]
	if cash.AssetClass != nil || cash.Symbol != nil || cash.ISINCode != nil || cash.Quantity != nil {
		t.Errorf("cash row has no instrument: %+v", cash)
	}
	if cancel := page.Transactions[4]; !cancel.IsCancellation || cancel.TransactionType != "DEPOSIT" {
		t.Errorf("cancellation keeps the original type: %+v", cancel)
	}
	if transfer := page.Transactions[5]; transfer.Amount != nil || transfer.NetAmount != nil || *transfer.Quantity != "5" {
		t.Errorf("securities transfer has null amounts and a quantity: %+v", transfer)
	}
	if unknown := page.Transactions[6]; unknown.TransactionType != "FUTURE_KIND" || *unknown.AssetClass != "FUND" {
		t.Errorf("unknown enums must pass through: %+v", unknown)
	}

	// JSON keeps nulls as null so "absent" and "empty" stay distinguishable.
	b, err := json.Marshal(page.Transactions[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"transaction_time":null`, `"filled_price":null`, `"exchange_rate":null`, `"net_amount":"1234752"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("JSON %s lacks %s", b, want)
		}
	}
}

func TestTransactionsEmptyPageKeepsPaging(t *testing.T) {
	body := `{"result":{"transactions":[],"nextCursor":"more","hasNext":true}}`
	c, _ := newLedgerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Query()) != 0 {
			t.Errorf("zero filter must send no query, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(body))
	})
	page, err := c.Transactions(context.Background(), TransactionsFilter{})
	if err != nil {
		t.Fatalf("Transactions: %v", err)
	}
	if !page.HasNext || page.NextCursor != "more" {
		t.Errorf("an empty page must still report the next page: %+v", page)
	}
	if page.Transactions == nil {
		t.Error("empty list must encode as [], not null")
	}

	body = `{"result":{"transactions":[],"nextCursor":null,"hasNext":false}}`
	page, err = c.Transactions(context.Background(), TransactionsFilter{})
	if err != nil || page.HasNext || page.NextCursor != "" {
		t.Errorf("last page = %+v, %v", page, err)
	}
}

func TestTransactionsValidatesBeforeRequest(t *testing.T) {
	c, requests := newLedgerTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"transactions":[],"nextCursor":null,"hasNext":false}}`))
	})
	ctx := context.Background()
	for _, ok := range []TransactionsFilter{
		{Limit: 1}, {Limit: 100}, {From: "2026-09-01", To: "2026-09-01"}, {To: "2099-12-31"},
		{Types: TransactionTypes},
	} {
		if _, err := c.Transactions(ctx, ok); err != nil {
			t.Errorf("%+v rejected: %v", ok, err)
		}
	}
	accepted := *requests
	for name, bad := range map[string]TransactionsFilter{
		"limit below range": {Limit: -1},
		"limit above range": {Limit: 101},
		"empty type":        {Types: []string{"BUY", "", "SELL"}},
		"unknown type":      {Types: []string{"TRANSFER"}},
		"from after to":     {From: "2026-09-02", To: "2026-09-01"},
		"bad from":          {From: "2026/09/01"},
		"bad to":            {To: "20260901"},
	} {
		if _, err := c.Transactions(ctx, bad); err == nil {
			t.Errorf("%s: %+v accepted", name, bad)
		}
	}
	if *requests != accepted {
		t.Errorf("invalid filters reached the server: %d requests, want %d", *requests, accepted)
	}
}

func TestTransactionsReturnsAPIErrors(t *testing.T) {
	c, _ := newLedgerTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid-request","message":"dummy"}}`))
	})
	_, err := c.Transactions(context.Background(), TransactionsFilter{Cursor: "stale"})
	var apiErr *APIError
	if err == nil || !errors.As(err, &apiErr) || apiErr.Code != http.StatusBadRequest {
		t.Fatalf("want a 400 APIError passthrough, got %v", err)
	}
}
