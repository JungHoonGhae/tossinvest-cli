package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

func TestUserFacingPlaceErrorFormatsFundingGuidance(t *testing.T) {
	t.Parallel()

	err := userFacingPlaceError(rootPathsForTest(), &trading.BranchRequiredError{
		Branch:        trading.BranchFundingRequired,
		StatusCode:    422,
		BrokerMessage: "계좌 잔액이 부족해요 / 구매를 위해 21,511원을 채울게요.",
	}, &orderintent.PlaceIntent{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}

	message := err.Error()
	if !strings.Contains(message, "balance or orderable amount is insufficient") {
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
	}, &orderintent.PlaceIntent{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        500,
		CurrencyMode: "KRW",
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}

	message := err.Error()
	if !strings.Contains(message, "FX exchange or foreign-currency usage consent is required") {
		t.Fatalf("expected fx guidance, got %q", message)
	}
	if !strings.Contains(message, "FX exchange / foreign-currency consent screen for this US stock order") {
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
	}, &orderintent.PlaceIntent{
		Symbol:       "TSLL",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     1,
		Price:        1000,
		CurrencyMode: "KRW",
	})
	if err == nil {
		t.Fatal("expected formatted error")
	}

	message := err.Error()
	if !strings.Contains(message, "Order preparation passed") {
		t.Fatalf("expected post-prepare intro, got %q", message)
	}
	if !strings.Contains(message, "Short by $0.68.") {
		t.Fatalf("expected need-exchange line, got %q", message)
	}
	if !strings.Contains(message, "Estimated exchange amount: 1,020 KRW") {
		t.Fatalf("expected estimated exchange amount, got %q", message)
	}
	if !strings.Contains(message, "Estimated exchange rate: 1,500.21 KRW/USD") {
		t.Fatalf("expected exchange rate detail, got %q", message)
	}
	if !strings.Contains(message, "remain in the account as USD") {
		t.Fatalf("expected retained-USD warning, got %q", message)
	}
	if !strings.Contains(message, "accept_fx_consent=true") {
		t.Fatalf("expected config guidance for automation, got %q", message)
	}
}

func rootPathsForTest() config.Paths {
	return config.Paths{}
}

func TestUserFacingCommandErrorAuthErrorMentionsExtend(t *testing.T) {
	t.Parallel()

	authErr := &tossclient.AuthError{StatusCode: 401, Endpoint: "/api/v1/account/list"}
	got := userFacingCommandError(authErr).Error()
	if !strings.Contains(got, "tossctl auth extend") {
		t.Fatalf("expected auth extend hint, got %q", got)
	}
	if !strings.Contains(got, "tossctl auth login") {
		t.Fatalf("expected auth login hint, got %q", got)
	}
}

func TestUserFacingCommandErrorExtensionTimeout(t *testing.T) {
	t.Parallel()

	wrapped := &auth.ExtensionTimeoutError{Elapsed: 120 * time.Second}
	got := userFacingCommandError(wrapped).Error()
	if !strings.Contains(got, "waited 2m0s") && !strings.Contains(got, "waited 120s") {
		t.Fatalf("expected timeout elapsed detail, got %q", got)
	}
	if !strings.Contains(got, "rerun") {
		t.Fatalf("expected retry hint, got %q", got)
	}
	if !strings.Contains(got, "tossctl auth extend") {
		t.Fatalf("expected extend command hint, got %q", got)
	}
}

func TestUserFacingCommandErrorContextCanceled(t *testing.T) {
	t.Parallel()

	got := userFacingCommandError(context.Canceled).Error()
	if !strings.Contains(got, "canceled") {
		t.Fatalf("expected cancellation message, got %q", got)
	}
}

func TestUserFacingCommandErrorExtensionRejected(t *testing.T) {
	t.Parallel()

	got := userFacingCommandError(auth.ErrExtensionRejected).Error()
	if !strings.Contains(got, "denied") && !strings.Contains(got, "canceled") {
		t.Fatalf("expected rejection wording, got %q", got)
	}
}

func TestUserFacingCommandErrorSessionErrNoSessionMentionsLogin(t *testing.T) {
	t.Parallel()

	for _, err := range []error{session.ErrNoSession, tossclient.ErrNoSession} {
		got := userFacingCommandError(err).Error()
		if !strings.Contains(got, "tossctl auth login") {
			t.Fatalf("for %v: expected login hint, got %q", err, got)
		}
		if !strings.Contains(got, "no active session") {
			t.Fatalf("for %v: expected friendly message, got %q", err, got)
		}
	}
}

func TestBuildPlaceCommandReflectsNormalizedIntent(t *testing.T) {
	t.Parallel()

	// KR limit sell — auto-detected market must show --market kr, never the
	// flag default --market us (the bug found during live testing).
	krCmd := buildPlaceCommand("place", &orderintent.PlaceIntent{
		Symbol: "114800", Market: "kr", Side: "sell", OrderType: "limit",
		Quantity: 1, Price: 1100, CurrencyMode: "KRW",
	}, "TOKEN")
	if !strings.Contains(krCmd, "--market kr") {
		t.Fatalf("expected --market kr, got %q", krCmd)
	}
	if strings.Contains(krCmd, "--market us") {
		t.Fatalf("must not echo --market us for a KR order: %q", krCmd)
	}

	// Fractional buy is amount-based — must use --amount, never --qty/--price.
	fracCmd := buildPlaceCommand("place", &orderintent.PlaceIntent{
		Symbol: "TSLA", Market: "us", Side: "buy", OrderType: "market",
		Amount: 10, Fractional: true, CurrencyMode: "KRW",
	}, "T")
	if !strings.Contains(fracCmd, "--amount 10") {
		t.Fatalf("fractional buy must use --amount, got %q", fracCmd)
	}
	if strings.Contains(fracCmd, "--qty") || strings.Contains(fracCmd, "--price") {
		t.Fatalf("fractional buy must not use --qty/--price, got %q", fracCmd)
	}
}
