package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

type probeTransport func(*http.Request) (*http.Response, error)

func (f probeTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func probeResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func namedProbe(t *testing.T, name string) Probe {
	t.Helper()
	for _, p := range Probes() {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("missing probe %s", name)
	return Probe{}
}

func TestRunProbesWatchlistPrerequisite(t *testing.T) {
	t.Parallel()
	groups := namedProbe(t, "watchlist-groups")
	group := namedProbe(t, "watchlist-group")
	independent := Probe{Name: "independent", Method: "GET", URL: "https://monitor.invalid/independent", Check: statusAndPath("result", "bool")}
	const validGroups = `{"result":{"watchlists":[{"id":731,"itemCount":0,"items":[]}]}}`
	for _, tc := range []struct {
		name        string
		status      int
		body        string
		omit        bool
		wrongFolder bool
		wantOK      bool
		wantSkipped bool
		wantCalled  bool
		wantDetail  string
	}{
		{name: "valid folder", status: 200, body: validGroups, wantOK: true, wantCalled: true},
		{name: "empty list", status: 200, body: `{"result":{"watchlists":[]}}`, wantSkipped: true, wantDetail: "account has no watchlist folders"},
		{name: "server failure", status: 503, body: `{}`, wantDetail: "blocked by watchlist-groups"},
		{name: "invalid JSON", status: 200, body: `{`, wantDetail: "blocked by watchlist-groups"},
		{name: "missing list", status: 200, body: `{"result":{}}`, wantDetail: "blocked by watchlist-groups"},
		{name: "invalid folder ID", status: 200, body: `{"result":{"watchlists":[{"id":0,"itemCount":0,"items":[]}]}}`, wantDetail: "blocked by watchlist-groups"},
		{name: "missing prerequisite", omit: true, wantDetail: "blocked by watchlist-groups"},
		{name: "wrong returned folder", status: 200, body: validGroups, wrongFolder: true, wantCalled: true, wantDetail: "does not contain requested folder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var mu sync.Mutex
			calls := make(map[string]int)
			client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
				mu.Lock()
				defer mu.Unlock()
				url := req.URL.String()
				calls[url]++
				switch url {
				case groups.URL:
					return probeResponse(tc.status, tc.body), nil
				case strings.ReplaceAll(group.URL, "{watchlistGroupId}", "731"):
					if req.URL.Query().Get("ids") != "731" || req.URL.Query().Get("includePrice") != "true" {
						t.Errorf("resolved query = %s", req.URL.RawQuery)
					}
					if calls[groups.URL] != 1 {
						t.Error("dependent request ran without exactly one prerequisite request")
					}
					if tc.wrongFolder {
						return probeResponse(200, strings.ReplaceAll(validGroups, "731", "999")), nil
					}
					return probeResponse(200, validGroups), nil
				case independent.URL:
					return probeResponse(200, `{"result":true}`), nil
				default:
					return nil, fmt.Errorf("unexpected test request: %s", url)
				}
			})}
			// Deliberately place the dependent before its prerequisite. Results
			// must retain this order even though execution cannot.
			probes := []Probe{group, independent}
			if !tc.omit {
				probes = append(probes, groups)
			}
			results := runProbes(context.Background(), nil, probes, client)
			if len(results) != len(probes) {
				t.Fatalf("results = %d, want %d", len(results), len(probes))
			}
			for i, result := range results {
				if result.Probe.Name != probes[i].Name {
					t.Errorf("result %d = %s, want %s", i, result.Probe.Name, probes[i].Name)
				}
			}
			result := results[0]
			if result.OK != tc.wantOK || result.Skipped != tc.wantSkipped || !strings.Contains(result.Detail, tc.wantDetail) {
				t.Errorf("dependent = OK:%t skipped:%t detail:%q, want OK:%t skipped:%t detail containing %q", result.OK, result.Skipped, result.Detail, tc.wantOK, tc.wantSkipped, tc.wantDetail)
			}
			if !results[1].OK || calls[independent.URL] != 1 {
				t.Error("independent probe must still execute and succeed")
			}
			called := calls[strings.ReplaceAll(group.URL, "{watchlistGroupId}", "731")] != 0
			if called != tc.wantCalled {
				t.Errorf("dependent called = %t, want %t", called, tc.wantCalled)
			}
			if !tc.omit {
				if calls[groups.URL] != 1 {
					t.Errorf("prerequisite called %d times, want 1", calls[groups.URL])
				}
				if results[2].Status != tc.status {
					t.Errorf("original status lost: %d, want %d", results[2].Status, tc.status)
				}
				if !tc.wantOK && !tc.wantSkipped && !tc.wrongFolder && results[2].OK {
					t.Error("prerequisite failure must remain visible")
				}
			}
		})
	}
}

func TestRunProbesAccountPrerequisite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, wantKey, wantDetail string
		status                          int
		omit                            bool
	}{
		{name: "primary key", status: 200, body: `{"result":{"primaryKey":"primary-test","accountList":[{"key":"first-test"}]}}`, wantKey: "primary-test"},
		{name: "fallback key", status: 200, body: `{"result":{"accountList":[{"key":"first-test"}]}}`, wantKey: "first-test"},
		{name: "empty key", status: 200, body: `{"result":{"accountList":[]}}`, wantDetail: "did not return a primary account key"},
		{name: "failed lookup", status: 401, body: `{}`, wantDetail: "blocked by account-list"},
		{name: "invalid schema", status: 200, body: `{"result":{}}`, wantDetail: "blocked by account-list"},
		{name: "missing prerequisite", omit: true, wantDetail: "blocked by account-list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var accountCalls, scopedCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Header.Get("X-Test-Session") != "test-session" {
					t.Error("session header missing")
				}
				if cookie, err := req.Cookie("test-cookie"); err != nil || cookie.Value != "test-cookie-value" {
					t.Error("session cookie missing")
				}
				if req.Header.Get("Origin") != "https://www.tossinvest.com" || req.Header.Get("Accept") != "application/json" {
					t.Error("browser headers missing")
				}
				switch req.URL.Path {
				case "/accounts":
					accountCalls.Add(1)
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				case "/scoped":
					scopedCalls.Add(1)
					if accountCalls.Load() != 1 {
						t.Error("scoped request ran before account lookup")
					}
					if got := req.Header.Get("accountKey"); got != tc.wantKey {
						t.Errorf("accountKey = %q, want %q", got, tc.wantKey)
					}
					body, err := io.ReadAll(req.Body)
					if err != nil || string(body) != `{"test":true}` || req.Method != "POST" || req.Header.Get("Content-Type") != "application/json" {
						t.Error("probe method, body or content type changed")
					}
					_, _ = io.WriteString(w, `{"result":true}`)
				default:
					t.Errorf("unexpected test request: %s", req.URL.Path)
					w.WriteHeader(404)
				}
			}))
			t.Cleanup(server.Close)
			account := namedProbe(t, "account-list")
			account.URL = server.URL + "/accounts"
			probes := []Probe{{Name: "scoped", Method: "POST", URL: server.URL + "/scoped", Body: `{"test":true}`, AccountScoped: true, Check: statusAndPath("result", "bool")}}
			if !tc.omit {
				probes = append(probes, account)
			}
			sess := &session.Session{Headers: map[string]string{"accountKey": "stale-test", "X-Test-Session": "test-session"}, Cookies: map[string]string{"test-cookie": "test-cookie-value"}}
			results := runProbes(context.Background(), sess, probes, server.Client())
			result := results[0]
			wantOK := tc.wantKey != ""
			if result.OK != wantOK || result.Skipped || !strings.Contains(result.Detail, tc.wantDetail) {
				t.Errorf("scoped = OK:%t skipped:%t detail:%q", result.OK, result.Skipped, result.Detail)
			}
			if got := scopedCalls.Load(); (got == 1) != wantOK || got > 1 {
				t.Errorf("scoped requests = %d, want executed:%t", got, wantOK)
			}
			if !tc.omit && (accountCalls.Load() != 1 || results[1].Status != tc.status) {
				t.Error("account lookup must run once and retain its original status")
			}
			if sess.Headers["accountKey"] != "stale-test" {
				t.Error("runner mutated the supplied session")
			}
		})
	}
}

type incompleteProbeBody struct{ closed bool }

func (*incompleteProbeBody) Read(p []byte) (int, error) {
	// Even a complete-looking JSON document must not pass after a read error.
	return copy(p, `{"result":{"watchlists":[]}}`), io.ErrUnexpectedEOF
}

func (b *incompleteProbeBody) Close() error { b.closed = true; return nil }

func TestRunProbesIncompletePrerequisiteResponse(t *testing.T) {
	t.Parallel()
	body := &incompleteProbeBody{}
	groups := namedProbe(t, "watchlist-groups")
	calls := 0
	client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() != groups.URL {
			t.Errorf("unexpected request after incomplete prerequisite: %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body}, nil
	})}
	results := runProbes(context.Background(), nil, []Probe{groups, namedProbe(t, "watchlist-group")}, client)
	if results[0].OK || results[0].Status != 200 || !strings.Contains(results[0].Detail, "read response") {
		t.Errorf("incomplete body accepted: %+v", results[0])
	}
	if results[1].OK || results[1].Skipped || !strings.Contains(results[1].Detail, "blocked by watchlist-groups") {
		t.Errorf("incomplete prerequisite treated as an empty list: %+v", results[1])
	}
	if calls != 1 || !body.closed {
		t.Errorf("calls = %d, body closed = %t", calls, body.closed)
	}
}

func TestRunProbesConcurrencyAndCancellation(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var calls atomic.Int32
		client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			<-req.Context().Done()
			return nil, req.Context().Err()
		})}
		var probes []Probe
		for i := 0; i < maxConcurrentProbes*3; i++ {
			probes = append(probes, Probe{Name: fmt.Sprintf("probe-%d", i), Method: "GET", URL: "https://monitor.invalid/probe", Check: statusAndPath("result", "bool")})
		}
		var results []Result
		go func() { results = runProbes(ctx, nil, probes, client) }()
		synctest.Wait()
		if got := calls.Load(); got != maxConcurrentProbes {
			t.Fatalf("in-flight requests = %d, want %d", got, maxConcurrentProbes)
		}
		cancel()
		synctest.Wait()
		if len(results) != len(probes) || calls.Load() != maxConcurrentProbes {
			t.Fatalf("results = %d, calls = %d; queued requests must not execute after cancellation", len(results), calls.Load())
		}
		for i, result := range results {
			if result.Probe.Name != probes[i].Name || result.OK || result.Skipped || !strings.Contains(result.Detail, "context canceled") {
				t.Errorf("cancelled result %d = %+v", i, result)
			}
		}
	})
}

func TestRunProbesDeadlineAndPreCancelledContext(t *testing.T) {
	t.Parallel()
	for _, preCancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("pre-cancelled=%t", preCancelled), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if preCancelled {
					cancel()
				}
				calls := 0
				client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
					calls++
					<-req.Context().Done()
					return nil, req.Context().Err()
				})}
				started := time.Now()
				results := runProbes(ctx, nil, []Probe{namedProbe(t, "watchlist-groups"), namedProbe(t, "watchlist-group")}, client)
				wantCalls, wantElapsed, wantDetail := 1, 10*time.Second, "context deadline exceeded"
				if preCancelled {
					wantCalls, wantElapsed, wantDetail = 0, 0, "context canceled"
				}
				if calls != wantCalls || time.Since(started) != wantElapsed || !strings.Contains(results[0].Detail, wantDetail) {
					t.Errorf("calls = %d, elapsed = %s, detail = %s", calls, time.Since(started), results[0].Detail)
				}
				if results[1].OK || results[1].Skipped {
					t.Error("cancelled or timed-out prerequisite cannot make the dependent pass or skip")
				}
			})
		})
	}
}

func TestRunProbesCatalogExperimentGate(t *testing.T) {
	t.Parallel()
	for _, optIn := range []bool{false, true} {
		t.Run(fmt.Sprintf("paper=%t", optIn), func(t *testing.T) {
			t.Parallel()
			var experiments []string
			if optIn {
				experiments = []string{"paper-trading"}
			}
			probes := Probes(experiments...)
			client := &http.Client{Transport: probeTransport(func(req *http.Request) (*http.Response, error) {
				return probeResponse(503, `{}`), nil
			})}
			results := runProbes(context.Background(), nil, probes, client)
			paperCount := 0
			for i, result := range results {
				if result.Probe.Name != probes[i].Name || result.OK || result.Skipped {
					t.Errorf("failed catalog result %d changed order or classification: %+v", i, result)
				}
				if strings.HasPrefix(result.Probe.Name, "paper-") {
					paperCount++
					if result.Status != 503 {
						t.Errorf("opted-in paper probe was not executed: %+v", result)
					}
				}
			}
			wantPaper := 0
			if optIn {
				wantPaper = 4
			}
			if paperCount != wantPaper || len(results) != len(Probes())+wantPaper {
				t.Errorf("paper results = %d, total = %d", paperCount, len(results))
			}
		})
	}
}
