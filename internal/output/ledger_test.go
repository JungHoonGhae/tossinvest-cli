package output

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func strPtr(s string) *string { return &s }

// Dummy rows only — no real account data.
func ledgerFixture() domain.LedgerTransactionList {
	return domain.LedgerTransactionList{
		Transactions: []domain.LedgerTransaction{
			{
				TransactionType: "BUY", Settled: false, SettlementDate: "2026-09-03",
				MarketCountry: "KR", Currency: "KRW", AssetClass: strPtr("STOCK"), Symbol: strPtr("000001"),
				Quantity: strPtr("10"), Amount: strPtr("1234567"), Commission: "185", Tax: "0", NetAmount: strPtr("1234752"),
			},
			{
				TransactionType: "DEPOSIT", Settled: true, SettlementDate: "2026-09-01",
				TransactionTime: strPtr("2026-09-01T06:30:00+09:00"), MarketCountry: "US", Currency: "USD",
				AssetClass: strPtr("BOND"), ISINCode: strPtr("US0000000002"),
				Amount: strPtr("1000.50"), Commission: "0", Tax: "0", NetAmount: strPtr("1000.50"), ExchangeRate: strPtr("1350.10"),
			},
			{
				TransactionType: "DEPOSIT", IsCancellation: true, Settled: true, SettlementDate: "2026-08-30",
				MarketCountry: "KR", Currency: "KRW", Amount: strPtr("20000"), Commission: "0", Tax: "0", NetAmount: strPtr("20000"),
			},
			{
				TransactionType: "FUTURE_KIND", Settled: true, SettlementDate: "2026-08-28",
				MarketCountry: "KR", Currency: "KRW", Commission: "0", Tax: "0",
			},
		},
		NextCursor: "dummyCursor_2-x",
		HasNext:    true,
	}
}

func TestWriteLedgerTransactionsTable(t *testing.T) {
	defer setTestLang(t)()
	i18n.SetLang("ko")
	var buf bytes.Buffer
	if err := WriteLedgerTransactions(&buf, FormatTable, ledgerFixture()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"거래 내역: 4건",
		"미결제", "결제",
		"1,234,567", "1,234,752", // integer part grouped
		"1,000.50",     // fraction kept exactly as sent
		"US0000000002", // bond: ISIN stands in for the missing symbol
		"입금 취소",        // cancellation keeps the original type
		"FUTURE_KIND",  // unknown enum shown raw
		"다음 페이지: --cursor dummyCursor_2-x",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "output.ledger.") {
		t.Errorf("table leaked a raw i18n key:\n%s", out)
	}
}

func TestWriteLedgerTransactionsEmptyPages(t *testing.T) {
	defer setTestLang(t)()
	i18n.SetLang("en")
	var buf bytes.Buffer
	if err := WriteLedgerTransactions(&buf, FormatTable, domain.LedgerTransactionList{Transactions: []domain.LedgerTransaction{}, NextCursor: "more", HasNext: true}); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "more pages follow") || !strings.Contains(out, "--cursor more") {
		t.Errorf("an empty page with has_next must not read as the end:\n%s", out)
	}

	buf.Reset()
	if err := WriteLedgerTransactions(&buf, FormatTable, domain.LedgerTransactionList{Transactions: []domain.LedgerTransaction{}}); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "No transactions match") || strings.Contains(out, "--cursor") {
		t.Errorf("last empty page:\n%s", out)
	}
}

func TestWriteLedgerTransactionsCSVKeepsDecimalsVerbatim(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLedgerTransactions(&buf, FormatCSV, ledgerFixture()); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 5 {
		t.Fatalf("want header + 4 rows, got %d", len(records))
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[h] = i
	}
	bond := records[2]
	if bond[col["amount"]] != "1000.50" || bond[col["exchange_rate"]] != "1350.10" || bond[col["symbol"]] != "" {
		t.Errorf("bond row = %v", bond)
	}
	if records[1][col["transaction_time"]] != "" || records[1][col["settled"]] != "false" {
		t.Errorf("null time must be empty and settled explicit: %v", records[1])
	}
	if records[3][col["is_cancellation"]] != "true" {
		t.Errorf("cancellation row = %v", records[3])
	}
}

func TestLedgerDecimal(t *testing.T) {
	for in, want := range map[string]string{
		"0": "0", "999": "999", "1000": "1,000", "1234567.890": "1,234,567.890",
		".5": ".5", "-1000": "-1000", "1e3": "1e3", "": "-",
		"123456789012345678901234567890": "123456789012345678901234567890",
	} {
		v := in
		if got := ledgerDecimal(&v); got != want {
			t.Errorf("ledgerDecimal(%q) = %q, want %q", in, got, want)
		}
	}
	if got := ledgerDecimal(nil); got != "-" {
		t.Errorf("ledgerDecimal(nil) = %q", got)
	}
}
