package client

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 더미 값 — 실계좌·실시세 데이터 아님.
const cryptoBody = `{"result":[
{"productCode":"VWAP.KRW-BTC","base":100000000,"open":100000000,"high":102000000,"low":99000000,
 "close":101000000,"volume":12.5,"value":1250000000,"changeType":"UP",
 "high52w":150000000,"low52w":80000000,"usdPerKrwExchangeRate":0.0007,
 "premium":-400000,"premiumRate":-0.4},
{"productCode":"VWAP.KRW-ETH","base":2000000,"open":2000000,"high":2010000,"low":1980000,
 "close":1990000,"volume":300,"value":600000000,"changeType":"DOWN",
 "high52w":5000000,"low52w":1500000,"usdPerKrwExchangeRate":0.0007,
 "premium":8000,"premiumRate":0.4}]}`

func cryptoServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/crypto-prices" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("productCodes"); got != "VWAP.KRW-BTC,VWAP.KRW-ETH" {
			t.Fatalf("productCodes = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
}

func TestGetCryptoPrices(t *testing.T) {
	server := cryptoServer(t, cryptoBody)
	defer server.Close()

	got, err := testClientFor(server).GetCryptoPrices(t.Context(), []string{"BTC", "eth"})
	if err != nil {
		t.Fatalf("GetCryptoPrices() error = %v", err)
	}
	if len(got.Prices) != 2 {
		t.Fatalf("len(Prices) = %d, want 2", len(got.Prices))
	}

	btc := got.Prices[0]
	if btc.Symbol != "BTC" {
		t.Errorf("Symbol = %q, want BTC", btc.Symbol)
	}
	if btc.ProductCode != "VWAP.KRW-BTC" {
		t.Errorf("ProductCode = %q", btc.ProductCode)
	}
	// 서버는 등락을 안 준다 — base 대비로 계산한다.
	if btc.Change != 1_000_000 {
		t.Errorf("Change = %v, want 1000000", btc.Change)
	}
	// 퍼센트 스케일: 1.0 은 1%다. ×100 을 두 번 하면 여기서 걸린다.
	if math.Abs(btc.ChangeRate-1.0) > 1e-9 {
		t.Errorf("ChangeRate = %v, want 1.0", btc.ChangeRate)
	}

	// 프리미엄은 부호를 유지한다 — 음수는 국내가 글로벌보다 싸다는 뜻이라
	// 절대값으로 바꾸면 의미가 뒤집힌다.
	if btc.PremiumRate != -0.4 {
		t.Errorf("PremiumRate = %v, want -0.4", btc.PremiumRate)
	}
	if got.Prices[1].PremiumRate != 0.4 {
		t.Errorf("ETH PremiumRate = %v, want 0.4", got.Prices[1].PremiumRate)
	}
	if got.FetchedAt.IsZero() {
		t.Error("FetchedAt not set")
	}
}

// base 가 0 이면 등락률은 0/0 이다. 서버가 신규 상장 등으로 0 을 줄 수 있으니
// NaN/Inf 를 출력으로 흘리지 않는지 본다.
func TestGetCryptoPricesZeroBase(t *testing.T) {
	server := cryptoServer(t, `{"result":[{"productCode":"VWAP.KRW-BTC","base":0,"close":100}]}`)
	defer server.Close()

	got, err := testClientFor(server).GetCryptoPrices(t.Context(), []string{"BTC", "ETH"})
	if err != nil {
		t.Fatalf("GetCryptoPrices() error = %v", err)
	}
	if r := got.Prices[0].ChangeRate; r != 0 {
		t.Errorf("ChangeRate = %v, want 0 (not NaN/Inf)", r)
	}
}

func TestGetCryptoPricesRequiresSymbols(t *testing.T) {
	server := cryptoServer(t, cryptoBody)
	defer server.Close()

	if _, err := testClientFor(server).GetCryptoPrices(t.Context(), nil); err == nil {
		t.Fatal("want error for empty symbols, got nil")
	}
}
