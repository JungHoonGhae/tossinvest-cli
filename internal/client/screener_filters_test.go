package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실데이터 아님.
func filterServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/screener/filters/range":
			// range 는 filter 가 중첩 객체다. 평평한 filterId 를 보내면 서버가 400 을
			// 주고, 그게 이 엔드포인트가 오래 안 풀렸던 이유다.
			var body struct {
				Filter struct {
					ID string `json:"id"`
				} `json:"filter"`
				Nation string `json:"nation"`
			}
			if err := json.Unmarshal(raw, &body); err != nil || body.Filter.ID == "" {
				t.Fatalf("range body not nested: %s", raw)
			}
			if body.Filter.ID == "기간필요" {
				w.WriteHeader(400)
				w.Write([]byte(`{"error":{"statusCode":400,"code":"screener.invalid.filter-condition-period"}}`))
				return
			}
			w.Write([]byte(`{"result":{"min":-8021.27,"max":6973.80}}`))
		case "/api/v1/screener/filters/base":
			// base 는 반대로 평평한 filterId 를 받는다.
			var body struct {
				FilterID string `json:"filterId"`
			}
			if err := json.Unmarshal(raw, &body); err != nil || body.FilterID == "" {
				t.Fatalf("base body not flat: %s", raw)
			}
			w.Write([]byte(`{"result":{"basedAt":"2026-01-02"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestGetScreenerFilterRanges(t *testing.T) {
	server := filterServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetScreenerFilterRanges(t.Context(), []string{"PER", "PBR"}, "")
	if err != nil {
		t.Fatalf("GetScreenerFilterRanges() error = %v", err)
	}
	if got.Nation != "kr" {
		t.Errorf("Nation = %q, want kr (default)", got.Nation)
	}
	if len(got.Filters) != 2 {
		t.Fatalf("len(Filters) = %d, want 2", len(got.Filters))
	}
	f := got.Filters[0]
	if f.Min == nil || f.Max == nil || *f.Min != -8021.27 {
		t.Errorf("range not carried: %+v", f)
	}
	if f.BasedAt != "2026-01-02" {
		t.Errorf("BasedAt = %q", f.BasedAt)
	}
}

// 조건이 더 필요한 필터 하나가 400 이어도 나머지는 살아야 한다 — 열 개를 물었으면
// 답한 여덟 개는 받아야 쓸모가 있다.
func TestGetScreenerFilterRangesPartialFailure(t *testing.T) {
	server := filterServer(t)
	defer server.Close()

	got, err := testClientFor(server).GetScreenerFilterRanges(t.Context(), []string{"기간필요", "PER"}, "kr")
	if err != nil {
		t.Fatalf("one bad filter aborted the call: %v", err)
	}
	if len(got.Filters) != 2 {
		t.Fatalf("len(Filters) = %d, want 2", len(got.Filters))
	}
	// 서버 원문 코드를 그대로 — 토스가 매핑을 공개하지 않아 번역하면 추측이 된다.
	if got.Filters[0].Unavailable != "screener.invalid.filter-condition-period" {
		t.Errorf("reason = %q", got.Filters[0].Unavailable)
	}
	if got.Filters[0].Min != nil {
		t.Error("failed filter carries a range")
	}
	if got.Filters[1].Min == nil {
		t.Error("good filter lost its range")
	}
}

func TestGetScreenerFilterRangesRequiresIDs(t *testing.T) {
	server := filterServer(t)
	defer server.Close()
	if _, err := testClientFor(server).GetScreenerFilterRanges(t.Context(), nil, "kr"); err == nil {
		t.Fatal("want error for empty filter list")
	}
}
