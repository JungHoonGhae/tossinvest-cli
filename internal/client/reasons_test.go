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
		// 요청은 A000001, A000002, A000003 순. 응답은 하나가 빠지고 순서도
		// 뒤집혔으며, 요청하지 않은 행도 섞였다.
		_, _ = w.Write([]byte(`{"result":{"signals":[
 {"productCode":"A000003","reasoningDescription":"세 번째 사유"},
 {"productCode":"A999999","reasoningDescription":"요청하지 않은 사유"},
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
	if got.Reasons[0].ProductCode != "A000001" || got.Reasons[1].ProductCode != "A000003" {
		t.Fatalf("reasons must follow request order, got %v then %v", got.Reasons[0].ProductCode, got.Reasons[1].ProductCode)
	}
	if len(got.Missing) != 1 || got.Missing[0] != "A000002" {
		t.Fatalf("omitted symbol must be reported, got missing=%v", got.Missing)
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

func TestGetStockReasonsPreservesAliasesResolvingToSameCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"A000001"}]}}`))
		case "/api/v1/dashboard/wts/overview/ai-signals":
			_, _ = w.Write([]byte(`{"result":{"signals":[{"productCode":"A000001","reasoningDescription":"same stock"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), InfoBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	got, err := c.GetStockReasons(context.Background(), []string{"A000001", "첫번째"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Reasons) != 2 {
		t.Fatalf("aliases resolving to one code must preserve two request positions: %+v", got)
	}
	if got.Reasons[0].Symbol != "A000001" || got.Reasons[1].Symbol != "첫번째" {
		t.Fatalf("request identities were overwritten: %+v", got.Reasons)
	}
}
