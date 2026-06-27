package output

import (
	"bytes"
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
