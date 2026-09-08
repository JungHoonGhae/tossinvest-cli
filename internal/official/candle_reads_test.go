package official

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
)

func TestNewestFirstCandlesRenderChronologically(t *testing.T) {
	raw := apiCandlePage{Candles: []apiCandle{
		{Timestamp: "2026-09-08T09:00:00+09:00", OpenPrice: "72000", HighPrice: "73000", LowPrice: "71000", ClosePrice: "73000"},
		{Timestamp: "2026-09-07T09:00:00+09:00", OpenPrice: "71000", HighPrice: "72000", LowPrice: "70000", ClosePrice: "71000"},
	}}
	chart := adaptCandles("005930", "1d", raw)
	if !chart.Candles[0].Time.Before(chart.Candles[1].Time) || chart.Candles[1].Close != 73000 {
		t.Fatalf("not oldest first: %+v", chart.Candles)
	}
	if raw.Candles[0].ClosePrice != "73000" {
		t.Fatal("adapter mutated input")
	}
	for _, format := range []output.Format{output.FormatTable, output.FormatJSON, output.FormatCSV} {
		t.Run(string(format), func(t *testing.T) {
			var buf bytes.Buffer
			if err := output.WriteChart(&buf, format, chart); err != nil {
				t.Fatal(err)
			}
			switch format {
			case output.FormatTable:
				if !strings.Contains(strings.SplitN(buf.String(), "\n", 2)[0], "73,000") {
					t.Fatalf("wrong headline: %s", buf.String())
				}
			case output.FormatJSON:
				var decoded domain.Chart
				if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded.Candles[0].Close != 71000 || decoded.Candles[1].Close != 73000 {
					t.Fatal("JSON order changed")
				}
			case output.FormatCSV:
				rows, err := csv.NewReader(&buf).ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				if len(rows) != 3 || rows[1][4] != "71000" || rows[2][4] != "73000" {
					t.Fatalf("CSV order: %v", rows)
				}
			}
		})
	}
}

// TestAdaptCandlesUnit verifies the pure adapter for Candles.
func TestAdaptCandlesUnit(t *testing.T) {
	raw := apiCandlePage{
		Candles: []apiCandle{
			{
				Timestamp:  "2026-03-25T09:00:00+09:00",
				OpenPrice:  "71600",
				HighPrice:  "72300",
				LowPrice:   "71500",
				ClosePrice: "72000",
				Volume:     "3521000",
				Currency:   "KRW",
			},
		},
		NextBefore: "2026-03-25T08:59:59+09:00",
	}

	got := adaptCandles("005930", "1d", raw)

	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if got.Interval != "1d" {
		t.Fatalf("Interval: want 1d, got %q", got.Interval)
	}
	if len(got.Candles) != 1 {
		t.Fatalf("len(Candles): want 1, got %d", len(got.Candles))
	}
	c := got.Candles[0]
	if c.Open != 71600 {
		t.Fatalf("Open: want 71600, got %v", c.Open)
	}
	if c.High != 72300 {
		t.Fatalf("High: want 72300, got %v", c.High)
	}
	if c.Low != 71500 {
		t.Fatalf("Low: want 71500, got %v", c.Low)
	}
	if c.Close != 72000 {
		t.Fatalf("Close: want 72000, got %v", c.Close)
	}
	if c.Volume != 3521000 {
		t.Fatalf("Volume: want 3521000, got %v", c.Volume)
	}
	// Timestamp must be parsed to a non-zero time.
	if c.Time.IsZero() {
		t.Fatal("Time: expected non-zero")
	}
	// The parsed time should be 2026-03-25 00:00:00 UTC (09:00 KST = UTC+9).
	want := time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC)
	if !c.Time.UTC().Equal(want) {
		t.Fatalf("Time UTC: want %v, got %v", want, c.Time.UTC())
	}
	// Fields not available from /candles endpoint.
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
	if got.Name != "" {
		t.Fatalf("Name: expected empty, got %q", got.Name)
	}
	if got.Base != 0 {
		t.Fatalf("Base: expected 0, got %v", got.Base)
	}
}

// TestAdaptCandlesEmpty verifies empty candle slice handling.
func TestAdaptCandlesEmpty(t *testing.T) {
	got := adaptCandles("AAPL", "1m", apiCandlePage{})
	if got.Symbol != "AAPL" {
		t.Fatalf("Symbol: want AAPL, got %q", got.Symbol)
	}
	if len(got.Candles) != 0 {
		t.Fatalf("Candles: want empty, got %d", len(got.Candles))
	}
}

// TestAdaptCandlesBadTimestamp verifies that an unparseable timestamp yields
// a zero time.Time rather than a hard error.
func TestAdaptCandlesBadTimestamp(t *testing.T) {
	raw := apiCandlePage{
		Candles: []apiCandle{
			{Timestamp: "not-a-date", ClosePrice: "100"},
		},
	}
	got := adaptCandles("X", "1d", raw)
	if !got.Candles[0].Time.IsZero() {
		t.Fatalf("Time: expected zero for bad timestamp, got %v", got.Candles[0].Time)
	}
}

// TestCandlesIntegration tests Candles() against an httptest server.
func TestCandlesIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/candles":
			q := r.URL.Query()
			if q.Get("symbol") != "005930" {
				t.Errorf("symbol: want 005930, got %q", q.Get("symbol"))
			}
			if q.Get("interval") != "1d" {
				t.Errorf("interval: want 1d, got %q", q.Get("interval"))
			}
			_, _ = w.Write([]byte(`{"result":{"candles":[{"timestamp":"2026-03-25T09:00:00+09:00","openPrice":"71600","highPrice":"72300","lowPrice":"71500","closePrice":"72000","volume":"3521000","currency":"KRW"}],"nextBefore":"2026-03-25T08:59:59+09:00"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)

	got, err := c.Candles(context.Background(), "005930", "1d", 0, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Candles) != 1 {
		t.Fatalf("Candles: want 1, got %d", len(got.Candles))
	}
	if got.Candles[0].Close != 72000 {
		t.Fatalf("Close: want 72000, got %v", got.Candles[0].Close)
	}
}
