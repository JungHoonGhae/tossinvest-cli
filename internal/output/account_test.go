package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

var testAccountSummary = domain.AccountSummary{
	TotalAssetAmount:      10000000,
	EvaluatedProfitAmount: 500000,
	ProfitRate:            0.05,
	OrderableAmountKRW:    1000000,
	OrderableAmountUSD:    500,
}

// Regression guard: buffer (non-TTY) must produce no ANSI codes.
func TestAccountSummaryPlainWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccountSummary(&buf, FormatTable, testAccountSummary); err != nil {
		t.Fatalf("WriteAccountSummary error: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("non-TTY WriteAccountSummary table output must contain no ANSI escape sequences")
	}
}

func TestWriteAccountPrimeMember(t *testing.T) {
	userID := "u-1"
	primeType := "STANDARD"
	start := "2026-07-01"
	end := "2026-07-31"
	cycle := 3
	p := domain.PrimeStatus{
		IsMember:        true,
		UserID:          &userID,
		PrimeType:       &primeType,
		BenefitsStartAt: &start,
		BenefitsEndAt:   &end,
		CycleNumber:     &cycle,
		Month:           "2026-07",
		Exchange:        domain.PrimeExchangeFee{NonPrimeFee: 25, PrimeFee: 15, BenefitFee: 10},
		InterestKRW:     domain.PrimeInterestTier{Status: "ACTIVE", NonPrimeInterest: 1, PrimeInterest: 2, BenefitInterest: 3},
		InterestUSD:     domain.PrimeInterestTier{Status: "ACTIVE", NonPrimeInterest: 1, PrimeInterest: 2, BenefitInterest: 3},
		BaseRate:        3.5,
		MonthlyTotalKRW: 12345.6,
	}

	var buf bytes.Buffer
	if err := WriteAccountPrime(&buf, FormatTable, p); err != nil {
		t.Fatalf("WriteAccountPrime(table) error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "STANDARD") {
		t.Errorf("table output missing PrimeType, got: %s", out)
	}

	buf.Reset()
	if err := WriteAccountPrime(&buf, FormatJSON, p); err != nil {
		t.Fatalf("WriteAccountPrime(json) error = %v", err)
	}
	var decoded domain.PrimeStatus
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json decode error = %v", err)
	}
	if decoded.MonthlyTotalKRW != 12345.6 {
		t.Errorf("decoded MonthlyTotalKRW = %v, want 12345.6", decoded.MonthlyTotalKRW)
	}

	buf.Reset()
	if err := WriteAccountPrime(&buf, FormatCSV, p); err != nil {
		t.Fatalf("WriteAccountPrime(csv) error = %v", err)
	}
	if !strings.Contains(buf.String(), "25") || !strings.Contains(buf.String(), "15") || !strings.Contains(buf.String(), "10") {
		t.Errorf("csv output missing expected fee values, got: %s", buf.String())
	}
}

func TestWriteAccountPrimeNonMember(t *testing.T) {
	p := domain.PrimeStatus{
		IsMember:        false,
		Month:           "2026-07",
		Exchange:        domain.PrimeExchangeFee{NonPrimeFee: 25, PrimeFee: 15, BenefitFee: 25},
		InterestKRW:     domain.PrimeInterestTier{Status: "ACTIVE", NonPrimeInterest: 1, PrimeInterest: 2, BenefitInterest: 1},
		InterestUSD:     domain.PrimeInterestTier{Status: "ACTIVE", NonPrimeInterest: 1, PrimeInterest: 2, BenefitInterest: 1},
		BaseRate:        3.5,
		MonthlyTotalKRW: 0,
	}
	var buf bytes.Buffer
	if err := WriteAccountPrime(&buf, FormatTable, p); err != nil {
		t.Fatalf("WriteAccountPrime(table) error = %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<nil>") {
		t.Errorf("table output leaked a nil-pointer format artifact, got: %s", out)
	}
}
