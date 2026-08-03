package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미.

const dummyMonthly = `{"result":{"events":[
 {"id":{"group":"ECONOMIC"},"date":"2026-01-05","view":{
   "title":"더미 지표 발표",
   "subtitle":{"text":"더미 설명"},
   "economicIndicatorValue":{"unit":"Index Point","actual":null,"forecast":54.0,"historical":53.3}}},
 {"id":{"group":"USD_EARNINGS_ANNOUNCEMENT"},"date":"2026-01-06","view":{
   "title":"더미상사 실적발표","subtitle":null,
   "landingOption":{"type":"NAVIGATE","url":"/stocks/US00000000001"},
   "upcomingLive":{"liveAt":"2026-01-06T21:30:00+09:00"}}},
 {"id":{"group":"HOLIDAY"},"date":"2026-01-07","view":{"title":"국내 휴장일","subtitle":{"text":"더미 공휴일"}}},
 {"id":{"group":"BRAND_NEW_GROUP"},"date":"2026-01-08","view":{"title":"모르는 종류"}}
]}}`

func calendarServer(t *testing.T, withSummary bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v4/calendar/monthly/"):
			if r.Method != http.MethodPost {
				// GET 은 실제 서버가 405 를 준다. 우리가 POST 를 보내는지 고정한다.
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			_, _ = w.Write([]byte(dummyMonthly))
		case r.URL.Path == "/api/v1/nova-calendar/ai/summary/weekly":
			if !withSummary {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"result":{"title":"더미 주간 요약","contents":"더미 본문"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newCalendarClient(srv *httptest.Server) *Client {
	return New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
}

func TestGetMarketCalendar(t *testing.T) {
	srv := calendarServer(t, true)
	defer srv.Close()

	got, err := newCalendarClient(srv).GetMarketCalendar(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 4 {
		t.Fatalf("이벤트 4건이어야 한다: %d", len(got.Events))
	}

	ind := got.Events[0]
	if ind.Kind != "economic" || ind.Note != "더미 설명" {
		t.Errorf("지표 이벤트가 틀렸다: %+v", ind)
	}
	// 예상치·직전값이 캘린더의 값이다. 날짜만 있는 캘린더는 흔하다.
	if ind.Indicator == nil || ind.Indicator.Forecast == nil || *ind.Indicator.Forecast != 54.0 {
		t.Errorf("forecast 가 유실됐다: %+v", ind.Indicator)
	}
	// 아직 발표 전이면 actual 은 nil 이어야 한다 — 0.0 으로 접으면 "0 으로 나왔다"
	// 로 오독된다.
	if ind.Indicator.Actual != nil {
		t.Errorf("미발표 actual 이 nil 이 아니다: %v", *ind.Indicator.Actual)
	}

	earn := got.Events[1]
	if earn.Kind != "earnings_us" || earn.Symbol != "US00000000001" {
		t.Errorf("실적 이벤트의 종목이 안 붙었다: %+v", earn)
	}
	if earn.LiveAt == "" {
		t.Error("실적발표 라이브 시각이 유실됐다")
	}

	if got.Events[2].Kind != "holiday" {
		t.Errorf("휴장일 분류가 틀렸다: %+v", got.Events[2])
	}

	// 모르는 group 은 버리지 않고 other 로 넘긴다 — 토스가 종류를 추가해도
	// 조용히 사라지면 안 된다.
	unknown := got.Events[3]
	if unknown.Kind != "other" || unknown.Group != "BRAND_NEW_GROUP" {
		t.Errorf("모르는 group 처리가 틀렸다: %+v", unknown)
	}

	if got.Summary != "더미 주간 요약" || got.SummaryDetail != "더미 본문" {
		t.Errorf("주간 요약이 유실됐다: %q / %q", got.Summary, got.SummaryDetail)
	}
}

// 요약은 곁가지다. 그것 때문에 캘린더 전체가 죽으면 안 되고, 조용히 삼켜도 안 된다.
func TestGetMarketCalendarSurvivesSummaryFailure(t *testing.T) {
	srv := calendarServer(t, false)
	defer srv.Close()

	got, err := newCalendarClient(srv).GetMarketCalendar(context.Background(), "")
	if err != nil {
		t.Fatalf("요약 실패가 캘린더를 죽였다: %v", err)
	}
	if len(got.Events) != 4 {
		t.Errorf("이벤트가 유실됐다: %d", len(got.Events))
	}
	if len(got.Warnings) == 0 {
		t.Error("요약 실패를 조용히 삼켰다")
	}
}

// 요약은 이번 주 이야기라 다른 달에 붙이면 날짜가 어긋난다.
func TestGetMarketCalendarSkipsSummaryForOtherMonths(t *testing.T) {
	srv := calendarServer(t, true)
	defer srv.Close()

	got, err := newCalendarClient(srv).GetMarketCalendar(context.Background(), "2020-03")
	if err != nil {
		t.Fatal(err)
	}
	if got.Month != "2020-03" {
		t.Errorf("Month: want 2020-03, got %q", got.Month)
	}
	if got.Summary != "" {
		t.Errorf("과거 달에 이번 주 요약이 붙었다: %q", got.Summary)
	}
}

func TestGetMarketCalendarRejectsBadMonth(t *testing.T) {
	srv := calendarServer(t, true)
	defer srv.Close()

	for _, bad := range []string{"2026-1", "202608", "8월", "2026-08-01"} {
		if _, err := newCalendarClient(srv).GetMarketCalendar(context.Background(), bad); err == nil {
			t.Errorf("%q 가 통과했다", bad)
		}
	}
}
