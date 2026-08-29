package hybrid

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

// FX 는 유일하게 방향이 반대인 라우팅이다. 다른 오버라이드는 official 을 먼저
// 부르지만, 여기서는 그러면 **두 자격증명을 가진 사용자가 받던 행이 줄어든다** —
// WTS 는 전체 피드를, official 은 한 쌍만 준다.

func officialFX(t *testing.T, srv *httptest.Server) *official.Client {
	t.Helper()
	return official.New(
		official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
}

func TestExchangeRatesPrefersWTSAndDoesNotCallOfficial(t *testing.T) {
	wtsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"exchangeRates":[
          {"code":"FX@KRW/USD","name":"원/달러","base":1379.0,"close":1380.5},
          {"code":"FX@DXY","name":"달러인덱스","base":98.0,"close":98.2}]}}`))
	}))
	defer wtsSrv.Close()

	offSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "exchange-rate") {
			t.Error("WTS 가 답하는데 official 을 불렀다 — 행이 줄어든다")
		}
		_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer offSrv.Close()

	var buf bytes.Buffer
	c := New(client.New(client.Config{InfoBaseURL: wtsSrv.URL}), officialFX(t, offSrv),
		Policy{Prefer: "auto", Fallback: true}, &buf)

	got, err := c.GetExchangeRates(context.Background())
	if err != nil {
		t.Fatalf("GetExchangeRates: %v", err)
	}
	if len(got.Rates) < 2 {
		t.Errorf("WTS 전체 피드를 그대로 돌려줘야 한다, got %d rows", len(got.Rates))
	}
	if buf.Len() != 0 {
		t.Errorf("폴백이 없었으므로 stderr 는 조용해야 한다: %q", buf.String())
	}
}

// 공식 키만 있고 웹 세션이 없는 사용자는 `market fx` 에서 아무것도 못 받았다 —
// 커맨드의 "both" 표기가 사실이 아니었다. 한 쌍이라도 돌려준다.
func TestExchangeRatesFallsBackToOfficialWhenWTSFails(t *testing.T) {
	wtsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer wtsSrv.Close()

	offSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "token") {
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"baseCurrency":"USD","quoteCurrency":"KRW","exchangeRate":1380.5}}`))
	}))
	defer offSrv.Close()

	var buf bytes.Buffer
	c := New(client.New(client.Config{InfoBaseURL: wtsSrv.URL}), officialFX(t, offSrv),
		Policy{Prefer: "auto", Fallback: true}, &buf)

	got, err := c.GetExchangeRates(context.Background())
	if err != nil {
		t.Fatalf("official 폴백이 동작해야 한다: %v", err)
	}
	if len(got.Rates) != 1 {
		t.Fatalf("official 은 한 쌍을 준다, got %d", len(got.Rates))
	}
	// 줄어든 사실이 보여야 한다 — 조용히 적게 주면 사용자가 전체인 줄 안다.
	if !strings.Contains(buf.String(), "USD/KRW") {
		t.Errorf("축소 사실을 알려야 한다: %q", buf.String())
	}
}

// official 자격증명이 없으면 폴백할 곳이 없다. WTS 에러를 그대로 올려야
// 사용자가 "로그인하라" 는 고칠 수 있는 정보를 받는다.
func TestExchangeRatesWithoutOfficialSurfacesWTSError(t *testing.T) {
	wtsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer wtsSrv.Close()

	c := New(client.New(client.Config{InfoBaseURL: wtsSrv.URL}), nil,
		Policy{Prefer: "auto", Fallback: true}, &bytes.Buffer{})
	if _, err := c.GetExchangeRates(context.Background()); err == nil {
		t.Fatal("official 이 없으면 WTS 에러가 올라와야 한다")
	}
}
