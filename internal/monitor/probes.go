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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/ops"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// Probe describes one endpoint to validate.
type Probe struct {
	Name                 string
	Method               string
	URL                  string
	Body                 string
	AccountScoped        bool
	WatchlistGroupScoped bool
	Check                func(status int, body []byte) error
}

// Result of one probe execution.
type Result struct {
	Probe    Probe
	OK       bool
	Skipped  bool // valid state made a state-dependent probe inapplicable
	Status   int
	Duration time.Duration
	Detail   string // failure or skip detail; empty on an ordinary pass
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
		out = append(out, Probe{Name: spec.Name, Method: spec.Method, URL: spec.URL, Body: spec.Body, AccountScoped: spec.AccountScoped, WatchlistGroupScoped: spec.WatchlistGroupScoped, Check: spec.Check})
	}
	// CLI-surface probes without a registry operation (covered by cmd quote/market).
	out = append(out,
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
	accountListIndex := -1
	watchlistGroupsIndex := -1
	accountKey := ""
	var watchlistGroupID int64
	for i, probe := range probes {
		if probe.Name == "account-list" {
			accountListIndex = i
			var body []byte
			results[i], body = executeProbe(ctx, sess, probe, "")
			if results[i].OK {
				accountKey = accountKeyFromList(body)
			}
			break
		}
	}
	for i, probe := range probes {
		if probe.Name != "watchlist-groups" {
			continue
		}
		watchlistGroupsIndex = i
		var body []byte
		results[i], body = executeProbe(ctx, sess, probe, "")
		if results[i].OK {
			watchlistGroupID = watchlistGroupIDFromList(body)
		}
		break
	}

	sem := make(chan struct{}, maxConcurrentProbes)
	var wg sync.WaitGroup
	for i, p := range probes {
		if i == accountListIndex || i == watchlistGroupsIndex {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, p Probe) {
			defer wg.Done()
			defer func() { <-sem }()
			if p.AccountScoped && accountKey == "" {
				results[i] = Result{Probe: p, Detail: "account-list did not return a primary account key"}
				return
			}
			if p.WatchlistGroupScoped {
				if watchlistGroupID == 0 {
					results[i] = Result{Probe: p, Skipped: true, Detail: "not applicable: account has no watchlist folders"}
					return
				}
				p.URL = strings.ReplaceAll(p.URL, "{watchlistGroupId}", strconv.FormatInt(watchlistGroupID, 10))
				baseCheck := p.Check
				p.Check = func(status int, body []byte) error {
					if err := baseCheck(status, body); err != nil {
						return err
					}
					if !watchlistGroupResponseContains(body, watchlistGroupID) {
						return fmt.Errorf("result.watchlists does not contain requested folder %d", watchlistGroupID)
					}
					return nil
				}
			}
			results[i] = runOne(ctx, sess, p, accountKey)
		}(i, p)
	}
	wg.Wait()
	return results
}

func watchlistGroupResponseContains(body []byte, wanted int64) bool {
	var envelope struct {
		Result struct {
			Watchlists []struct {
				ID int64 `json:"id"`
			} `json:"watchlists"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return false
	}
	for _, group := range envelope.Result.Watchlists {
		if group.ID == wanted {
			return true
		}
	}
	return false
}

func watchlistGroupIDFromList(body []byte) int64 {
	var envelope struct {
		Result struct {
			Watchlists []struct {
				ID int64 `json:"id"`
			} `json:"watchlists"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Result.Watchlists) == 0 {
		return 0
	}
	return envelope.Result.Watchlists[0].ID
}

func runOne(ctx context.Context, sess *session.Session, p Probe, accountKey string) Result {
	result, _ := executeProbe(ctx, sess, p, accountKey)
	return result
}

func executeProbe(ctx context.Context, sess *session.Session, p Probe, accountKey string) (Result, []byte) {
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
		return res, nil
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
	if p.AccountScoped && accountKey != "" {
		req.Header.Set("accountKey", accountKey)
	}

	start := time.Now()
	resp, err := (&http.Client{}).Do(req)
	res.Duration = time.Since(start)
	if err != nil {
		res.Detail = "transport: " + err.Error()
		return res, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	res.Status = resp.StatusCode
	if checkErr := p.Check(resp.StatusCode, body); checkErr != nil {
		res.Detail = checkErr.Error()
		return res, body
	}
	res.OK = true
	return res, body
}

func accountKeyFromList(body []byte) string {
	var envelope struct {
		Result struct {
			PrimaryKey  string `json:"primaryKey"`
			AccountList []struct {
				Key string `json:"key"`
			} `json:"accountList"`
		} `json:"result"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return ""
	}
	if key := strings.TrimSpace(envelope.Result.PrimaryKey); key != "" {
		return key
	}
	if len(envelope.Result.AccountList) > 0 {
		return strings.TrimSpace(envelope.Result.AccountList[0].Key)
	}
	return ""
}

// expectStatus / expectPath moved to internal/ops (probe specs live next to
// their operations there); kept as aliases for the hand-listed probes above
// and the package tests.
var (
	expectStatus = ops.ExpectStatus
	expectPath   = ops.ExpectPath
)
