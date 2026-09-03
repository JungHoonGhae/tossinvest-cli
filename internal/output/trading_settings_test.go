package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteTradingSettingsExposesEveryVerifiedSetting(t *testing.T) {
	t.Parallel()
	settings := domain.TradingSettings{
		AccountScope:           "scope-123",
		SimpleTradeEnabled:     false,
		InvestorExchangeChoice: "integrated",
		ATSNotificationEnabled: true,
		OptionRealTimeTick: domain.OptionRealTimeTickStatus{
			Requested: true, Serviced: false, RawShouldCharged: true,
		},
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteTradingSettings(&out, format, settings); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		for _, want := range []string{"scope-123", "integrated", "simple", "ats", "requested", "serviced", "raw"} {
			if !strings.Contains(strings.ToLower(out.String()), want) {
				t.Fatalf("%s missing %q: %s", format, want, out.String())
			}
		}
	}
}
