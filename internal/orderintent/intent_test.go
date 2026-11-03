package orderintent

import (
	"strings"
	"testing"
)

func TestNormalizePlace(t *testing.T) {
	intent, err := NormalizePlace(PlaceInput{
		Symbol:       "tsll",
		Market:       "US",
		Side:         "BUY",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "krw",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}

	if intent.Symbol != "TSLL" {
		t.Fatalf("expected uppercase symbol, got %q", intent.Symbol)
	}
	if intent.Side != "buy" {
		t.Fatalf("expected normalized side, got %q", intent.Side)
	}
	if intent.Market != "us" {
		t.Fatalf("expected normalized market, got %q", intent.Market)
	}
	if intent.CurrencyMode != "KRW" {
		t.Fatalf("expected normalized currency mode, got %q", intent.CurrencyMode)
	}
}

func TestNormalizeAmendRequiresMutationField(t *testing.T) {
	if _, err := NormalizeAmend("5", nil, nil); err == nil {
		t.Fatal("expected error when amend does not change quantity or price")
	}
}

func TestNormalizeCancelRequiresSymbol(t *testing.T) {
	intent, err := NormalizeCancel("5", "tsll")
	if err != nil {
		t.Fatalf("NormalizeCancel returned error: %v", err)
	}
	if intent.Symbol != "TSLL" {
		t.Fatalf("expected uppercase symbol, got %q", intent.Symbol)
	}
}

func TestNormalizePlaceKRSymbolWithDefaultMarketAutoRoutesToKR(t *testing.T) {
	// A 6-digit Korean code under the default "us" market is auto-routed to kr
	// rather than rejected — a KR code is never a valid US ticker.
	intent, err := NormalizePlace(PlaceInput{
		Symbol:       "005930",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        200000,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("expected KR symbol to auto-route, got error: %v", err)
	}
	if intent.Market != "kr" {
		t.Fatalf("expected auto-routed market kr, got %q", intent.Market)
	}
}

func TestNormalizePlaceFractionalKRSymbolStillRejected(t *testing.T) {
	// Auto-routing a KR code to kr must not silently enable fractional (US-only).
	_, err := NormalizePlace(PlaceInput{
		Symbol:       "005930",
		Market:       "us",
		Side:         "buy",
		OrderType:    "market",
		Amount:       10000,
		CurrencyMode: "KRW",
		Fractional:   true,
	})
	if err == nil {
		t.Fatal("expected fractional KR order to be rejected after auto-route")
	}
}

func TestNormalizePlaceKRSymbolWithKRMarketSucceeds(t *testing.T) {
	intent, err := NormalizePlace(PlaceInput{
		Symbol:       "005930",
		Market:       "kr",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        200000,
		CurrencyMode: "KRW",
	})
	if err != nil {
		t.Fatalf("NormalizePlace returned error: %v", err)
	}
	if intent.Market != "kr" {
		t.Fatalf("expected market kr, got %q", intent.Market)
	}
}

func TestNormalizePlaceFractionalAutoMarketOrder(t *testing.T) {
	intent, err := NormalizePlace(PlaceInput{Symbol: "TSLL", Side: "buy", Amount: 18000, Fractional: true})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if intent.OrderType != "market" {
		t.Fatalf("expected market, got %q", intent.OrderType)
	}
	if intent.Quantity != 0 {
		t.Fatalf("expected quantity 0, got %v", intent.Quantity)
	}
}

func TestNormalizePlaceFractionalKRRejects(t *testing.T) {
	_, err := NormalizePlace(PlaceInput{Symbol: "290080", Market: "kr", Side: "buy", Amount: 8000, Fractional: true})
	if err == nil {
		t.Fatal("expected error for KR fractional")
	}
}

func TestNormalizePlaceFractionalRequiresAmount(t *testing.T) {
	_, err := NormalizePlace(PlaceInput{Symbol: "TSLL", Side: "buy", Fractional: true})
	if err == nil {
		t.Fatal("expected error when amount is zero")
	}
}

func TestNormalizePlaceFractionalSellAcceptsDecimalQuantity(t *testing.T) {
	// Official Open API (≥1.1.5): US MARKET SELL takes a decimal quantity.
	intent, err := NormalizePlace(PlaceInput{Symbol: "TSLL", Side: "sell", Quantity: 0.5, Fractional: true})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if intent.OrderType != "market" {
		t.Fatalf("expected market, got %q", intent.OrderType)
	}
	if intent.Quantity != 0.5 {
		t.Fatalf("expected quantity 0.5, got %v", intent.Quantity)
	}
	if intent.Amount != 0 {
		t.Fatalf("expected amount 0, got %v", intent.Amount)
	}
}

func TestNormalizePlaceFractionalSellRequiresQuantity(t *testing.T) {
	if _, err := NormalizePlace(PlaceInput{Symbol: "TSLL", Side: "sell", Fractional: true}); err == nil {
		t.Fatal("expected error when fractional sell quantity is zero")
	}
}

func TestNormalizePlaceFractionalSellRejectsExcessScale(t *testing.T) {
	// More than 6 decimal places must be rejected (official limit).
	if _, err := NormalizePlace(PlaceInput{Symbol: "TSLL", Side: "sell", Quantity: 0.1234567, Fractional: true}); err == nil {
		t.Fatal("expected error for quantity with >6 decimal places")
	}
}

func TestInferMarketFromStockCode(t *testing.T) {
	cases := []struct {
		code   string
		expect string
	}{
		{"A290080", "kr"},
		{"A005930", "kr"},
		{"US20220809012", "us"},
		{"AMX0240221001", "us"},
		{"NAS0241211006", "us"},
	}
	for _, tc := range cases {
		got := InferMarketFromStockCode(tc.code)
		if got != tc.expect {
			t.Errorf("InferMarketFromStockCode(%q) = %q, want %q", tc.code, got, tc.expect)
		}
	}
}

func TestConfirmTokenIsDeterministic(t *testing.T) {
	canonical := CanonicalPlace(PlaceIntent{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})

	first := ConfirmToken(canonical)
	second := ConfirmToken(canonical)
	if first != second {
		t.Fatalf("expected stable token, got %q and %q", first, second)
	}
}

// TestIdempotencyKeyFormat guards issue #8: the derived official clientOrderId
// must satisfy the schema (`^[a-zA-Z0-9\-_]+$`, max 36) and be stable per
// inputs — a retry after a transport error must carry the same key so the
// server deduplicates instead of double-ordering.
func TestIdempotencyKeyFormat(t *testing.T) {
	intent := PlaceIntent{Symbol: "TSLL", Market: "us", Side: "buy",
		OrderType: "limit", Quantity: 1, Price: 500, CurrencyMode: "KRW"}
	first := IdempotencyKey(intent)
	if len(first) != 36 {
		t.Fatalf("key length = %d, want 36", len(first))
	}
	for i := range first {
		c := first[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			t.Fatalf("key must be lowercase hex, got %q", first)
		}
	}
	if second := IdempotencyKey(intent); second != first {
		t.Fatalf("key must be deterministic, got %q then %q", first, second)
	}
	changed := intent
	changed.Price = 501
	if IdempotencyKey(changed) == first {
		t.Fatal("different inputs must yield different keys")
	}
}

// TestCanonicalPlaceBindsIdempotencyKey: the confirm token commits to the key
// the server will enforce, mirroring the conditional-order precedent.
func TestCanonicalPlaceBindsIdempotencyKey(t *testing.T) {
	intent := PlaceIntent{Symbol: "TSLL", Market: "us", Side: "buy",
		OrderType: "limit", Quantity: 1, Price: 500, CurrencyMode: "KRW"}
	canonical := CanonicalPlace(intent)
	if !strings.Contains(canonical, IdempotencyKey(intent)) {
		t.Fatalf("canonical must carry the idempotency key, got %q", canonical)
	}
}
