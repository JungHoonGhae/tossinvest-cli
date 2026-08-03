package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미.

// 랭킹과 등락이 이 피드의 값이다. 순위를 잃으면 그냥 또 하나의 뉴스 목록이 된다.
func TestGetMarketIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"updatedAt":"2026-01-01T00:00:00+09:00","issues":[
		 {"rank":1,"rankStatus":"UP","topic":"더미 토픽","topicTitle":"더미 제목",
		  "sourceCount":12,"issueCategory":"NEWS",
		  "sources":[{"sourceName":"더미통신","title":"더미 기사","createdAt":"2026-01-01"}]},
		 {"rank":2,"rankStatus":"DOWN","topic":"두번째","topicTitle":"두번째 제목","sourceCount":3}]}}`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetMarketIssues(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Issues) != 2 {
		t.Fatalf("2건이어야 한다: %d", len(got.Issues))
	}
	a := got.Issues[0]
	if a.Rank != 1 || a.RankStatus != "UP" {
		t.Errorf("랭킹 정보가 유실됐다: %+v", a)
	}
	if a.Title != "더미 제목" || a.Topic != "더미 토픽" {
		t.Errorf("토픽/제목이 뒤바뀌었다: %+v", a)
	}
	// sourceCount 는 서버 총계고 sources 는 서버가 보낸 일부다. 둘을 같은 것으로
	// 접으면 "12건 중 1건만 있다" 는 사실이 사라진다.
	if a.SourceCount != 12 || len(a.Sources) != 1 {
		t.Errorf("소스 총계와 실제 목록을 구분하지 못했다: count=%d len=%d", a.SourceCount, len(a.Sources))
	}
	if a.Sources[0].Name != "더미통신" {
		t.Errorf("소스 이름이 유실됐다: %+v", a.Sources[0])
	}
	if got.Issues[1].RankStatus != "DOWN" {
		t.Errorf("두 번째 등락: %+v", got.Issues[1])
	}
	if got.UpdatedAt == "" {
		t.Error("갱신 시각이 유실됐다")
	}
}
