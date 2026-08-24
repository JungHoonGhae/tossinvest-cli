package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미.
const dummyBadged = `{"result":{"badgedIndices":[
 {"indexCode":"KGG01P","displayName":"더미지수","category":"INDEX","direction":"DOWN",
  "isAnomaly":true,"changeRate":-3.5,"aiSignalTitle":"왜 떨어졌을까?",
  "aiSignalId":"INDEX:KGG01P:20260101","keyword":"더미 키워드","zscore":2.4},
 {"indexCode":"ZZZ","displayName":"더미2","category":"COMMODITY","direction":"UP",
  "isAnomaly":false,"changeRate":0.2,"aiSignalTitle":"","aiSignalId":"","keyword":"","zscore":0}
],"marketEvents":[]}}`

func TestGetIndexAnomalies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/dashboard/wts/overview/indicator" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(dummyBadged))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	got, err := c.GetIndexAnomalies(context.Background())
	if err != nil {
		t.Fatalf("GetIndexAnomalies: %v", err)
	}
	if len(got.Indices) != 2 {
		t.Fatalf("expected 2 indices, got %d", len(got.Indices))
	}
	first := got.Indices[0]
	if !first.IsAnomaly || first.ZScore != 2.4 || first.ChangeRate != -3.5 {
		t.Errorf("anomaly fields lost: %+v", first)
	}
	if first.SignalTitle != "왜 떨어졌을까?" || first.Keyword != "더미 키워드" {
		t.Errorf("signal fields lost: %+v", first)
	}
	// 이상 아님도 그대로 나와야 한다 — 걸러내면 "평상시" 와 "조회 실패" 가 같아진다.
	if got.Indices[1].IsAnomaly {
		t.Errorf("non-anomaly must survive unflagged: %+v", got.Indices[1])
	}
}
