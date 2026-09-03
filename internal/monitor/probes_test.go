package monitor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/ops"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestProbesRegistryStableNames(t *testing.T) {
	want := map[string]bool{
		"account-commission-info":             true,
		"account-interest-years":              true,
		"ria-report":                          true,
		"order-funding":                       true,
		"account-list":                        true,
		"account-summary-overview":            true,
		"account-all-overview":                true,
		"portfolio-positions":                 true,
		"watchlist":                           true,
		"quote-stock-infos":                   true,
		"pending-orders":                      true,
		"quote-trades":                        true,
		"quote-orderbook":                     true,
		"quote-price-limits":                  true,
		"quote-crypto":                        true,
		"quote-stock-signals":                 true,
		"account-receivable":                  true,
		"option-expiries":                     true,
		"screener-filter-range":               true,
		"market-trading-hours":                true,
		"market-index":                        true,
		"stock-ranking":                       true,
		"investor-rankings":                   true,
		"index-prices":                        true,
		"earning-call":                        true,
		"earning-call-home":                   true,
		"earning-call-detail":                 true,
		"community-rankings":                  true,
		"lending-expected":                    true,
		"lending-top-revenue":                 true,
		"accumulation-plans":                  true,
		"profit-overview":                     true,
		"news-briefing":                       true,
		"market-news-briefing":                true,
		"sectors-tics":                        true,
		"sector-detail-simple":                true,
		"sector-detail-overview":              true,
		"sector-detail-stocks":                true,
		"sector-detail-etfs":                  true,
		"sector-detail-news":                  true,
		"theme-rankings":                      true,
		"trading-flows":                       true,
		"ai-signals":                          true,
		"ai-signal-detail":                    true,
		"screener-presets":                    true,
		"watchlist-groups":                    true,
		"market-calendar":                     true,
		"market-halt":                         true,
		"quote-reasons":                       true,
		"quote-charts":                        true,
		"auto-trades":                         true,
		"market-issues":                       true,
		"market-key-events":                   true,
		"open-banking-status":                 true,
		"open-banking-creatable":              true,
		"open-banking-registration":           true,
		"notification-settings":               true,
		"price-alerts":                        true,
		"hidden-holdings":                     true,
		"trading-simple-trade":                true,
		"trading-exchange-choice":             true,
		"trading-ats-notification":            true,
		"option-real-time-tick":               true,
		"securities-transfer-my-accounts":     true,
		"securities-transfer-recent-accounts": true,
	}
	got := map[string]bool{}
	for _, p := range Probes() {
		got[p.Name] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("missing probe %q", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("expected exactly %d probes, got %d (%v)", len(want), len(got), got)
	}
}

func TestDocumentedSurfaceCountsMatchRuntime(t *testing.T) {
	t.Parallel()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	operationCount := ops.NewCatalog().Count()
	registryProbeCount := len(ops.NewCatalog().Probes())
	probeCount := len(Probes())
	directProbeCount := probeCount - registryProbeCount

	claims := map[string][]string{
		"AGENTS.md": {
			fmt.Sprintf("현재 `monitor api` 는 %d개 read-only endpoint", probeCount),
		},
		"README.md": {
			fmt.Sprintf("API 표면은 **%d개 오퍼레이션**", operationCount),
			fmt.Sprintf("monitor api           # %d개 endpoint", probeCount),
		},
		"README.en.md": {
			fmt.Sprintf("surface is **%d operations**", operationCount),
			fmt.Sprintf("schema-probe %d endpoints", probeCount),
		},
		"docs/operations.md": {
			fmt.Sprintf("`monitor api` 명령은 %d개 read-only endpoint", probeCount),
			fmt.Sprintf("%d개는\n`internal/ops` 레지스트리", registryProbeCount),
			fmt.Sprintf("CLI 전용 %d개", directProbeCount),
		},
		"website-fumadocs/content/docs/guide/mcp.mdx": {
			fmt.Sprintf("표면은 **%d개 오퍼레이션**", operationCount),
		},
		"website-fumadocs/content/docs/guide/mcp.en.mdx": {
			fmt.Sprintf("surface is **%d operations**", operationCount),
		},
	}
	for relative, expected := range claims {
		data, err := os.ReadFile(filepath.Join(repo, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		for _, text := range expected {
			if !strings.Contains(string(data), text) {
				t.Errorf("%s is missing runtime-derived count claim %q", relative, text)
			}
		}
	}
}

func TestAccountScopedProbeInjectsResolvedAccountKey(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("accountKey"); got != "primary-test" {
			t.Errorf("accountKey = %q, want primary-test", got)
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	t.Cleanup(server.Close)
	probe := Probe{
		Name: "scoped", Method: http.MethodGet, URL: server.URL,
		AccountScoped: true, Check: statusAndPath("result", "bool"),
	}
	result := runOne(context.Background(), &session.Session{Headers: map[string]string{"accountKey": "stale-test"}}, probe, "primary-test")
	if !result.OK {
		t.Fatalf("probe failed: %s", result.Detail)
	}
}

func TestAccountKeyFromListUsesPrimaryThenFirstAccount(t *testing.T) {
	t.Parallel()
	if got := accountKeyFromList([]byte(`{"result":{"primaryKey":"primary-test","accountList":[{"key":"first-test"}]}}`)); got != "primary-test" {
		t.Fatalf("primary key = %q", got)
	}
	if got := accountKeyFromList([]byte(`{"result":{"accountList":[{"key":"first-test"}]}}`)); got != "first-test" {
		t.Fatalf("fallback key = %q", got)
	}
}

func TestExpectPathTypes(t *testing.T) {
	body := []byte(`{"result":{"a":"hi","b":12,"c":[1,2],"d":{"e":true},"f":null}}`)
	cases := []struct {
		path, typ string
		wantOK    bool
	}{
		{"result.a", "string", true},
		{"result.b", "number", true},
		{"result.c", "array", true},
		{"result.d", "object", true},
		{"result.d.e", "bool", true},
		{"result.f", "null", true},
		{"result.a", "number", false}, // wrong type
		{"result.missing", "string", false},
		{"result.c.0", "number", true},  // numeric segment indexes an array
		{"result.c.9", "number", false}, // out of range
		{"result.d.0", "number", false}, // object indexed as array
	}
	for _, c := range cases {
		err := expectPath(body, c.path, c.typ)
		gotOK := err == nil
		if gotOK != c.wantOK {
			t.Errorf("expectPath(%q, %q): wantOK=%v, gotErr=%v", c.path, c.typ, c.wantOK, err)
		}
	}
}

// PRIVACY invariant: every Check function's error string must not contain
// fragments of the response body — those bodies routinely carry account
// numbers, asset totals, and stockCode lists which we must never forward to
// any external webhook. Feed each probe a response body packed with
// synthetic PII-shaped markers and assert none of the markers escape into
// the error string.
//
// All literal values below are synthetic — fabricated to match the SHAPE of
// Toss responses (10-digit account numbers, USD floats, stock-code prefixes)
// without corresponding to any real account, holding, or person.
func TestProbeChecksDoNotLeakResponseBodyOnFailure(t *testing.T) {
	piiMarkers := []string{
		"9999999999",     // 10-digit account-number shape
		"1234567.890000", // USD-total shape
		"US00000000000",  // stockCode shape (synthetic prefix + zeros)
		"ZZZZ",           // ticker shape
		"sentinel-token-do-not-leak",
		"user@example.invalid",
	}
	piiPayload := `{"error":{"statusCode":500,"accountNo":"9999999999","total":1234567.890000,"stockCode":"US00000000000","symbol":"ZZZZ","token":"sentinel-token-do-not-leak","email":"user@example.invalid"}}`

	for _, p := range Probes() {
		// Trigger status-mismatch path (most common leak surface).
		err := p.Check(500, []byte(piiPayload))
		if err == nil {
			t.Errorf("%s: expected check to fail on status 500", p.Name)
			continue
		}
		for _, marker := range piiMarkers {
			if strings.Contains(err.Error(), marker) {
				t.Errorf("%s: error message leaks marker %q\n  detail: %s", p.Name, marker, err.Error())
			}
		}
	}
}

// Catches the #29 regression: feeding the post-#29 empty-sections response
// to the portfolio probe's Check must fail. This is the contract test that
// proves the monitor would have alerted on the actual incident.
func TestPortfolioPositionsCheckCatchesSectionsAllRegression(t *testing.T) {
	var positionsProbe Probe
	for _, p := range Probes() {
		if p.Name == "portfolio-positions" {
			positionsProbe = p
			break
		}
	}
	if positionsProbe.Name == "" {
		t.Fatal("portfolio-positions probe missing")
	}

	emptyBody := []byte(`{"result":{"sections":[],"pollIntervalMillis":3000}}`)
	if err := positionsProbe.Check(200, emptyBody); err == nil {
		t.Fatal("expected check to fail on empty sections (post-#29 regression shape)")
	}

	goodBody := []byte(`{"result":{"sections":[{"type":"SORTED_OVERVIEW","data":{"products":[]}}]}}`)
	if err := positionsProbe.Check(200, goodBody); err != nil {
		t.Fatalf("expected check to pass on valid shape, got: %v", err)
	}
}

func TestDiscoveryProbeChecksRejectBrokenSchemas(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		good string
		bad  string
	}{
		"news-briefing":                       {good: `{"result":{"items":[]}}`, bad: `{"result":{}}`},
		"market-news-briefing":                {good: `{"result":{"items":[]}}`, bad: `{"result":{}}`},
		"lending-top-revenue":                 {good: `{"result":{"items":[]}}`, bad: `{"result":{}}`},
		"sector-detail-overview":              {good: `{"result":{"ticsId":1}}`, bad: `{"result":{}}`},
		"sector-detail-stocks":                {good: `{"result":{"stocks":[],"totalCount":0}}`, bad: `{"result":{"stocks":[]}}`},
		"sector-detail-etfs":                  {good: `{"result":{"etfs":[],"totalCount":0}}`, bad: `{"result":{"etfs":[]}}`},
		"sector-detail-news":                  {good: `{"result":{"body":[],"totalCount":0}}`, bad: `{"result":{"body":[]}}`},
		"market-key-events":                   {good: `{"result":{"earnings":[],"eci":{"indicators":[]}}}`, bad: `{"result":{"earnings":[]}}`},
		"open-banking-status":                 {good: `{"result":{"savingCount":0}}`, bad: `{"result":{}}`},
		"open-banking-creatable":              {good: `{"result":true}`, bad: `{"result":"true"}`},
		"open-banking-registration":           {good: `{"result":false}`, bad: `{"result":0}`},
		"notification-settings":               {good: `{"result":[]}`, bad: `{"result":{}}`},
		"price-alerts":                        {good: `{"result":[]}`, bad: `{"result":{}}`},
		"hidden-holdings":                     {good: `{"result":{"hiddenStocks":[]}}`, bad: `{"result":{}}`},
		"account-all-overview":                {good: `{"result":[{"data":{"accountOverviews":[],"minorAccountOverviews":[],"totalAssetAmount":0}}]}`, bad: `{"result":[{"data":{"accountOverviews":[]}}]}`},
		"trading-simple-trade":                {good: `{"result":false}`, bad: `{"result":null}`},
		"trading-exchange-choice":             {good: `{"result":"integrated"}`, bad: `{"result":true}`},
		"trading-ats-notification":            {good: `{"result":true}`, bad: `{"result":"true"}`},
		"option-real-time-tick":               {good: `{"result":{"requested":false,"serviced":true,"shouldCharged":false}}`, bad: `{"result":{"requested":false,"serviced":true}}`},
		"securities-transfer-my-accounts":     {good: `{"result":[]}`, bad: `{"result":[{"bankCode":"092","accountNo":"123"}]}`},
		"securities-transfer-recent-accounts": {good: `{"result":[]}`, bad: `{"result":[{"bankCode":"088"}]}`},
	}
	probes := make(map[string]Probe)
	for _, probe := range Probes() {
		probes[probe.Name] = probe
	}
	for name, tc := range tests {
		probe, ok := probes[name]
		if !ok {
			t.Fatalf("probe %q missing", name)
		}
		if err := probe.Check(200, []byte(tc.good)); err != nil {
			t.Errorf("%s good schema: %v", name, err)
		}
		if err := probe.Check(200, []byte(tc.bad)); err == nil {
			t.Errorf("%s accepted broken schema", name)
		}
	}
}
