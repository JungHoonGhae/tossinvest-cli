package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// 더미 값 — 실제 상장 정보 아님.
func universeServer(t *testing.T, capture *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		if capture != nil {
			*capture = r.URL.RawQuery
		}
		_, _ = w.Write([]byte(`{"result":[
		  {"symbol":"AAA","name":"더미가","isinCode":"KR0000000001","securityType":"STOCK","isCommonShare":true},
		  {"symbol":"BBB","name":"더미나","isinCode":"KR0000000002","securityType":"ETF","isCommonShare":false}]}`))
	}))
}

func universeClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

func TestListStocks(t *testing.T) {
	var query string
	srv := universeServer(t, &query)
	defer srv.Close()

	got, err := universeClient(t, srv).ListStocks(context.Background(), "kospi", "", "", false)
	if err != nil {
		t.Fatalf("ListStocks: %v", err)
	}
	// 소문자로 넣어도 대문자로 정규화돼야 한다 — enum 이라 서버가 거절한다.
	if query != "market=KOSPI" {
		t.Errorf("query = %q, want market=KOSPI only", query)
	}
	if got.Market != "KOSPI" || len(got.Stocks) != 2 {
		t.Fatalf("got %+v", got)
	}
	// 서버가 symbol 오름차순으로 준다 — 재정렬하지 않는다.
	if got.Stocks[0].Symbol != "AAA" || got.Stocks[1].Symbol != "BBB" {
		t.Errorf("order changed: %+v", got.Stocks)
	}
	if !got.Stocks[0].CommonShare || got.Stocks[1].CommonShare {
		t.Errorf("commonShare crossed: %+v", got.Stocks)
	}
}

// commonShare=false 를 실어 보내면 서버 기본값을 덮어쓸 수 있다. 스펙에 그 기본값이
// 없으므로 켤 때만 보낸다.
func TestListStocksOmitsUnsetFilters(t *testing.T) {
	var query string
	srv := universeServer(t, &query)
	defer srv.Close()

	if _, err := universeClient(t, srv).ListStocks(context.Background(), "NASDAQ", "ACTIVE", "ETF", true); err != nil {
		t.Fatalf("ListStocks: %v", err)
	}
	for _, want := range []string{"market=NASDAQ", "status=ACTIVE", "securityType=ETF", "commonShare=true"} {
		if !contains(query, want) {
			t.Errorf("query %q missing %q", query, want)
		}
	}
}

func TestListStocksRejectsUnknownMarket(t *testing.T) {
	srv := universeServer(t, nil)
	defer srv.Close()
	_, err := universeClient(t, srv).ListStocks(context.Background(), "KOSPI200", "", "", false)
	if err == nil {
		t.Fatal("want error for unknown market")
	}
	// 에러가 지원 목록을 담아야 한다 — 어휘를 모르면 고칠 수 없다.
	if !contains(err.Error(), "KOSDAQ") {
		t.Errorf("error lacks the accepted list: %v", err)
	}
}

func TestListStocksRequiresMarket(t *testing.T) {
	srv := universeServer(t, nil)
	defer srv.Close()
	if _, err := universeClient(t, srv).ListStocks(context.Background(), "  ", "", "", false); err == nil {
		t.Fatal("want error for empty market")
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (hay == needle || len(needle) == 0 ||
		func() bool {
			for i := 0; i+len(needle) <= len(hay); i++ {
				if hay[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}())
}
