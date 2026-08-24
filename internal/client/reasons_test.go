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

// 전부 합성 더미.
//
// 이 엔드포인트의 함정: **서버가 사유 없는 종목을 응답에서 뺀다.** 요청 3개에 응답 2개가
// 오고 순서도 보장되지 않는다. 위치로 맞추면 종목과 사유가 어긋난 채 출력된다.
func TestGetStockReasonsMatchesByCodeNotPosition(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "want POST", http.StatusMethodNotAllowed)
			return
		}
		gotBody, _ = io.ReadAll(r.Body)
		// 요청은 A000001, A000002, A000003 순. 응답은 하나가 빠지고 순서도 뒤집혔다.
		_, _ = w.Write([]byte(`{"result":{"signals":[
 {"productCode":"A000003","reasoningDescription":"세 번째 사유"},
 {"productCode":"A000001","reasoningDescription":"첫 번째 사유"}
]}}`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	got, err := c.GetStockReasons(context.Background(),
		[]string{"A000001", "A000002", "A000003"})
	if err != nil {
		t.Fatalf("GetStockReasons: %v", err)
	}

	var sent struct {
		ProductCodes []string `json:"productCodes"`
	}
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body not JSON: %v", err)
	}
	if len(sent.ProductCodes) != 3 {
		t.Errorf("all 3 codes must be sent in one request, got %v", sent.ProductCodes)
	}

	if len(got.Reasons) != 2 {
		t.Fatalf("expected 2 reasons (server omits one), got %d", len(got.Reasons))
	}
	for _, r := range got.Reasons {
		switch r.ProductCode {
		case "A000001":
			if r.Description != "첫 번째 사유" || r.Symbol != "A000001" {
				t.Errorf("A000001 mismatched: %+v", r)
			}
		case "A000003":
			if r.Description != "세 번째 사유" || r.Symbol != "A000003" {
				t.Errorf("A000003 mismatched: %+v", r)
			}
		default:
			t.Errorf("unexpected code in result: %+v", r)
		}
	}
}
