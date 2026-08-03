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

func autoTradeSrv(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/auto-trading/histories") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
}

// 상태 코드는 그 자체로 아무 뜻도 없다. 이름으로 바꿔주지 않으면 "status 6" 을
// 보고 켜져 있는지 끝났는지 알 수 없다.
func TestListAutoTradesTranslatesStatus(t *testing.T) {
	srv := autoTradeSrv(t, `{"result":{"lastPage":false,"body":[
	 {"id":1,"type":"STOP_LOSS","status":"6","productCode":"DUMMY001","productName":null,
	  "marketDivision":"us","createdAt":"2026-01-01 00:00:00.000",
	  "settings":[{"quantity":{"amount":10,"allTrade":true},
	               "target":{"price":1000,"currencyType":"KRW"},
	               "order":{"price":990,"tradeType":"sell"}}]},
	 {"id":2,"type":"PROFIT_RATE","status":"1","productCode":"DUMMY002",
	  "marketDivision":"kr","settings":[]}]}}`)
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.ListAutoTrades(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("2건이어야 한다: %d", len(got.Items))
	}
	a := got.Items[0]
	if a.Status != "EXPIRED" || a.StatusCode != "6" {
		t.Errorf("상태 변환 실패: %q / %q", a.Status, a.StatusCode)
	}
	if a.Quantity != 10 || !a.AllQuantity {
		t.Errorf("수량이 유실됐다: %+v", a)
	}
	if a.TriggerPrice != 1000 || a.OrderPrice != 990 || a.Currency != "KRW" {
		t.Errorf("가격이 유실됐다: %+v", a)
	}
	if got.Items[1].Status != "READY" {
		t.Errorf("두 번째 상태: want READY, got %q", got.Items[1].Status)
	}
	// lastPage=false 는 뒤가 더 있다는 뜻이다. 삼키면 목록이 전부인 줄 안다.
	if !got.HasNext {
		t.Error("HasNext 가 false — 다음 페이지가 있다는 사실이 사라졌다")
	}
}

// 토스가 상태를 추가해도 그 규칙이 목록에서 사라지면 안 된다. 모르면 모른다고
// 표시하되 행은 남긴다.
func TestListAutoTradesKeepsUnknownStatus(t *testing.T) {
	srv := autoTradeSrv(t, `{"result":{"lastPage":true,"body":[
	 {"id":9,"type":"OCO","status":"99","productCode":"DUMMY003","settings":[]}]}}`)
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.ListAutoTrades(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("모르는 상태라고 행이 사라졌다: %d", len(got.Items))
	}
	if !strings.Contains(got.Items[0].Status, "UNKNOWN") || got.Items[0].StatusCode != "99" {
		t.Errorf("모르는 상태 표기가 틀렸다: %+v", got.Items[0])
	}
}
