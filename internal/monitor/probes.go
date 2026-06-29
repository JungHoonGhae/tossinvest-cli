// Package monitor runs schema-invariant probes against the read-only Toss
// endpoints the CLI depends on, so that breaking server-side changes (like
// the body-contract change in #29) are caught by a cron job before users
// hit them.
//
// The checks are intentionally narrow: status 200 + a single critical
// JSON path with the expected JSON type. Schema flexibility on non-critical
// fields is allowed so Toss adding new fields does not trip alerts.
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// Probe describes one endpoint to validate.
type Probe struct {
	Name   string
	Method string
	URL    string
	Body   string
	Check  func(status int, body []byte) error
}

// Result of one probe execution.
type Result struct {
	Probe    Probe
	OK       bool
	Status   int
	Duration time.Duration
	Detail   string // failure detail; empty if OK
}

// Probes returns the read-only endpoints we monitor.
//
// Picked to cover one representative endpoint per CLI surface:
//   - account / summary / positions / watchlist / quote / pending-orders
//
// Each probe's Check is a schema invariant — the smallest assertion that
// catches a contract change like #29 without false-positiving on Toss
// adding/removing unrelated fields.
func Probes() []Probe {
	const (
		api  = "https://wts-api.tossinvest.com"
		cert = "https://wts-cert-api.tossinvest.com"
		info = "https://wts-info-api.tossinvest.com"
	)
	return []Probe{
		{
			Name:   "account-list",
			Method: "GET",
			URL:    api + "/api/v1/account/list",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.accountList", "array")
			},
		},
		{
			Name:   "account-summary-overview",
			Method: "GET",
			URL:    cert + "/api/v3/my-assets/summaries/markets/all/overview",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				if err := expectPath(body, "result.overviewByMarket", "object"); err != nil {
					return err
				}
				return expectPath(body, "result.totalAssetAmount", "number")
			},
		},
		{
			// Catches the #29 regression: empty `{}` body returns
			// empty sections + pollIntervalMillis. Real sections array
			// must contain a SORTED_OVERVIEW entry with products[].
			Name:   "portfolio-positions",
			Method: "POST",
			URL:    cert + "/api/v2/dashboard/asset/sections/all",
			Body:   `{"types":["SORTED_OVERVIEW"]}`,
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				if err := expectPath(body, "result.sections", "array"); err != nil {
					return err
				}
				// Drill into first section: type must match filter.
				var env struct {
					Result struct {
						Sections []struct {
							Type string `json:"type"`
							Data struct {
								Products json.RawMessage `json:"products"`
							} `json:"data"`
						} `json:"sections"`
					} `json:"result"`
				}
				if err := json.Unmarshal(body, &env); err != nil {
					return fmt.Errorf("decode sections: %v", err)
				}
				if len(env.Result.Sections) == 0 {
					return fmt.Errorf("result.sections is empty — likely body-contract regression (#29-class)")
				}
				if env.Result.Sections[0].Type != "SORTED_OVERVIEW" {
					return fmt.Errorf("expected section[0].type=SORTED_OVERVIEW, got %q", env.Result.Sections[0].Type)
				}
				if !bytes.HasPrefix(bytes.TrimSpace(env.Result.Sections[0].Data.Products), []byte("[")) {
					return fmt.Errorf("section[0].data.products is not an array")
				}
				return nil
			},
		},
		{
			Name:   "watchlist",
			Method: "POST",
			URL:    cert + "/api/v2/dashboard/asset/sections/all",
			Body:   `{"types":["WATCHLIST"]}`,
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				var env struct {
					Result struct {
						Sections []struct {
							Type string `json:"type"`
						} `json:"sections"`
					} `json:"result"`
				}
				if err := json.Unmarshal(body, &env); err != nil {
					return fmt.Errorf("decode sections: %v", err)
				}
				if len(env.Result.Sections) == 0 {
					return fmt.Errorf("result.sections is empty — likely body-contract regression")
				}
				if env.Result.Sections[0].Type != "WATCHLIST" {
					return fmt.Errorf("expected section[0].type=WATCHLIST, got %q", env.Result.Sections[0].Type)
				}
				return nil
			},
		},
		{
			Name:   "quote-stock-infos",
			Method: "GET",
			URL:    info + "/api/v2/stock-infos/A005930",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				if err := expectPath(body, "result.symbol", "string"); err != nil {
					return err
				}
				return expectPath(body, "result.currency", "string")
			},
		},
		{
			Name:   "pending-orders",
			Method: "GET",
			URL:    cert + "/api/v1/trading/orders/histories/all/pending",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result", "array")
			},
		},
		{
			Name:   "quote-trades",
			Method: "GET",
			URL:    info + "/api/v2/stock-prices/A005930/ticks?viewType=krx_all&investMode=krx&count=1",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result", "array")
			},
		},
		{
			Name:   "quote-orderbook",
			Method: "GET",
			URL:    info + "/api/v3/stock-prices/A005930/quotes",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.offerPrices", "array")
			},
		},
		{
			Name:   "quote-price-limits",
			Method: "GET",
			URL:    info + "/api/v2/stock-prices/A005930/upper-lower",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.upperLimit", "number")
			},
		},
		{
			Name:   "market-trading-hours",
			Method: "GET",
			URL:    api + "/api/v2/system/trading-hours/integrated",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.kr", "object")
			},
		},
		{
			Name:   "market-index",
			Method: "GET",
			URL:    cert + "/api/v1/dashboard/wts/overview/indicator/index",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.majorIndicatorInfos", "array")
			},
		},
		{
			Name:   "stock-ranking",
			Method: "GET",
			URL:    info + "/api/v1/rankings/realtime/stock?size=1",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.data", "array")
			},
		},
		{
			Name:   "index-prices",
			Method: "GET",
			URL:    info + "/api/v1/index-prices/KGG01P",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.close", "number")
			},
		},
		{
			Name:   "investor-rankings",
			Method: "GET",
			URL:    info + "/api/v1/dashboard/wts/overview/rankings/by-investors",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.rankings", "object")
			},
		},
		{
			Name:   "earning-call",
			Method: "GET",
			URL:    info + "/api/v1/earning-call/upcoming",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result", "array")
			},
		},
		{
			Name:   "earning-call-home",
			Method: "GET",
			URL:    info + "/api/v1/earning-call/home",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.majorCompanies", "object")
			},
		},
		{
			Name:   "sectors-tics",
			Method: "GET",
			URL:    info + "/api/v1/tics/all",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.ticsItems", "array")
			},
		},
		{
			Name:   "theme-rankings",
			Method: "GET",
			URL:    info + "/api/v1/tics/rankings",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.data", "array")
			},
		},
		{
			Name:   "community-rankings",
			Method: "GET",
			URL:    info + "/api/v1/community/top-rankings/INFLUENCER",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.items", "array")
			},
		},
		{
			Name:   "news-briefing",
			Method: "GET",
			URL:    info + "/api/v1/dashboard/wts/overview/ai-signals/personalized",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.items", "array")
			},
		},
		{
			Name:   "trading-flows",
			Method: "GET",
			URL:    info + "/api/v1/stock-infos/trade/trend/trading-trend?productCode=A005930&size=1",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.body", "array")
			},
		},
		{
			Name:   "ai-signals",
			Method: "GET",
			URL:    info + "/api/v2/reasoning-contents/interest",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.data", "array")
			},
		},
		{
			Name:   "screener-presets",
			Method: "GET",
			URL:    cert + "/api/v2/screener/presets/common?useCustom=true",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result", "array")
			},
		},
		{
			Name:   "watchlist-groups",
			Method: "GET",
			URL:    cert + "/api/v1/new-watchlists?includePrice=false&lazyLoad=true",
			Check: func(status int, body []byte) error {
				if err := expectStatus(status, 200); err != nil {
					return err
				}
				return expectPath(body, "result.watchlists", "array")
			},
		},
	}
}

// maxConcurrentProbes bounds how many probes hit Toss at once. The probes are
// independent read-only GETs/POSTs, so running them concurrently turns a
// worst-case ~N×10s sequential wall-clock into roughly one 10s window, which
// matters most for the daily-monitor cron. Kept modest to stay polite to the
// upstream and avoid tripping rate limits.
const maxConcurrentProbes = 8

// Run executes all probes concurrently (bounded by maxConcurrentProbes) using
// the session for auth. Results are returned in probe order regardless of
// completion order, so output stays stable.
func Run(ctx context.Context, sess *session.Session) []Result {
	probes := Probes()
	results := make([]Result, len(probes))

	sem := make(chan struct{}, maxConcurrentProbes)
	var wg sync.WaitGroup
	for i, p := range probes {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, p Probe) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = runOne(ctx, sess, p)
		}(i, p)
	}
	wg.Wait()
	return results
}

func runOne(ctx context.Context, sess *session.Session, p Probe) Result {
	res := Result{Probe: p}
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if p.Body != "" {
		bodyReader = strings.NewReader(p.Body)
	}
	req, err := http.NewRequestWithContext(reqCtx, p.Method, p.URL, bodyReader)
	if err != nil {
		res.Detail = "build request: " + err.Error()
		return res
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", tossclient.DefaultBrowserUserAgent)
	req.Header.Set("Referer", "https://www.tossinvest.com/")
	req.Header.Set("Origin", "https://www.tossinvest.com")
	if p.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if sess != nil {
		for k, v := range sess.Cookies {
			req.AddCookie(&http.Cookie{Name: k, Value: v})
		}
		for k, v := range sess.Headers {
			req.Header.Set(k, v)
		}
	}

	start := time.Now()
	resp, err := (&http.Client{}).Do(req)
	res.Duration = time.Since(start)
	if err != nil {
		res.Detail = "transport: " + err.Error()
		return res
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	res.Status = resp.StatusCode
	if checkErr := p.Check(resp.StatusCode, body); checkErr != nil {
		res.Detail = checkErr.Error()
		return res
	}
	res.OK = true
	return res
}

// expectStatus reports a status-code mismatch. Response bodies are not
// embedded in the error so downstream alert payloads stay bounded.
func expectStatus(got, want int) error {
	if got == want {
		return nil
	}
	return fmt.Errorf("status %d (want %d)", got, want)
}

// expectPath walks a dotted JSON path (a.b.c) and asserts the value's type.
// Supported types: "string", "number", "bool", "object", "array", "null".
// Array indexing not supported — for nested-array checks, use a custom Check.
func expectPath(body []byte, path, wantType string) error {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("decode body: %v", err)
	}
	current := v
	for _, segment := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("path %q: expected object at %q, got %s", path, segment, jsonTypeOf(current))
		}
		next, found := obj[segment]
		if !found {
			return fmt.Errorf("path %q: key %q missing", path, segment)
		}
		current = next
	}
	got := jsonTypeOf(current)
	if got != wantType {
		return fmt.Errorf("path %q: expected %s, got %s", path, wantType, got)
	}
	return nil
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return "unknown"
}
