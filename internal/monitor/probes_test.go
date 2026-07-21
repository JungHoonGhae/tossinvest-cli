package monitor

import (
	"strings"
	"testing"
)

func TestProbesRegistryStableNames(t *testing.T) {
	want := map[string]bool{
		"account-list":             true,
		"account-summary-overview": true,
		"portfolio-positions":      true,
		"watchlist":                true,
		"quote-stock-infos":        true,
		"pending-orders":           true,
		"quote-trades":             true,
		"quote-orderbook":          true,
		"quote-price-limits":       true,
		"market-trading-hours":     true,
		"market-index":             true,
		"stock-ranking":            true,
		"investor-rankings":        true,
		"index-prices":             true,
		"earning-call":             true,
		"earning-call-home":        true,
		"community-rankings":       true,
		"lending-expected":         true,
		"accumulation-plans":       true,
		"profit-overview":          true,
		"news-briefing":            true,
		"sectors-tics":             true,
		"theme-rankings":           true,
		"trading-flows":            true,
		"ai-signals":               true,
		"screener-presets":         true,
		"watchlist-groups":         true,
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
		{"result.c.0", "number", false}, // array indexing not supported
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
