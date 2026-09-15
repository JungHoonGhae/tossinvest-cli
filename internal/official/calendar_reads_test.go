package official

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarketCalendarIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/market-calendar/KR":
			if got := r.URL.Query().Get("date"); got != "2026-03-25" {
				t.Errorf("date query = %q, want 2026-03-25", got)
			}
			_, _ = w.Write([]byte(`{"result":{"today":{"date":"2026-03-25"}}}`))
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

	got, err := c.MarketCalendar(context.Background(), "kr", "2026-03-25")
	if err != nil {
		t.Fatal(err)
	}
	if got.Country != "KR" {
		t.Errorf("Country = %q, want KR", got.Country)
	}
	if got.Today.Date != "2026-03-25" {
		t.Fatalf("unexpected calendar payload: %+v", got)
	}
	// 세션이 하나도 없는 날은 휴장이다.
	if !got.Today.Holiday {
		t.Error("a day with no sessions must be reported as a holiday")
	}
}

func TestMarketCalendarRejectsBadCountry(t *testing.T) {
	c := New(Credentials{APIKey: "k"}, filepath.Join(t.TempDir(), "t.json"))
	if _, err := c.MarketCalendar(context.Background(), "JP", ""); err == nil {
		t.Fatal("expected error for unsupported country")
	}
}

// 두 시장이 같은 정보를 다른 모양으로 준다 — KR 은 세션을 integrated 아래
// 중첩하고 단일가 시각을 붙이며, US 는 최상위에 평탄하게 두고 데이마켓을
// 추가한다. 아래 페이로드는 공식 spec 의 예시 형태 그대로다.
func calendarServer(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case path:
			_, _ = w.Write([]byte(body))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newCalendarClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

func TestMarketCalendarNormalizesKR(t *testing.T) {
	srv := calendarServer(t, "/api/v1/market-calendar/KR", `{"result":{"today":{
      "date":"2026-03-25",
      "integrated":{
        "preMarket":{"startTime":"2026-03-25T08:00:00+09:00","endTime":"2026-03-25T09:00:00+09:00","singlePriceAuctionStartTime":"2026-03-25T08:50:00+09:00"},
        "regularMarket":{"startTime":"2026-03-25T09:00:00+09:00","endTime":"2026-03-25T15:30:00+09:00","singlePriceAuctionStartTime":"2026-03-25T15:20:00+09:00"},
        "afterMarket":{"startTime":"2026-03-25T15:30:00+09:00","endTime":"2026-03-25T20:00:00+09:00","singlePriceAuctionEndTime":"2026-03-25T15:40:00+09:00"}}}}}`)
	defer srv.Close()

	got, err := newCalendarClient(t, srv).MarketCalendar(context.Background(), "KR", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Today.Holiday {
		t.Fatal("a day with sessions must not be a holiday")
	}
	var names []string
	for _, s := range got.Today.Sessions {
		names = append(names, s.Name)
	}
	want := []string{"pre_market", "regular_market", "after_market"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("sessions = %v, want %v (거래 순서)", names, want)
	}
	if got.Today.Sessions[0].SinglePriceAuctionStart == "" {
		t.Error("KR 단일가 시각이 유실됐다")
	}
}

// Synthetic sessions exercise the OpenAPI 1.2.17 union/null contract, not
// exchange schedules. Bounds must come from the response, not local constants.
func TestMarketCalendarKRAfterMarketUnion(t *testing.T) {
	for _, tc := range []struct {
		name, integrated, start, end, auctionEnd string
		wantSessions                             int
		holiday                                  bool
	}{
		{
			name:       "KRX open, NXT closed",
			integrated: `{"afterMarket":{"startTime":"2026-09-15T16:00:00+09:00","endTime":"2026-09-15T18:00:00+09:00","singlePriceAuctionEndTime":null}}`,
			start:      "2026-09-15T16:00:00+09:00", end: "2026-09-15T18:00:00+09:00", wantSessions: 1,
		},
		{
			name:       "NXT open, KRX closed",
			integrated: `{"afterMarket":{"startTime":"2026-09-15T15:30:00+09:00","endTime":"2026-09-15T20:00:00+09:00","singlePriceAuctionEndTime":"2026-09-15T15:40:00+09:00"}}`,
			start:      "2026-09-15T15:30:00+09:00", end: "2026-09-15T20:00:00+09:00", auctionEnd: "2026-09-15T15:40:00+09:00", wantSessions: 1,
		},
		{
			name:       "both open, server union preserved",
			integrated: `{"afterMarket":{"startTime":"2026-09-15T15:45:00+09:00","endTime":"2026-09-15T20:15:00+09:00","singlePriceAuctionEndTime":"2026-09-15T15:55:00+09:00"}}`,
			start:      "2026-09-15T15:45:00+09:00", end: "2026-09-15T20:15:00+09:00", auctionEnd: "2026-09-15T15:55:00+09:00", wantSessions: 1,
		},
		{
			name:         "both after-markets closed, regular session open",
			integrated:   `{"regularMarket":{"startTime":"2026-09-15T09:00:00+09:00","endTime":"2026-09-15T15:30:00+09:00"},"afterMarket":null}`,
			wantSessions: 1,
		},
		{
			name: "both exchanges closed all day", integrated: `null`, holiday: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := calendarServer(t, "/api/v1/market-calendar/KR",
				`{"result":{"today":{"date":"2026-09-15","integrated":`+tc.integrated+`}}}`)
			defer srv.Close()
			got, err := newCalendarClient(t, srv).MarketCalendar(context.Background(), "KR", "2026-09-15")
			if err != nil {
				t.Fatal(err)
			}
			if got.Today.Holiday != tc.holiday || len(got.Today.Sessions) != tc.wantSessions {
				t.Fatalf("today = %+v, want holiday=%v, sessions=%d", got.Today, tc.holiday, tc.wantSessions)
			}
			found := false
			for _, session := range got.Today.Sessions {
				if session.Name != "after_market" {
					continue
				}
				found = true
				if session.Start != tc.start || session.End != tc.end || session.SinglePriceAuctionEnd != tc.auctionEnd {
					t.Errorf("after_market = %+v, want start=%s end=%s auctionEnd=%s", session, tc.start, tc.end, tc.auctionEnd)
				}
				encoded, err := json.Marshal(session)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(encoded, &fields); err != nil {
					t.Fatal(err)
				}
				if value, exists := fields["single_price_auction_end"]; exists != (tc.auctionEnd != "") || (exists && value != tc.auctionEnd) {
					t.Errorf("auction end JSON = %s", encoded)
				}
			}
			if found != (tc.start != "") {
				t.Errorf("after_market present=%v, want %v", found, tc.start != "")
			}
		})
	}
}

func TestMarketCalendarNormalizesUS(t *testing.T) {
	srv := calendarServer(t, "/api/v1/market-calendar/US", `{"result":{"today":{
      "date":"2026-03-25",
      "preMarket":{"startTime":"2026-03-25T18:00:00+09:00","endTime":"2026-03-25T22:30:00+09:00"},
      "dayMarket":{"startTime":"2026-03-25T14:00:00+09:00","endTime":"2026-03-25T18:00:00+09:00"},
      "regularMarket":{"startTime":"2026-03-25T22:30:00+09:00","endTime":"2026-03-26T05:00:00+09:00"},
      "afterMarket":{"startTime":"2026-03-26T05:00:00+09:00","endTime":"2026-03-26T09:00:00+09:00"}}}}`)
	defer srv.Close()

	got, err := newCalendarClient(t, srv).MarketCalendar(context.Background(), "US", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range got.Today.Sessions {
		names = append(names, s.Name)
		if s.SinglePriceAuctionStart != "" || s.SinglePriceAuctionEnd != "" {
			t.Errorf("US 에는 단일가가 없다 — 지어내면 안 된다: %+v", s)
		}
	}
	// 미국에만 있는 데이마켓이 빠지면 안 된다.
	if strings.Join(names, ",") != "pre_market,day_market,regular_market,after_market" {
		t.Errorf("sessions = %v", names)
	}
}

// 휴장 표현이 시장마다 다르다 — KR 은 integrated 를 null 로, US 는 세션을
// 전부 비워서. 호출자가 어느 시장에 물었는지 몰라도 되게 한 플래그다.
func TestMarketCalendarHolidayBothMarkets(t *testing.T) {
	for _, tc := range []struct{ country, body string }{
		{"KR", `{"result":{"today":{"date":"2026-05-05","integrated":null}}}`},
		{"US", `{"result":{"today":{"date":"2026-07-03"}}}`},
	} {
		t.Run(tc.country, func(t *testing.T) {
			srv := calendarServer(t, "/api/v1/market-calendar/"+tc.country, tc.body)
			defer srv.Close()
			got, err := newCalendarClient(t, srv).MarketCalendar(context.Background(), tc.country, "")
			if err != nil {
				t.Fatal(err)
			}
			if !got.Today.Holiday {
				t.Errorf("%s 휴장이 인식되지 않았다: %+v", tc.country, got.Today)
			}
			if len(got.Today.Sessions) != 0 {
				t.Errorf("휴장인데 세션이 있다: %+v", got.Today.Sessions)
			}
		})
	}
}
