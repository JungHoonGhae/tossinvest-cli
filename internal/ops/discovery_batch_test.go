package ops

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func discoveryWTSDeps(t *testing.T, handler http.Handler) *Deps {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	wts := tossclient.New(tossclient.Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "test"}},
	})
	return &Deps{
		WTS:  hybrid.New(wts, nil, hybrid.Policy{Prefer: routing.WTS}, &bytes.Buffer{}),
		Auth: AuthStatus{WTS: BackendStatus{Connected: true}},
	}
}

func TestDiscoveryBatchOperationsAreCataloguedAndReadOnly(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{"market_key_events", "banking_status", "notification_settings"} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Errorf("operation %q missing", id)
			continue
		}
		if op.Backend != "wts" || op.Write {
			t.Errorf("operation %q metadata = %#v", id, op)
		}
		if op.Probe == nil {
			t.Errorf("operation %q needs a regression probe", id)
		}
	}
	overview, ok := catalog.Get("account_overview")
	if !ok || overview.Method != http.MethodPost {
		t.Fatalf("account_overview method = %q, want POST", overview.Method)
	}
}

func TestHiddenReadBatchOperationsAreCataloguedAndMonitored(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	for _, id := range []string{"market_news_briefing", "sector_detail", "lending_top_revenue"} {
		op, ok := catalog.Get(id)
		if !ok {
			t.Errorf("operation %q missing", id)
			continue
		}
		if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil {
			t.Errorf("operation %q metadata = %#v", id, op)
		}
	}
	wantSectorProbes := map[string]bool{
		"sector-detail-simple":   false,
		"sector-detail-overview": false,
		"sector-detail-stocks":   false,
		"sector-detail-etfs":     false,
		"sector-detail-news":     false,
	}
	for _, probe := range catalog.Probes() {
		if _, ok := wantSectorProbes[probe.Name]; ok {
			wantSectorProbes[probe.Name] = true
		}
	}
	for name, found := range wantSectorProbes {
		if !found {
			t.Errorf("sector detail dependency probe %q missing", name)
		}
	}
}

func TestAISignalDetailOperationIsCallableAndMonitored(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/dashboard/wts/overview/ai-signals/detail" ||
			r.URL.Query().Get("productCode") != "A005930" ||
			r.URL.Query().Get("productType") != "STOCKS" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"signalId":"signal-1","reasoning":{"issue":{"assetCode":"A005930","assetName":"예시 종목","assetType":"STOCKS","description":{"data":[]},"originCodes":[]},"keywords":[],"news":{"data":[]}},"relatedReasoning":{"details":[]},"terms":{}}}`))
	}))
	catalog := NewCatalog()

	op, ok := catalog.Get("ai_signal_detail")
	if !ok {
		t.Fatal("ai_signal_detail operation missing")
	}
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil {
		t.Fatalf("operation metadata = %#v", op)
	}
	if len(op.Params) != 2 || op.Params[1].Required {
		t.Fatalf("product_type should be optional with the stock default: %#v", op.Params)
	}
	result, err := catalog.Call(context.Background(), deps, "ai_signal_detail", map[string]any{
		"symbol": "005930",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.(domain.AISignalDetail); !got.Found || got.SignalID != "signal-1" {
		t.Fatalf("result = %#v", got)
	}
	for _, tc := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "no current signal", body: `{"result":null}`},
		{name: "active signal", body: `{"result":{"signalId":"s1","reasoning":{"issue":{"assetCode":"A005930"},"news":{"data":[]}},"relatedReasoning":{"details":[]}}}`},
		{name: "contract drift", body: `{"result":{"signalId":"s1"}}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := op.Probe.Check(http.StatusOK, []byte(tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("probe error = %v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestBankingStatusOperationMasksIdentityUnlessFull(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/autotrade/open-banking/info/find" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"name":"홍길동","connectedOpenBankingAccount":{"accountNo":"123-456-789","bankCode":"088","openBankingId":42},"openBankingAccounts":[],"registrableAccounts":[],"savingCount":1}}`))
	}))
	catalog := NewCatalog()

	maskedAny, err := catalog.Call(context.Background(), deps, "banking_status", nil)
	if err != nil {
		t.Fatalf("masked: %v", err)
	}
	masked := maskedAny.(domain.OpenBankingStatus)
	if masked.HolderName == "홍길동" || masked.ConnectedAccount.AccountNo == "123-456-789" {
		t.Fatalf("MCP default leaked identity: %#v", masked)
	}

	fullAny, err := catalog.Call(context.Background(), deps, "banking_status", map[string]any{"full": true})
	if err != nil {
		t.Fatalf("full: %v", err)
	}
	full := fullAny.(domain.OpenBankingStatus)
	if full.HolderName != "홍길동" || full.ConnectedAccount.AccountNo != "123-456-789" {
		t.Fatalf("full view missing identity: %#v", full)
	}
}

func TestAccountOverviewOperationMasksIdentityUnlessFull(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/dashboard/all-accounts" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":[{"data":{"accountOverviews":[{"accountName":"일반","accountNo":"123-456-789","pendingOrderCount":0,"totalAssetAmount":1}],"minorAccountOverviews":[],"totalAssetAmount":1}}]}`))
	}))
	catalog := NewCatalog()

	maskedAny, err := catalog.Call(context.Background(), deps, "account_overview", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := maskedAny.(domain.AccountOverview).Accounts[0].AccountNo; got == "123-456-789" {
		t.Fatal("account_overview MCP default leaked account number")
	}
	fullAny, err := catalog.Call(context.Background(), deps, "account_overview", map[string]any{"full": true})
	if err != nil {
		t.Fatal(err)
	}
	if got := fullAny.(domain.AccountOverview).Accounts[0].AccountNo; got != "123-456-789" {
		t.Fatalf("full account number = %q", got)
	}
}
