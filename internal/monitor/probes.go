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
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/ops"
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
// Most probes are declared next to their operation in the internal/ops
// registry (an operation owns probes for all of its HTTP dependencies) and
// remainder are CLI-surface probes with no registry operation (quote/market
// commands that call the WTS client directly) and stay hand-listed below.
//
// Each probe's Check is a schema invariant — the smallest assertion that
// catches a contract change like #29 without false-positiving on Toss
// adding/removing unrelated fields.
func Probes() []Probe {
	const (
		api  = "https://wts-api.tossinvest.com"
		info = "https://wts-info-api.tossinvest.com"
	)
	var out []Probe
	for _, spec := range ops.NewCatalog().Probes() {
		out = append(out, Probe{Name: spec.Name, Method: spec.Method, URL: spec.URL, Body: spec.Body, Check: spec.Check})
	}
	// CLI-surface probes without a registry operation (covered by cmd quote/market).
	out = append(out,
		Probe{
			Name:   "account-list",
			Method: "GET",
			URL:    api + "/api/v1/account/list",
			Check:  statusAndPath("result.accountList", "array"),
		},
		Probe{
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
		Probe{
			Name:   "quote-trades",
			Method: "GET",
			URL:    info + "/api/v2/stock-prices/A005930/ticks?viewType=krx_all&investMode=krx&count=1",
			Check:  statusAndPath("result", "array"),
		},
		Probe{
			Name:   "quote-orderbook",
			Method: "GET",
			URL:    info + "/api/v3/stock-prices/A005930/quotes",
			Check:  statusAndPath("result.offerPrices", "array"),
		},
		Probe{
			Name:   "quote-price-limits",
			Method: "GET",
			URL:    info + "/api/v2/stock-prices/A005930/upper-lower",
			Check:  statusAndPath("result.upperLimit", "number"),
		},
		Probe{
			Name:   "market-trading-hours",
			Method: "GET",
			URL:    api + "/api/v2/system/trading-hours/integrated",
			Check:  statusAndPath("result.kr", "object"),
		},
	)
	return out
}

func statusAndPath(path, typ string) func(int, []byte) error {
	return func(status int, body []byte) error {
		if err := expectStatus(status, 200); err != nil {
			return err
		}
		return expectPath(body, path, typ)
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

// expectStatus / expectPath moved to internal/ops (probe specs live next to
// their operations there); kept as aliases for the hand-listed probes above
// and the package tests.
var (
	expectStatus = ops.ExpectStatus
	expectPath   = ops.ExpectPath
)
