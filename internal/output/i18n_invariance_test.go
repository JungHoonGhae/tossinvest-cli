package output

import (
	"bytes"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// TestJSONOutputLanguageInvariant is the safety net for the localization work
// in this package: --json output must be byte-for-byte identical regardless
// of the active locale, because it is produced via encoding/json on domain
// structs with static `json:"..."` tags that never route through i18n.T.
// If a future change accidentally sends a JSON field through i18n.T, this
// test fails.
func TestJSONOutputLanguageInvariant(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	renderers := []struct {
		name   string
		render func(w *bytes.Buffer) error
	}{
		{
			name: "positions",
			render: func(w *bytes.Buffer) error {
				return WritePositions(w, FormatJSON, testPositions)
			},
		},
		{
			name: "quote",
			render: func(w *bytes.Buffer) error {
				return WriteQuote(w, FormatJSON, testQuote)
			},
		},
		{
			name: "orders",
			render: func(w *bytes.Buffer) error {
				return WriteOrders(w, FormatJSON, testInvarianceOrders)
			},
		},
		{
			name: "watchlist",
			render: func(w *bytes.Buffer) error {
				return WriteWatchlist(w, FormatJSON, testWatchlistItems)
			},
		},
	}

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			var enBuf, koBuf bytes.Buffer

			i18n.SetLang("en")
			if err := r.render(&enBuf); err != nil {
				t.Fatalf("render (en) failed: %v", err)
			}

			i18n.SetLang("ko")
			if err := r.render(&koBuf); err != nil {
				t.Fatalf("render (ko) failed: %v", err)
			}

			if enBuf.String() != koBuf.String() {
				t.Fatalf("JSON output differs by locale for %s:\nen=%q\nko=%q", r.name, enBuf.String(), koBuf.String())
			}
		})
	}
}

// TestCSVOutputLanguageInvariant is the CSV counterpart of the JSON
// invariance test: --csv output goes through encoding/csv with a fixed
// column-key header slice, never i18n.T.
func TestCSVOutputLanguageInvariant(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	renderers := []struct {
		name   string
		render func(w *bytes.Buffer) error
	}{
		{
			name: "positions",
			render: func(w *bytes.Buffer) error {
				return WritePositions(w, FormatCSV, testPositions)
			},
		},
		{
			name: "quote",
			render: func(w *bytes.Buffer) error {
				return WriteQuote(w, FormatCSV, testQuote)
			},
		},
		{
			name: "orders",
			render: func(w *bytes.Buffer) error {
				return WriteOrders(w, FormatCSV, testInvarianceOrders)
			},
		},
		{
			name: "transactions",
			render: func(w *bytes.Buffer) error {
				return WriteTransactions(w, FormatCSV, testTransactions)
			},
		},
	}

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			var enBuf, koBuf bytes.Buffer

			i18n.SetLang("en")
			if err := r.render(&enBuf); err != nil {
				t.Fatalf("render (en) failed: %v", err)
			}

			i18n.SetLang("ko")
			if err := r.render(&koBuf); err != nil {
				t.Fatalf("render (ko) failed: %v", err)
			}

			if enBuf.String() != koBuf.String() {
				t.Fatalf("CSV output differs by locale for %s:\nen=%q\nko=%q", r.name, enBuf.String(), koBuf.String())
			}
		})
	}
}

// TestTableOutputIsLocalized is a sanity check for the flip side of the
// invariant above: human table output (FormatTable) IS expected to change
// with locale, since that's the entire point of this package's i18n work.
// This guards against someone "fixing" the invariance tests by routing
// everything (including table headers) through a single hardcoded language.
func TestTableOutputIsLocalized(t *testing.T) {
	restore := setTestLang(t)
	defer restore()

	i18n.SetLang("en")
	var enBuf bytes.Buffer
	if err := WritePositions(&enBuf, FormatTable, testPositions); err != nil {
		t.Fatalf("render (en) failed: %v", err)
	}

	i18n.SetLang("ko")
	var koBuf bytes.Buffer
	if err := WritePositions(&koBuf, FormatTable, testPositions); err != nil {
		t.Fatalf("render (ko) failed: %v", err)
	}

	if enBuf.String() == koBuf.String() {
		t.Fatalf("expected table headers to differ between en and ko locales, got identical output")
	}
}

// testInvarianceOrders is synthetic order data (no real account data) used
// only to exercise the order-list renderer across locales.
var testInvarianceOrders = []domain.Order{
	{
		ID:             "ORD-TEST-0001",
		Symbol:         "005930",
		Name:           "삼성전자",
		Market:         "KOSPI",
		Side:           "BUY",
		Status:         "PENDING",
		Quantity:       10,
		FilledQuantity: 0,
		Price:          70000,
		OrderDate:      "2026-01-01",
	},
}

// setTestLang saves/restores the active i18n language around a test so
// SetLang calls in this file don't leak into other tests in the package.
func setTestLang(t *testing.T) func() {
	t.Helper()
	prev := i18n.Lang()
	return func() {
		i18n.SetLang(prev)
	}
}
