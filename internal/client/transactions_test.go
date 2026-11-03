package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestListTransactionsParsesTradeAndCashEntries(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/my-assets/transactions/markets/kr" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("range.from") != "2024-01-01" {
			t.Fatalf("range.from = %q", q.Get("range.from"))
		}
		if q.Get("range.to") != "2024-01-15" {
			t.Fatalf("range.to = %q", q.Get("range.to"))
		}
		if q.Get("filters") != "0" {
			t.Fatalf("filters = %q", q.Get("filters"))
		}
		if q.Get("size") != "50" {
			t.Fatalf("size = %q", q.Get("size"))
		}
		_, _ = w.Write([]byte(`{
			"result": {
				"pagingParam": {"number": 0, "size": 50, "key": "abc", "range": "", "filters": "0", "type": ""},
				"body": [
					{
						"type": "1",
						"transactionType": {"code": "5", "displayName": "매수"},
						"displayType": "50",
						"summary": null,
						"stockCode": "A999001",
						"stockName": "샘플종목1",
						"productName": "샘플종목1",
						"quantity": 10.0,
						"amount": 1000000.0,
						"adjustedAmount": -1000000.0,
						"commissionAmount": 0.0,
						"totalTaxAmount": 0.0,
						"date": "2024-01-15",
						"settlementDate": "2024-01-17",
						"compositeKey": {"orderDate": "2024-01-15", "tradeType": "buy", "stockCode": "A999001", "assetIoType": null, "id": null}
					},
					{
						"type": "2",
						"transactionType": {"code": "1", "displayName": "입금"},
						"displayType": "13",
						"summary": "배당금입금",
						"stockCode": "A999002",
						"stockName": "샘플종목2",
						"quantity": 0.0,
						"amount": 10000.0,
						"adjustedAmount": 8000.0,
						"commissionAmount": 0.0,
						"totalTaxAmount": 2000.0,
						"balanceAmount": 500000.0,
						"cancelTradeYn": false,
						"summaryNo": "0001",
						"tradeTypeName": "배당금입금",
						"dateTime": "2024-01-15 10:00:00.000",
						"referenceType": null,
						"compositeKey": {"date": "2024-01-15", "no": 1}
					}
				],
				"lastPage": true,
				"allAsset": true
			}
		}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
		},
	})

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, KoreaLocation)
	to := time.Date(2024, 1, 15, 0, 0, 0, 0, KoreaLocation)
	page, err := client.ListTransactions(context.Background(), "kr", from, to, "all", 0, 0)
	if err != nil {
		t.Fatalf("ListTransactions returned error: %v", err)
	}

	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
	if !page.LastPage {
		t.Fatal("expected lastPage=true")
	}

	trade := page.Items[0]
	if trade.Category != "trade" {
		t.Fatalf("first item category = %q, want trade", trade.Category)
	}
	if trade.DisplayName != "매수" {
		t.Fatalf("trade displayName = %q", trade.DisplayName)
	}
	if trade.AdjustedAmount != -1000000.0 {
		t.Fatalf("trade adjusted = %v", trade.AdjustedAmount)
	}
	if trade.Currency != "KRW" {
		t.Fatalf("trade currency = %q", trade.Currency)
	}
	if trade.SettlementDate != "2024-01-17" {
		t.Fatalf("trade settlement = %q", trade.SettlementDate)
	}
	if trade.TradeType != "buy" {
		t.Fatalf("trade tradeType = %q", trade.TradeType)
	}

	cash := page.Items[1]
	if cash.Category != "cash" {
		t.Fatalf("second item category = %q, want cash", cash.Category)
	}
	if cash.Summary != "배당금입금" {
		t.Fatalf("cash summary = %q", cash.Summary)
	}
	if cash.BalanceAmount != 500000.0 {
		t.Fatalf("cash balance = %v", cash.BalanceAmount)
	}
	if cash.DateTime != "2024-01-15 10:00:00.000" {
		t.Fatalf("cash datetime = %q", cash.DateTime)
	}
}

func TestListTransactionsRejectsOversizeRange(t *testing.T) {
	t.Parallel()
	client := New(Config{
		Session: &session.Session{Cookies: map[string]string{"x": "y"}},
	})
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := client.ListTransactions(context.Background(), "kr", from, to, "all", 0, 0)
	if err == nil {
		t.Fatal("expected error for oversize range")
	}
}

func TestListTransactionsRejectsInvalidMarket(t *testing.T) {
	t.Parallel()
	client := New(Config{
		Session: &session.Session{Cookies: map[string]string{"x": "y"}},
	})
	_, err := client.ListTransactions(context.Background(), "jp", time.Time{}, time.Time{}, "all", 0, 0)
	if err == nil {
		t.Fatal("expected error for unsupported market")
	}
}

func TestListAllTransactionsPagesUntilLastPage(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		to := r.URL.Query().Get("range.to")
		switch to {
		case "2026-04-19":
			// First page: oldest item on 2026-04-01 — client should narrow range.to to 2026-04-01 next.
			_, _ = w.Write([]byte(`{"result":{"pagingParam":{"number":0,"size":2},"body":[
				{"type":"1","transactionType":{"code":"5","displayName":"매수"},"stockCode":"A","stockName":"A Inc","amount":100,"adjustedAmount":-100,"date":"2026-04-01","compositeKey":{"orderDate":"2026-04-01","tradeType":"buy","id":111}}
			],"lastPage":false}}`))
		case "2026-04-01":
			_, _ = w.Write([]byte(`{"result":{"pagingParam":{"number":0,"size":2},"body":[
				{"type":"2","transactionType":{"code":"1","displayName":"입금"},"stockCode":"B","stockName":"B Inc","amount":50,"adjustedAmount":50,"dateTime":"2026-03-15 10:00:00.000","compositeKey":{"date":"2026-03-15","no":2}}
			],"lastPage":true}}`))
		default:
			t.Fatalf("unexpected range.to=%s (call #%d)", to, calls)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "x"}},
	})

	items, err := client.ListAllTransactions(
		context.Background(), "kr",
		time.Date(2026, 1, 1, 0, 0, 0, 0, KoreaLocation),
		time.Date(2026, 4, 19, 0, 0, 0, 0, KoreaLocation),
		"all", 0, 5,
	)
	if err != nil {
		t.Fatalf("ListAllTransactions error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 page fetches, got %d", calls)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Category != "trade" || items[1].Category != "cash" {
		t.Fatalf("unexpected categories: %v / %v", items[0].Category, items[1].Category)
	}
}

func TestGetTransactionsOverviewParsesBuckets(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/my-assets/transactions/markets/kr/overview" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":{
			"orderableAmount":{"krw":100000,"usd":null},
			"withdrawableAmount":{
				"amount0":{"krw":100000,"usd":null},"date0":"2024-01-15",
				"amount1":{"krw":100000,"usd":null},"date1":"2024-01-16",
				"amount2":null,"date2":null,
				"amount3":null,"date3":null
			},
			"depositAmount":{
				"amount0":{"krw":1000,"usd":null},"date0":"2024-01-17",
				"amount1":null,"date1":null,
				"amount2":null,"date2":null,
				"amount3":null,"date3":null
			},
			"estimateSettlementAmount":{
				"day1":{"settlementKorDate":"2024-01-16","buyAmount":1000,"sellAmount":0},
				"day2":{"settlementKorDate":"2024-01-17","buyAmount":0,"sellAmount":2000}
			},
			"withdrawableAmountBottomSheet":[
				{"title":"출금가능금액","amount":{"krw":100000,"usd":null}}
			]
		}}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "x"}},
	})

	ov, err := client.GetTransactionsOverview(context.Background(), "kr")
	if err != nil {
		t.Fatalf("GetTransactionsOverview error: %v", err)
	}
	if ov.OrderableKRW != 100000 {
		t.Fatalf("orderableKRW = %v", ov.OrderableKRW)
	}
	if len(ov.Withdrawable) != 2 {
		t.Fatalf("withdrawable buckets = %d (want 2)", len(ov.Withdrawable))
	}
	if len(ov.Deposit) != 1 {
		t.Fatalf("deposit buckets = %d (want 1)", len(ov.Deposit))
	}
	if len(ov.EstimateSettlement) != 2 {
		t.Fatalf("settlement = %d", len(ov.EstimateSettlement))
	}
	if ov.EstimateSettlement[1].SellAmount != 2000 {
		t.Fatalf("settlement day2 sell = %v", ov.EstimateSettlement[1].SellAmount)
	}
	if len(ov.WithdrawableBottomSheet) != 1 || ov.WithdrawableBottomSheet[0].Title != "출금가능금액" {
		t.Fatalf("bottom sheet parse failed: %+v", ov.WithdrawableBottomSheet)
	}
}

func TestExtractTransactionDatePrefersOrderDateOverSettlement(t *testing.T) {
	t.Parallel()
	// US type=1 trade records leave date/dateTime null; only orderDate (past)
	// and settlementDate (future T+2) are set. The range filter must use
	// orderDate so a trade placed on 2026-04-17 isn't excluded from --to 2026-04-19.
	tx := domain.Transaction{
		OrderDate:      "2026-04-17",
		SettlementDate: "2026-04-21",
	}
	got := extractTransactionDate(tx)
	want := time.Date(2026, 4, 17, 0, 0, 0, 0, KoreaLocation)
	if !got.Equal(want) {
		t.Fatalf("extractTransactionDate = %v, want %v (OrderDate must win over SettlementDate)", got, want)
	}
}

func TestListTransactionsFiltersOutOfRangeItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{
			"pagingParam":{"number":0,"size":50,"key":"","filters":"0","type":""},
			"body":[
				{"type":"1","transactionType":{"code":"5","displayName":"매수"},"stockCode":"A999001","amount":200,"adjustedAmount":-200,"date":"2024-02-01"},
				{"type":"1","transactionType":{"code":"5","displayName":"매수"},"stockCode":"A999002","amount":100,"adjustedAmount":-100,"date":"2024-01-10"}
			],
			"lastPage":true
		}}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "x"}},
	})

	page, err := client.ListTransactions(
		context.Background(), "kr",
		time.Date(2024, 1, 15, 0, 0, 0, 0, KoreaLocation),
		time.Date(2024, 2, 28, 0, 0, 0, 0, KoreaLocation),
		"all", 0, 0,
	)
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item after range filter (Toss ignores range.from), got %d", len(page.Items))
	}
	if page.Items[0].StockCode != "A999001" {
		t.Fatalf("expected to keep 2024-02-01 item, got StockCode=%q", page.Items[0].StockCode)
	}
}

func TestListTransactionsUnwrapsPagingScalars(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{
			"pagingParam":{"number":0,"size":50,"key":"abc","filters":"0","type":""},
			"body":[{"type":"1","transactionType":{"code":"5","displayName":"매수"},"amount":1,"adjustedAmount":-1,"date":"2026-04-17"}],
			"lastPage":false
		}}`))
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "x"}},
	})

	page, err := client.ListTransactions(
		context.Background(), "kr",
		time.Date(2026, 4, 1, 0, 0, 0, 0, KoreaLocation),
		time.Date(2026, 4, 19, 0, 0, 0, 0, KoreaLocation),
		"all", 0, 0,
	)
	if err != nil {
		t.Fatalf("ListTransactions error: %v", err)
	}
	if page.Next == nil {
		t.Fatal("expected Next pagination cursor")
	}
	if page.Next.Filters != "0" {
		t.Fatalf("Next.Filters = %q, want %q (no JSON quoting)", page.Next.Filters, "0")
	}
	if page.Next.Type != "" {
		t.Fatalf("Next.Type = %q, want empty", page.Next.Type)
	}
}

func TestRawJSONScalarToString(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		``:        "",
		`null`:    "",
		`"0"`:     "0",
		`"hello"`: "hello",
		`42`:      "42",
	}
	for in, want := range cases {
		got := rawJSONScalarToString([]byte(in))
		if got != want {
			t.Fatalf("rawJSONScalarToString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTransactionFilter(t *testing.T) {
	t.Parallel()
	cases := map[string]TransactionFilter{
		"":         TransactionFilterAll,
		"all":      TransactionFilterAll,
		"trade":    TransactionFilterTrades,
		"trades":   TransactionFilterTrades,
		"cash":     TransactionFilterCash,
		"inout":    TransactionFilterInOut,
		"transfer": TransactionFilterInOut,
		"cash-alt": TransactionFilterCashAlt,
		"cashalt":  TransactionFilterCashAlt,
		"0":        TransactionFilterAll,
		"1":        TransactionFilterTrades,
		"2":        TransactionFilterCash,
		"3":        TransactionFilterInOut,
		"6":        TransactionFilterCashAlt,
	}
	for in, want := range cases {
		got, err := parseTransactionFilter(in)
		if err != nil {
			t.Fatalf("parseTransactionFilter(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("parseTransactionFilter(%q) = %v, want %v", in, got, want)
		}
	}

	if _, err := parseTransactionFilter("42"); err == nil {
		t.Fatal("expected error for unsupported numeric")
	}
	if _, err := parseTransactionFilter("garbage"); err == nil {
		t.Fatal("expected error for unknown name")
	}
}
