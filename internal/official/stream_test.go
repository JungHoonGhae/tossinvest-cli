package official

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// streamServer serves both the token endpoint and the websocket, since Stream
// fetches a token before dialing. onDeclare receives the raw declaration frame
// and returns the frames to push back.
func streamServer(t *testing.T, onDeclare func(declaration []byte) []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer AT" {
			t.Errorf("handshake Authorization: want %q, got %q", "Bearer AT", got)
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, decl, err := conn.Read(ctx)
		if err != nil {
			t.Errorf("read declaration: %v", err)
			return
		}
		for _, frame := range onDeclare(decl) {
			if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
				return
			}
		}
	}))
}

func streamClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithStreamURL("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/v1"),
	)
}

// TestStreamDeclaresAndDispatches covers the whole contract: the declaration is
// sent as a single JSON array (full-replace), and ack/data/error frames all
// arrive on the same connection, split only by the top-level `type`.
func TestStreamDeclaresAndDispatches(t *testing.T) {
	var declared []Subscription
	srv := streamServer(t, func(decl []byte) []string {
		if err := json.Unmarshal(decl, &declared); err != nil {
			t.Errorf("declaration is not a JSON array: %v (%s)", err, decl)
		}
		return []string{
			`{"type":"subscriptions","subscribed":["trade:us:AAPL"],"rejected":[{"target":"trade:kr:999999","code":"stock-not-found","message":"없는 종목"}]}`,
			`{"type":"message","topic":"trade:us:AAPL","data":{"price":"243.26","volume":"8","currency":"USD"}}`,
			`{"type":"error","error":{"code":"server-shutdown","message":"재연결해주세요"}}`,
		}
	})
	defer srv.Close()

	var got []StreamFrame
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := streamClient(t, srv).Stream(ctx,
		[]Subscription{{Type: "trade:us", Codes: []string{"AAPL"}}, {Type: "trade:kr", Codes: []string{"999999"}}},
		func(f StreamFrame) { got = append(got, f) },
	)
	// server-shutdown 은 장애가 아니라 예고된 핸드오프다 — 호출자가 구분할 수 있어야 한다.
	if err != ErrStreamShutdown {
		t.Fatalf("want ErrStreamShutdown, got %v", err)
	}

	if len(declared) != 2 || declared[0].Type != "trade:us" || declared[1].Codes[0] != "999999" {
		t.Fatalf("declaration mismatch: %+v", declared)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 frames, got %d: %+v", len(got), got)
	}
	if got[0].Type != "subscriptions" || len(got[0].Rejected) != 1 || got[0].Rejected[0].Code != "stock-not-found" {
		t.Fatalf("ack frame mismatch: %+v", got[0])
	}
	if got[1].Topic != "trade:us:AAPL" || !strings.Contains(string(got[1].Data), "243.26") {
		t.Fatalf("data frame mismatch: %+v", got[1])
	}
	if got[2].Error == nil || got[2].Error.Code != "server-shutdown" {
		t.Fatalf("error frame mismatch: %+v", got[2])
	}
}

func TestStreamRequiresSubscriptions(t *testing.T) {
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, filepath.Join(t.TempDir(), "t.json"))
	if err := c.Stream(context.Background(), nil, func(StreamFrame) {}); err == nil {
		t.Fatal("expected error for empty declaration")
	}
}

// TestStreamSubscriptionsGroupsByMarket pins the symbol → market rule: KRX codes
// are numeric-leading alphanumeric codes, other symbols are treated as US.
func TestStreamSubscriptionsGroupsByMarket(t *testing.T) {
	subs := StreamSubscriptions([]string{"005930", "aapl", " ", " 0101n0 ", "ABCDEF"}, []string{"035420", "00680K"}, []string{"3"})
	want := []Subscription{
		{Type: "trade:kr", Codes: []string{"005930", "0101N0"}},
		{Type: "trade:us", Codes: []string{"AAPL", "ABCDEF"}},
		{Type: "orderbook:kr", Codes: []string{"035420", "00680K"}},
		{Type: "personal:order", Codes: []string{"3"}},
	}
	if len(subs) != len(want) {
		t.Fatalf("want %d subscriptions, got %+v", len(want), subs)
	}
	for i := range want {
		if subs[i].Type != want[i].Type || strings.Join(subs[i].Codes, ",") != strings.Join(want[i].Codes, ",") {
			t.Fatalf("subscription %d: want %+v, got %+v", i, want[i], subs[i])
		}
	}
}

// TestStreamWithRetryStopsOnPersistent401 guards issue #12: a revoked key must
// not spin the reconnect loop forever. The client refreshes the token once
// (2 dials, 2 token calls) and then returns the permanent error.
func TestStreamWithRetryStopsOnPersistent401(t *testing.T) {
	var tokenCalls, dialCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			tokenCalls++
			_, _ = w.Write([]byte(`{"access_token":"AT` + strings.Repeat("X", tokenCalls) + `","expires_in":3600,"token_type":"Bearer"}`))
		default:
			dialCalls++
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)

	c := streamClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := c.StreamWithRetry(ctx,
		[]Subscription{{Type: "trade:us", Codes: []string{"AAPL"}}},
		func(StreamFrame) {}, nil)
	if !errors.Is(err, ErrStreamUnauthorized) {
		t.Fatalf("want permanent unauthorized error, got %v", err)
	}
	if dialCalls != 2 {
		t.Fatalf("want exactly 1 refresh retry (2 dials), got %d", dialCalls)
	}
	if tokenCalls != 2 {
		t.Fatalf("want initial + refresh token calls, got %d", tokenCalls)
	}
}

// TestStreamWithRetryStopsOn403: forbidden (e.g. unlisted IP) is permanent on
// the first handshake — no refresh retry, single dial.
func TestStreamWithRetryStopsOn403(t *testing.T) {
	var tokenCalls, dialCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			tokenCalls++
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		default:
			dialCalls++
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	t.Cleanup(srv.Close)

	c := streamClient(t, srv)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := c.StreamWithRetry(ctx,
		[]Subscription{{Type: "trade:us", Codes: []string{"AAPL"}}},
		func(StreamFrame) {}, nil)
	if !errors.Is(err, ErrStreamUnauthorized) {
		t.Fatalf("want permanent unauthorized error, got %v", err)
	}
	if dialCalls != 1 || tokenCalls != 1 {
		t.Fatalf("403 must not refresh or redial: dials=%d tokens=%d", dialCalls, tokenCalls)
	}
}

// TestStreamResetBackoff pins the healthy-connection rule: long-lived
// connections reset the backoff, flapping ones keep growing it.
func TestStreamResetBackoff(t *testing.T) {
	if streamResetBackoff(59 * time.Second) {
		t.Error("59s uptime must keep backing off")
	}
	if !streamResetBackoff(time.Minute) || !streamResetBackoff(10*time.Minute) {
		t.Error("uptime at/above the threshold must reset the backoff")
	}
}
