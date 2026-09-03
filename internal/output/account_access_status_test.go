package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteAccountAccessStatusPreservesVerifiedSignals(t *testing.T) {
	t.Parallel()
	status := domain.AccountAccessStatus{
		AccountScope: "opaque-scope",
		LastLogin: domain.AccountLastLogin{
			Channel: "W", OSName: "macOS", AgentName: "Chrome", Timestamp: "2026-09-03T12:34:56+09:00",
		},
		Margin: domain.AccountMarginRestriction{
			Frozen: true, StartDate: "2026-09-01", EndDate: "2026-09-30",
		},
		AccidentAccountCount: 2,
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteAccountAccessStatus(&out, format, status); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		for _, want := range []string{"opaque-scope", "macOS", "Chrome", "2026-09-03", "true", "2"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%s missing %q: %s", format, want, out.String())
			}
		}
	}
}
