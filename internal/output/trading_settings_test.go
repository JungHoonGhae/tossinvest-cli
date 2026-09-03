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
		SimpleTradeEnabled:     false,
		InvestorExchangeChoice: "integrated",
		ATSNotificationEnabled: true,
		OptionRealTimeTick: domain.OptionRealTimeTickStatus{
			Requested: true, Serviced: false, ShouldCharged: true,
		},
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteTradingSettings(&out, format, settings); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		for _, want := range []string{"integrated", "simple", "ats", "requested", "serviced", "should"} {
			if !strings.Contains(strings.ToLower(out.String()), want) {
				t.Fatalf("%s missing %q: %s", format, want, out.String())
			}
		}
	}
}
