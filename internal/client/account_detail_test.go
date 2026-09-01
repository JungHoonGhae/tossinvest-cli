package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미다.

// 미국 배당 수령 방식은 계좌 설정인데 웹 화면이 없어서(모바일 전용) 눈으로
// 확인할 방법이 없었다. 현금으로 받는지 주식으로 재투자되는지는 세금과 보유
// 수량이 갈리는 선택이라 조회 가치가 있다.
func TestAccountDetailIncludesUSDividendOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/detail":
			_, _ = w.Write([]byte(`{"result":{"no":"000-00-000000","status":"00",
				"openDate":"2020-01-01","lastTradeDate":"2026-01-02","accountName":"더미"}}`))
		case "/api/v1/rights/us/dividend-option/account-give-type":
			_, _ = w.Write([]byte(`{"result":{"accountNo":"000-00-000000",
				"giveType":"STOCK","updateDate":"2026-02-03"}}`))
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), APIBaseURL: srv.URL, CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetAccountDetail(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.USDividendOption == nil {
		t.Fatal("미국 배당 수령 방식이 빠졌다")
	}
	if got.USDividendOption.GiveType != "STOCK" {
		t.Errorf("GiveType: want STOCK, got %q", got.USDividendOption.GiveType)
	}
	if got.USDividendOption.UpdatedAt != "2026-02-03" {
		t.Errorf("UpdatedAt: want 2026-02-03, got %q", got.USDividendOption.UpdatedAt)
	}
}

// 이 엔드포인트가 죽어도 계좌 상세 전체가 죽으면 안 된다 — 나머지 선택 항목과
// 같은 취급이다.
func TestAccountDetailSurvivesDividendOptionFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/account/detail" {
			_, _ = w.Write([]byte(`{"result":{"no":"000-00-000000","status":"00"}}`))
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), APIBaseURL: srv.URL, CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetAccountDetail(context.Background())
	if err != nil {
		t.Fatalf("배당 옵션 실패가 전체를 죽였다: %v", err)
	}
	if got.USDividendOption != nil {
		t.Error("실패했는데 값이 채워졌다")
	}
	var found bool
	for _, w := range got.Warnings {
		if strings.Contains(w, "배당") {
			found = true
		}
	}
	if !found {
		t.Errorf("조용히 삼켰다 — 경고가 없다: %v", got.Warnings)
	}
}

// 필수(신원)만 성공하고 나머지가 실패해도 커맨드는 살아야 한다 —
// 미수거래 엔드포인트 하나 때문에 계좌번호를 못 보는 건 말이 안 된다.
func TestAccountDetailDegradesOnOptionalFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/account/detail":
			_, _ = w.Write([]byte(`{"result":{"no":"000-00-000000","status":"00",
				"openDate":"2020-01-01","lastTradeDate":"2026-01-02","accountName":"더미"}}`))
		case "/api/v1/transfer/withdrawable-status":
			_, _ = w.Write([]byte(`{"result":{"withdrawableAmount":{"day0":1,"day1":2,"day2":3},
				"withdrawableAmountLimit":{"perTransaction":10,"perDay":100,"sumOfTodayWithdrawals":5},
				"possibleDateOfFullWithdrawal":"2026-01-05"}}`))
		default:
			http.Error(w, "boom", http.StatusInternalServerError) // 나머지는 전부 실패
		}
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), APIBaseURL: srv.URL, CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetAccountDetail(context.Background())
	if err != nil {
		t.Fatalf("선택 항목 실패로 전체가 실패하면 안 된다: %v", err)
	}
	if got.Number != "000-00-000000" || got.OpenedAt != "2020-01-01" {
		t.Errorf("신원 정보가 유실됐다: %+v", got)
	}
	if got.Withdrawable == nil || got.Withdrawable.Day0 != 1 {
		t.Error("성공한 선택 항목(출금)이 유실됐다")
	}
	if got.MarginKR != nil || got.DifferentialMargin != nil {
		t.Error("실패한 항목은 nil 이어야 한다")
	}
	if len(got.Warnings) == 0 {
		t.Fatal("실패를 조용히 삼키면 안 된다 — 경고로 알려야 한다")
	}
	for _, want := range []string{"미수거래", "차등증거금", "송금 한도"} {
		found := false
		for _, wn := range got.Warnings {
			if strings.Contains(wn, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("경고에 %q 가 없다: %v", want, got.Warnings)
		}
	}
}

// 신원 조회가 실패하면 그건 진짜 실패다.
func TestAccountDetailFailsWhenIdentityFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(Config{HTTPClient: srv.Client(), APIBaseURL: srv.URL, CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	if _, err := c.GetAccountDetail(context.Background()); err == nil {
		t.Fatal("신원 조회 실패는 커맨드 실패여야 한다")
	}
}
