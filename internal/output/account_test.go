package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
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

// 더미 값 — 실계좌 데이터 아님.
var testCommissionSchedule = domain.CommissionSchedule{
	Korea: domain.CommissionTier{RatePercent: 0.011},
	US: domain.CommissionTier{
		RatePercent:    0.2,
		HasReduction:   true,
		ReductionEndAt: "2026-12-31",
	},
	USOptions: &domain.CommissionTier{PerContract: 2.49},
}

func TestWriteAccountCommissionTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccountCommission(&buf, FormatTable, testCommissionSchedule); err != nil {
		t.Fatalf("WriteAccountCommission(table) error = %v", err)
	}
	out := buf.String()
	// 요율은 이미 퍼센트 단위다. 어디선가 ×100 이 끼면 "1.1%" 로 새고 여기서 잡힌다.
	if !strings.Contains(out, "0.011%") {
		t.Errorf("table missing KR rate 0.011%%:\n%s", out)
	}
	// 옵션은 요율이 아니라 계약당 정액이다.
	if !strings.Contains(out, "$2.49") {
		t.Errorf("table missing US options per-contract fee:\n%s", out)
	}
	// 우대는 적용된 행에만 종료일이 붙는다.
	if !strings.Contains(out, "2026-12-31") {
		t.Errorf("table missing reduction end date:\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line != strings.TrimRight(line, " ") {
			t.Errorf("line has trailing whitespace: %q", line)
		}
	}
}

// 옵션 약정이 없는 계좌는 티어가 nil 이다. 0원 수수료 행으로 렌더하면 안 된다.
func TestWriteAccountCommissionTableNoOptions(t *testing.T) {
	schedule := testCommissionSchedule
	schedule.USOptions = nil

	var buf bytes.Buffer
	if err := WriteAccountCommission(&buf, FormatTable, schedule); err != nil {
		t.Fatalf("WriteAccountCommission(table) error = %v", err)
	}
	if got := strings.Count(buf.String(), "\n"); got != 3 {
		t.Errorf("expected header + 2 rows, got %d lines:\n%s", got, buf.String())
	}
}

// 더미 값 — 실계좌 데이터 아님.
var testAccountInterest = domain.AccountInterest{
	Year:  2025,
	Total: 1300,
	Monthly: []domain.InterestMonth{
		{Month: 1, Total: 0},
		{Month: 2, Total: 1300, Payments: []domain.InterestPayment{
			{Date: "2025-02-11", Amount: 1500, Tax: 200, PaymentAmount: 1300,
				StartDate: "2024-11-01", EndDate: "2025-01-31"},
			{Date: "2025-02-28", Amount: 400, PaymentAmount: 400,
				StartDate: "2025-02-01", EndDate: "2025-02-28", Estimated: true},
		}},
	},
}

func TestWriteAccountInterestTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccountInterest(&buf, FormatTable, testAccountInterest); err != nil {
		t.Fatalf("WriteAccountInterest(table) error = %v", err)
	}
	out := buf.String()
	// 예상 이자는 확정 수령액과 구분돼야 한다.
	if !strings.Contains(out, "2025-02-28") || !strings.Contains(out, i18n.T("output.accountInterest.estimatedMark")) {
		t.Errorf("estimated payment not marked:\n%s", out)
	}
	// 세전액·세금이 사라지면 실지급액만 남아 세금이 안 보인다.
	for _, want := range []string{"1500", "200", "1300"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	// 산정기간은 지급월과 다르므로 반드시 보여야 한다.
	if !strings.Contains(out, "2024-11-01") {
		t.Errorf("table missing accrual start date:\n%s", out)
	}
	// 지급 없는 달(1월)은 빠진다 — 서버는 12개월을 다 준다.
	if strings.Contains(out, "  1  ") {
		t.Errorf("empty month should be omitted:\n%s", out)
	}
}

// 빈 해에는 조회 가능한 연도를 알려줘야 사용자가 다음에 뭘 넣을지 안다.
func TestWriteAccountInterestEmptyYearListsAvailable(t *testing.T) {
	var buf bytes.Buffer
	empty := domain.AccountInterest{
		Year:           2020,
		Monthly:        []domain.InterestMonth{{Month: 1}, {Month: 2}},
		AvailableYears: []int{2024, 2025},
	}
	if err := WriteAccountInterest(&buf, FormatTable, empty); err != nil {
		t.Fatalf("WriteAccountInterest(table) error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "2024, 2025") {
		t.Errorf("empty year should list available years:\n%s", out)
	}
}
