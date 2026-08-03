package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미. 경제 일정은 시장 데이터라 계좌 정보가 없지만, 테스트 픽스처
// 규칙은 그대로 지킨다.

func TestGetEconomicCalendar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/dashboard/wts/overview/calendar/economic-events" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"events":[
			{"id":{"uniqueName":"20260101_더미지표_ECONOMIC","group":"ECONOMIC"},
			 "date":"2026-01-01","time":"23:59:59.999999999","title":"더미 지표 발표"},
			{"id":{"uniqueName":"20260102_더미고용_ECONOMIC","group":"ECONOMIC"},
			 "date":"2026-01-02","time":"12:00:00","title":"더미 고용 발표"}],
			"aiSummary":{"title":"더미 요약 문장"}}}`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetEconomicCalendar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 {
		t.Fatalf("이벤트 2건이어야 한다: %d", len(got.Events))
	}
	if got.Events[0].Title != "더미 지표 발표" || got.Events[0].Date != "2026-01-01" {
		t.Errorf("첫 이벤트가 틀렸다: %+v", got.Events[0])
	}
	if got.Events[0].Group != "ECONOMIC" {
		t.Errorf("group 이 유실됐다: %+v", got.Events[0])
	}
	if got.Events[0].ID != "20260101_더미지표_ECONOMIC" {
		t.Errorf("id 가 유실됐다: %q", got.Events[0].ID)
	}
	// 23:59:59.999999999 는 시각이 아니라 "그날 중"이라는 뜻이라 비워야 한다.
	// 그대로 두면 표에 23:59 가 줄줄이 찍혀 진짜 발표 시각을 가린다.
	if got.Events[0].Time != "" {
		t.Errorf("종일 이벤트에 시각이 남았다: %q", got.Events[0].Time)
	}
	if got.Events[1].Time != "12:00" {
		t.Errorf("시각이 HH:MM 으로 안 줄었다: %q", got.Events[1].Time)
	}
	if got.Summary != "더미 요약 문장" {
		t.Errorf("aiSummary: want 더미 요약 문장, got %q", got.Summary)
	}
	if got.FetchedAt.IsZero() {
		t.Error("FetchedAt 이 비었다")
	}
}

// 서버가 요약을 빼거나 일정이 없을 때 커맨드가 죽으면 안 된다. 장이 조용한
// 주간에는 실제로 빈 응답이 온다.
func TestGetEconomicCalendarEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"events":[],"aiSummary":null}}`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetEconomicCalendar(context.Background())
	if err != nil {
		t.Fatalf("빈 일정이 에러가 되면 안 된다: %v", err)
	}
	if len(got.Events) != 0 || got.Summary != "" {
		t.Errorf("빈 응답을 잘못 읽었다: %+v", got)
	}
}
