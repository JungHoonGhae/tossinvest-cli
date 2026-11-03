package push

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestParseStreamExtractsDataEvents(t *testing.T) {
	raw := strings.Join([]string{
		":heartbeat",
		"",
		"retry: 3600000",
		"",
		"id: abc123",
		`data: {"type":"pending-order-refresh","key":"1"}`,
		"",
		"id: def456",
		`data: {"type":"purchase-price-refresh","msg":{"stockCode":"US20000627001"},"key":"1"}`,
		"",
		"id: skip",
		"data: not-json",
		"",
	}, "\n")

	var got []Event
	if err := parseStream(strings.NewReader(raw), func(ev Event) {
		got = append(got, ev)
	}); err != nil {
		t.Fatalf("parseStream returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 valid events, got %d: %+v", len(got), got)
	}
	if got[0].ID != "abc123" || got[0].Type != "pending-order-refresh" {
		t.Fatalf("event 0 mismatch: %+v", got[0])
	}
	if got[1].Type != "purchase-price-refresh" || got[1].Msg["stockCode"] != "US20000627001" {
		t.Fatalf("event 1 mismatch: %+v", got[1])
	}
}

func TestListenRequiresSession(t *testing.T) {
	l := NewListener(&session.Session{})
	err := l.Listen(context.Background(), func(Event) {})
	if err != ErrNoSession {
		t.Fatalf("expected ErrNoSession, got %v", err)
	}
}

func TestListenSendsCookiesAndConsumesStream(t *testing.T) {
	var sawCookie atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("SESSION"); err == nil && c.Value == "sess-token" {
			sawCookie.Store(true)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("missing Accept header: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter is not a Flusher")
		}
		// One frame then close.
		_, _ = w.Write([]byte("id: 1\n"))
		_, _ = w.Write([]byte(`data: {"type":"pending-order-refresh","key":"1"}` + "\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	l := NewListener(
		&session.Session{Cookies: map[string]string{"SESSION": "sess-token"}},
		WithStreamURL(srv.URL),
	)

	var events []Event
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := l.Listen(ctx, func(ev Event) { events = append(events, ev) }); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}

	if !sawCookie.Load() {
		t.Fatal("expected SESSION cookie on request")
	}
	if len(events) != 1 || events[0].Type != "pending-order-refresh" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestParseStreamSignalsConnectionClose(t *testing.T) {
	raw := strings.Join([]string{
		"id: 1",
		`data: {"type":"share-holdings"}`,
		"",
		"event: connection-close",
		"data: bye",
		"",
	}, "\n")

	var got []Event
	err := parseStream(strings.NewReader(raw), func(ev Event) { got = append(got, ev) })
	if err == nil || err.Error() != "push: server requested reconnect" {
		t.Fatalf("expected errConnectionClose, got %v", err)
	}
	if len(got) != 1 || got[0].Type != "share-holdings" {
		t.Fatalf("expected only the share-holdings event before close, got: %+v", got)
	}
}

func TestListenRejectsUnauthorized(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		l := NewListener(
			&session.Session{Cookies: map[string]string{"SESSION": "x"}},
			WithStreamURL(srv.URL),
		)
		err := l.Listen(context.Background(), func(Event) {})
		srv.Close()
		if err == nil || !errors.Is(err, ErrPushUnauthorized) {
			t.Fatalf("HTTP %d must be permanent, got %v", code, err)
		}
	}
}

// TestListenWithRetryDoesNotLoopOnUnauthorized guards issue #12: the retry
// loop must return after the first 401 instead of reconnecting every 60s.
func TestListenWithRetryDoesNotLoopOnUnauthorized(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	l := NewListener(
		&session.Session{Cookies: map[string]string{"SESSION": "x"}},
		WithStreamURL(srv.URL),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := l.ListenWithRetry(ctx, func(Event) {})
	if !errors.Is(err, ErrPushUnauthorized) {
		t.Fatalf("want permanent unauthorized error, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("must not retry a 401, got %d attempts", calls.Load())
	}
}

// TestIdleReaderTimesOutOnSilence: no bytes within the deadline errors out
// instead of blocking forever (half-open stall).
func TestIdleReaderTimesOutOnSilence(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	if _, err := newIdleReader(r, 50*time.Millisecond).Read(make([]byte, 8)); !errors.Is(err, errIdleTimeout) {
		t.Fatalf("want idle timeout, got %v", err)
	}
}

// TestIdleReaderPassesFlowingData: heartbeats and frames reset the deadline.
func TestIdleReaderPassesFlowingData(t *testing.T) {
	if _, err := newIdleReader(strings.NewReader("id: 1\n\n"), 5*time.Second).Read(make([]byte, 16)); err != nil {
		t.Fatalf("flowing data must not time out, got %v", err)
	}
}

// TestListenWithRetryRecoversAfterIdleStall: the first connection goes silent
// (watchdog fires), the retry connects cleanly and delivers the event.
func TestListenWithRetryRecoversAfterIdleStall(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			// Silent stall: hold the connection open with zero bytes.
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(500 * time.Millisecond)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("id: 2\n"))
		_, _ = w.Write([]byte(`data: {"type":"recovered"}` + "\n\n"))
	}))
	defer srv.Close()

	l := NewListener(
		&session.Session{Cookies: map[string]string{"SESSION": "x"}},
		WithStreamURL(srv.URL),
	)
	l.idleTimeout = 100 * time.Millisecond

	var events []Event
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := l.ListenWithRetry(ctx, func(ev Event) {
		events = append(events, ev)
		cancel()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancel after recovery, got %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("want stall + recovery (2 attempts), got %d", calls.Load())
	}
	if len(events) != 1 || events[0].Type != "recovered" {
		t.Fatalf("unexpected events: %+v", events)
	}
}
