package main

import (
	"strings"
	"testing"

	tossclient "github.com/junghoonkye/tossinvest-cli/internal/client"
	"github.com/junghoonkye/tossinvest-cli/internal/config"
	"github.com/junghoonkye/tossinvest-cli/internal/trading"
)

func TestUserFacingPlaceErrorFormatsFundingGuidance(t *testing.T) {
	t.Parallel()

	err := userFacingPlaceError(rootPathsForTest(), &trading.BranchRequiredError{
		Branch:        trading.BranchFundingRequired,
		StatusCode:    422,
		BrokerMessage: "계좌 잔액이 부족해요 / 구매를 위해 21,511원을 채울게요.",
	}, &placeFlags{
		symbol:       "TSLL",
		market:       "us",
		side:         "buy",
		orderType:    "limit",
		quantity:     1,
		price:        500,
		currencyMode: "KRW",
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}

	message := err.Error()
	if !strings.Contains(message, "잔액 또는 주문가능금액이 부족") {
		t.Fatalf("expected funding guidance, got %q", message)
	}
	if !strings.Contains(message, "tossctl order preview --symbol TSLL --market us --side buy --type limit --qty 1 --price 500 --currency-mode KRW") {
		t.Fatalf("expected preview retry command, got %q", message)
	}
	if !strings.Contains(message, "--confirm <new-confirm-token>") {
		t.Fatalf("expected execute retry template, got %q", message)
	}
}

func TestUserFacingPlaceErrorFormatsFXGuidance(t *testing.T) {
	t.Parallel()

	err := userFacingPlaceError(rootPathsForTest(), &trading.BranchRequiredError{
		Branch:        trading.BranchFXConsentRequired,
		StatusCode:    500,
		BrokerMessage: "환전 후 주문하려면 외화 사용 동의가 필요해요.",
	}, &placeFlags{
		symbol:       "TSLL",
		market:       "us",
		side:         "buy",
		orderType:    "limit",
		quantity:     1,
		price:        500,
		currencyMode: "KRW",
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}

	message := err.Error()
	if !strings.Contains(message, "환전 또는 외화 사용 동의가 필요") {
		t.Fatalf("expected fx guidance, got %q", message)
	}
	if !strings.Contains(message, "Toss 앱 또는 웹에서 해당 미국주식 주문의 환전 또는 외화 사용 동의 화면으로 이동") {
		t.Fatalf("expected fx steps, got %q", message)
	}
}

func TestUserFacingPlaceErrorFormatsPostPrepareFXGuidance(t *testing.T) {
	t.Parallel()

	err := userFacingPlaceError(rootPathsForTest(), &trading.BranchRequiredError{
		Branch:     trading.BranchFXConsentRequired,
		Source:     trading.BranchSourcePostPrepareConfirmation,
		StatusCode: 200,
		FX: &trading.FXConfirmationContext{
			NeedExchangeUSD:      0.68,
			EstimatedExchangeKRW: 1020,
			USDExchangeRate:      1500.21,
			GettingBackKRWKnown:  true,
			GettingBackKRW:       false,
		},
	}, &placeFlags{
		symbol:       "TSLL",
		market:       "us",
		side:         "buy",
		orderType:    "limit",
		quantity:     1,
		price:        1000,
		currencyMode: "KRW",
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}

	message := err.Error()
	if !strings.Contains(message, "주문 준비는 통과했지만") {
		t.Fatalf("expected post-prepare intro, got %q", message)
	}
	if !strings.Contains(message, "0.68달러가 부족해요.") {
		t.Fatalf("expected need-exchange line, got %q", message)
	}
	if !strings.Contains(message, "예상 환전 금액: 1,020원") {
		t.Fatalf("expected estimated exchange amount, got %q", message)
	}
	if !strings.Contains(message, "예상 환율: 1,500.21원/USD") {
		t.Fatalf("expected exchange rate detail, got %q", message)
	}
	if !strings.Contains(message, "계좌에는 달러로 남아있어요") {
		t.Fatalf("expected retained-USD warning, got %q", message)
	}
	if !strings.Contains(message, "accept_fx_consent=true") {
		t.Fatalf("expected config guidance for automation, got %q", message)
	}
}

func TestUserFacingCommandErrorIncludesAuthEndpointAndStatus(t *testing.T) {
	t.Parallel()

	err := userFacingCommandError(&tossclient.AuthError{
		StatusCode: 403,
		Endpoint:   "https://wts-cert-api.tossinvest.com/api/v3/my-assets/summaries/markets/all/overview",
	})
	if err == nil {
		t.Fatal("expected transformed error")
	}

	text := err.Error()
	if !strings.Contains(text, "status 403") {
		t.Fatalf("expected status code in error, got %q", text)
	}
	if !strings.Contains(text, "/api/v3/my-assets/summaries/markets/all/overview") {
		t.Fatalf("expected endpoint path in error, got %q", text)
	}
	if !strings.Contains(text, "tossctl auth login") {
		t.Fatalf("expected login hint in error, got %q", text)
	}
}

func rootPathsForTest() config.Paths {
	return config.Paths{}
}
