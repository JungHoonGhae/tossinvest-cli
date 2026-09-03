package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteHiddenHoldingsJSONDoesNotExposeAccountKey(t *testing.T) {
	var out bytes.Buffer
	holdings := domain.HiddenHoldings{
		AccountKey:   "acct-sensitive-internal-key",
		AccountScope: "scope-safe",
		Holdings: []domain.HiddenHolding{{
			ProductCode:      "A005930",
			Name:             "Sample Corp",
			Type:             "STOCK",
			TradableQuantity: 3,
		}},
	}

	if err := WriteHiddenHoldings(&out, FormatJSON, holdings); err != nil {
		t.Fatalf("WriteHiddenHoldings JSON: %v", err)
	}
	if strings.Contains(out.String(), holdings.AccountKey) || strings.Contains(out.String(), "account_key") {
		t.Fatalf("internal account key leaked in JSON: %s", out.String())
	}
	if !strings.Contains(out.String(), `"account_scope": "scope-safe"`) {
		t.Fatalf("safe account scope was not rendered: %s", out.String())
	}
	if !strings.Contains(out.String(), `"product_code": "A005930"`) {
		t.Fatalf("holding was not rendered: %s", out.String())
	}
}

func TestWritePriceAlertsCSV(t *testing.T) {
	var out bytes.Buffer
	alerts := domain.PriceAlerts{
		ProductCode: "A005930",
		Alerts: []domain.PriceAlert{{
			TargetPrice: 70000,
			Currency:    "KRW",
		}},
	}

	if err := WritePriceAlerts(&out, FormatCSV, alerts); err != nil {
		t.Fatalf("WritePriceAlerts CSV: %v", err)
	}
	if got, want := out.String(), "product_code,target_price,currency\nA005930,70000,KRW\n"; got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}
