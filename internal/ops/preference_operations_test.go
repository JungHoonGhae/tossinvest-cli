package ops

import (
	"context"
	"io"
	"testing"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hiddenholding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/pricealert"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

type opsPriceAlertClient struct {
	alerts []domain.PriceAlert
}

func (c *opsPriceAlertClient) ResolveProductCode(_ context.Context, symbol string) (string, error) {
	return symbol, nil
}
func (c *opsPriceAlertClient) ListPriceAlerts(_ context.Context, code string) (domain.PriceAlerts, error) {
	return domain.PriceAlerts{ProductCode: code, Alerts: append([]domain.PriceAlert(nil), c.alerts...)}, nil
}
func (c *opsPriceAlertClient) AddPriceAlert(_ context.Context, _ string, price float64, currency string) error {
	c.alerts = append(c.alerts, domain.PriceAlert{TargetPrice: price, Currency: currency})
	return nil
}
func (c *opsPriceAlertClient) DeletePriceAlert(_ context.Context, _ string, price float64, currency string) error {
	for i, alert := range c.alerts {
		if alert.TargetPrice == price && alert.Currency == currency {
			c.alerts = append(c.alerts[:i], c.alerts[i+1:]...)
			break
		}
	}
	return nil
}

type opsHiddenClient struct {
	holdings []domain.HiddenHolding
}

func (c *opsHiddenClient) ResolveProductCode(_ context.Context, symbol string) (string, error) {
	return symbol, nil
}
func (c *opsHiddenClient) ListHiddenHoldings(_ context.Context, accountKey string) (domain.HiddenHoldings, error) {
	if accountKey == "" {
		accountKey = "primary-test"
	}
	return domain.HiddenHoldings{AccountKey: accountKey, Holdings: append([]domain.HiddenHolding(nil), c.holdings...)}, nil
}
func (c *opsHiddenClient) HideHolding(_ context.Context, _ string, code string) error {
	c.holdings = append(c.holdings, domain.HiddenHolding{ProductCode: code})
	return nil
}
func (c *opsHiddenClient) ShowHolding(_ context.Context, _ string, code string) error {
	for i, holding := range c.holdings {
		if holding.ProductCode == code {
			c.holdings = append(c.holdings[:i], c.holdings[i+1:]...)
			break
		}
	}
	return nil
}

func TestPreferenceOperationsAreSecuritiesWTSWrites(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{"price_alerts", "price_alert_add", "price_alert_remove", "hidden_holdings", "hidden_holding_hide", "hidden_holding_show"} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Fatalf("operation %s missing", id)
		}
		if op.Domain != "securities" || op.Backend != "wts" {
			t.Fatalf("operation %s metadata = %#v", id, op)
		}
		if (id == "price_alert_add" || id == "price_alert_remove" || id == "hidden_holding_hide" || id == "hidden_holding_show") != op.Write {
			t.Fatalf("operation %s write=%v", id, op.Write)
		}
	}
	banking, _ := catalog.Get("banking_status")
	if banking.Domain != "securities" {
		t.Fatalf("banking_status domain = %q, want securities", banking.Domain)
	}
	openAPIIP, _ := catalog.Get("openapi_ip_replace_current")
	if openAPIIP.Domain != "system" {
		t.Fatalf("openapi_ip domain = %q, want system", openAPIIP.Domain)
	}
	auth, _ := catalog.Get("auth_status")
	if auth.Domain != "system" {
		t.Fatalf("auth_status domain = %q, want system", auth.Domain)
	}
}

func TestPreferenceWriteOperationsUseSafeServices(t *testing.T) {
	t.Parallel()
	priceClient := &opsPriceAlertClient{}
	hiddenClient := &opsHiddenClient{}
	routed := hybrid.New(tossclient.New(tossclient.Config{}), nil, hybrid.Policy{Prefer: routing.WTS}, io.Discard)
	deps := &Deps{
		WTS:            routed,
		PriceAlerts:    pricealert.NewService(priceClient),
		HiddenHoldings: hiddenholding.NewService(hiddenClient),
		Auth:           AuthStatus{WTS: BackendStatus{Connected: true}},
	}
	catalog := NewCatalog()

	previewAny, err := catalog.Call(context.Background(), deps, "price_alert_add", map[string]any{"symbol": "A005930", "price": 70000, "currency": "KRW"})
	if err != nil {
		t.Fatal(err)
	}
	preview := previewAny.(pricealert.Plan)
	if len(priceClient.alerts) != 0 || preview.ConfirmToken == "" {
		t.Fatalf("price preview mutated: %#v %#v", preview, priceClient)
	}
	if _, err := catalog.Call(context.Background(), deps, "price_alert_add", map[string]any{"symbol": "A005930", "price": 70000, "currency": "KRW", "execute": true, "confirm": preview.ConfirmToken}); err != nil {
		t.Fatal(err)
	}
	if len(priceClient.alerts) != 1 {
		t.Fatalf("price alert not applied: %#v", priceClient)
	}

	hiddenPreviewAny, err := catalog.Call(context.Background(), deps, "hidden_holding_hide", map[string]any{"symbol": "A005930"})
	if err != nil {
		t.Fatal(err)
	}
	hiddenPreview := hiddenPreviewAny.(hiddenholding.Plan)
	if len(hiddenClient.holdings) != 0 || hiddenPreview.ConfirmToken == "" {
		t.Fatalf("hidden preview mutated: %#v %#v", hiddenPreview, hiddenClient)
	}
	if _, err := catalog.Call(context.Background(), deps, "hidden_holding_hide", map[string]any{"symbol": "A005930", "execute": true, "confirm": hiddenPreview.ConfirmToken}); err != nil {
		t.Fatal(err)
	}
	if len(hiddenClient.holdings) != 1 {
		t.Fatalf("hidden holding not applied: %#v", hiddenClient)
	}
}
