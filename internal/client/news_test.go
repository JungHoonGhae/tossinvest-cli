package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미다.

func newsClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
}

func TestNewsScopeResolution(t *testing.T) {
	cases := map[string]string{
		"":              "ALL_HIGHLIGHT", // 기본
		"all":           "ALL_HIGHLIGHT",
		"watchlist":     "PERSONALIZE_WATCH",
		"holdings":      "PERSONALIZE_HOLD",
		"latest":        "HOT",           // 서버 이름이 오해를 부르는 지점
		"soaring":       "SOARING_STOCK", // 급상승은 HOT 이 아니다
		"WATCHLIST":     "PERSONALIZE_WATCH",
		"ALL_HIGHLIGHT": "ALL_HIGHLIGHT", // raw enum 통과
		"FUTURE_SCOPE":  "FUTURE_SCOPE",  // 토스가 추가하면 릴리즈 없이 사용 가능
	}
	for in, want := range cases {
		got, err := NewsScope(in)
		if err != nil {
			t.Errorf("NewsScope(%q) 오류: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NewsScope(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := NewsScope("없는별칭"); err == nil {
		t.Error("소문자 오타는 거부해야 한다")
	}
}

// limit 은 서버 상한(50)으로 깎아서 보내고, 0 이면 아예 안 보낸다.
func TestMarketNewsClampsLimit(t *testing.T) {
	cases := []struct {
		limit    int
		wantSize any
		wantSent bool
	}{
		{0, nil, false},
		{5, float64(5), true},
		{999, float64(MaxNewsLimit), true},
	}
	for _, tc := range cases {
		var got map[string]any
		c := newsClient(t, func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &got)
			_, _ = w.Write([]byte(`{"result":{"type":"ALL_HIGHLIGHT","title":"모든 주요 뉴스","news":[]}}`))
		})
		if _, err := c.GetMarketNews(context.Background(), "ALL_HIGHLIGHT", tc.limit); err != nil {
			t.Fatal(err)
		}
		size, sent := got["size"]
		if sent != tc.wantSent {
			t.Errorf("limit=%d: size 전송 여부 = %v, want %v", tc.limit, sent, tc.wantSent)
		}
		if tc.wantSent && size != tc.wantSize {
			t.Errorf("limit=%d: size = %v, want %v", tc.limit, size, tc.wantSize)
		}
	}
}

// 응답의 title 을 그대로 실어 나른다 — 한글 라벨을 코드에 박지 않기 위함.
func TestMarketNewsCarriesServerTitleAndStocks(t *testing.T) {
	c := newsClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"type":"PERSONALIZE_WATCH","title":"관심 뉴스","news":[
			{"newsId":"n1","title":"더미 헤드라인","summary":"더미 요약","createdAt":"2026-07-25 09:00:00",
			 "source":"더미통신","newsType":"ARTICLE","nation":"kr",
			 "relatedStocks":[{"stockCode":"A000000","stockName":"더미종목","market":"KOSPI","fluctuation":2.15}]}]}}`))
	})
	got, err := c.GetMarketNews(context.Background(), "PERSONALIZE_WATCH", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "관심 뉴스" {
		t.Errorf("Title = %q — 서버 라벨을 그대로 써야 한다", got.Title)
	}
	if len(got.Items) != 1 || got.Items[0].Summary != "더미 요약" {
		t.Fatalf("items = %+v", got.Items)
	}
	st := got.Items[0].Stocks
	if len(st) != 1 || st[0].Fluctuation != 2.15 {
		t.Errorf("관련 종목 등락률이 유실됐다: %+v — 이 피드의 핵심 값이다", st)
	}
}
