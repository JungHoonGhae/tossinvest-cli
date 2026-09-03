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

func TestTradingSettingsOperationIsCallableAndOwnsDependencyProbes(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/list":
			_, _ = w.Write([]byte(`{"result":{"accountList":[{"key":"primary-test"}],"primaryKey":"primary-test"}}`))
		case "/api/v1/trading/settings/simple-trade":
			if r.Header.Get("accountKey") != "primary-test" {
				t.Fatalf("simple-trade accountKey = %q", r.Header.Get("accountKey"))
			}
			_, _ = w.Write([]byte(`{"result":false}`))
		case "/api/v2/trading/settings/investor-exchange-choice-type":
			_, _ = w.Write([]byte(`{"result":"nxt"}`))
		case "/api/v1/users/settings/me/ats-notification":
			_, _ = w.Write([]byte(`{"result":true}`))
		case "/api/v1/member-subscriptions/get-option-real-time-tick":
			_, _ = w.Write([]byte(`{"result":{"requested":false,"serviced":true,"shouldCharged":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	catalog := NewCatalog()
	op, ok := catalog.Get("trading_settings")
	if !ok {
		t.Fatal("trading_settings operation missing")
	}
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil || len(op.ExtraProbes) != 3 {
		t.Fatalf("operation metadata = %#v", op)
	}
	gotAny, err := catalog.Call(context.Background(), deps, "trading_settings", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := gotAny.(domain.TradingSettings)
	if got.InvestorExchangeChoice != "nxt" || !got.ATSNotificationEnabled || !got.OptionRealTimeTick.Serviced {
		t.Fatalf("result = %#v", got)
	}

	probes := append([]ProbeSpec{*op.Probe}, op.ExtraProbes...)
	want := map[string]string{
		"trading-simple-trade":     "bool",
		"trading-exchange-choice":  "string",
		"trading-ats-notification": "bool",
		"option-real-time-tick":    "object",
	}
	for _, probe := range probes {
		typ, found := want[probe.Name]
		if !found {
			t.Errorf("unexpected probe %q", probe.Name)
			continue
		}
		delete(want, probe.Name)
		body := `{"result":true}`
		if typ == "string" {
			body = `{"result":"integrated"}`
		} else if typ == "object" {
			body = `{"result":{"requested":false,"serviced":true,"shouldCharged":false}}`
		}
		if err := probe.Check(http.StatusOK, []byte(body)); err != nil {
			t.Errorf("%s rejected verified schema: %v", probe.Name, err)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing probes: %v", want)
	}
}

func TestSecuritiesTransferAccountsOperationMasksNumbersAndOwnsDependencyProbes(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/list":
			_, _ = w.Write([]byte(`{"result":{"accountList":[{"key":"primary-test"}],"primaryKey":"primary-test"}}`))
		case "/api/v1/securities-transfer/my-accounts":
			_, _ = w.Write([]byte(`{"result":[{"bankCode":"092","accountNo":"123-456-789","accountId":"own-1"}]}`))
		case "/api/v1/securities-transfer/recent-accounts":
			_, _ = w.Write([]byte(`{"result":[{"bankCode":"088","accountNo":"987-654-321"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	catalog := NewCatalog()
	op, ok := catalog.Get("securities_transfer_accounts")
	if !ok {
		t.Fatal("securities_transfer_accounts operation missing")
	}
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil || len(op.ExtraProbes) != 1 {
		t.Fatalf("operation metadata = %#v", op)
	}

	maskedAny, err := catalog.Call(context.Background(), deps, "securities_transfer_accounts", nil)
	if err != nil {
		t.Fatal(err)
	}
	masked := maskedAny.(domain.SecuritiesTransferAccounts)
	if masked.OwnAccounts[0].AccountNo == "123-456-789" || masked.RecentAccounts[0].AccountNo == "987-654-321" {
		t.Fatalf("default result leaked account numbers: %#v", masked)
	}
	fullAny, err := catalog.Call(context.Background(), deps, "securities_transfer_accounts", map[string]any{"full": true})
	if err != nil {
		t.Fatal(err)
	}
	full := fullAny.(domain.SecuritiesTransferAccounts)
	if full.OwnAccounts[0].AccountNo != "123-456-789" || full.RecentAccounts[0].AccountNo != "987-654-321" {
		t.Fatalf("full result = %#v", full)
	}

	for _, probe := range append([]ProbeSpec{*op.Probe}, op.ExtraProbes...) {
		if err := probe.Check(http.StatusOK, []byte(`{"result":[]}`)); err != nil {
			t.Errorf("%s rejected empty valid list: %v", probe.Name, err)
		}
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

func TestEarningCallDetailOperationIsCallableAndMonitored(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/earning-call/events/228692/info" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"eventId":228692,"status":"UPCOMING","title":"Example call","companyCode":"NYS001FR9-E0","companyName":"Example","liveAt":"2026-09-10T20:00:00+09:00","audioUrl":null,"transcriptUrl":null,"slideFileUrl":null}}`))
	}))
	catalog := NewCatalog()

	op, ok := catalog.Get("earning_call_detail")
	if !ok {
		t.Fatal("earning_call_detail operation missing")
	}
	if op.Backend != "wts" || op.Domain != "securities" || op.Write || op.Probe == nil {
		t.Fatalf("operation metadata = %#v", op)
	}
	if len(op.Params) != 1 || op.Params[0].Name != "event_id" || !op.Params[0].Required {
		t.Fatalf("event_id parameter = %#v", op.Params)
	}
	result, err := catalog.Call(context.Background(), deps, "earning_call_detail", map[string]any{
		"event_id": 228692,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := result.(domain.EarningCallDetail)
	if got.EventID != 228692 || got.Title != "Example call" || got.AudioURL != nil {
		t.Fatalf("result = %#v", got)
	}
	if err := op.Probe.Check(http.StatusOK, []byte(`{"result":{"eventId":228692}}`)); err != nil {
		t.Fatalf("probe rejected valid detail payload: %v", err)
	}
}

func TestBankingStatusOperationMasksIdentityUnlessFull(t *testing.T) {
	t.Parallel()
	deps := discoveryWTSDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/autotrade/open-banking/info/find":
			_, _ = w.Write([]byte(`{"result":{"name":"홍길동","connectedOpenBankingAccount":{"accountNo":"123-456-789","bankCode":"088","openBankingId":42},"openBankingAccounts":[],"registrableAccounts":[],"savingCount":1}}`))
		case "/api/v1/autotrade/open-banking/creatable":
			_, _ = w.Write([]byte(`{"result":true}`))
		case "/api/v1/autotrade/open-banking/need-registration":
			_, _ = w.Write([]byte(`{"result":false}`))
		default:
			http.NotFound(w, r)
		}
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

func TestBankingStatusOperationOwnsEveryHTTPDependencyProbe(t *testing.T) {
	t.Parallel()
	op, ok := NewCatalog().Get("banking_status")
	if !ok {
		t.Fatal("banking_status operation missing")
	}
	probes := map[string]ProbeSpec{}
	if op.Probe != nil {
		probes[op.Probe.Name] = *op.Probe
	}
	for _, probe := range op.ExtraProbes {
		probes[probe.Name] = probe
	}
	for _, name := range []string{"open-banking-status", "open-banking-creatable", "open-banking-registration"} {
		probe, found := probes[name]
		if !found {
			t.Errorf("dependency probe %q missing", name)
			continue
		}
		if err := probe.Check(http.StatusOK, []byte(`{"result":true}`)); err != nil && name != "open-banking-status" {
			t.Errorf("%s rejected boolean contract: %v", name, err)
		}
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
