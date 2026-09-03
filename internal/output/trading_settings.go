package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func WriteTradingSettings(w io.Writer, format Format, settings domain.TradingSettings) error {
	simpleTrade := strconv.FormatBool(settings.SimpleTradeEnabled)
	atsNotification := strconv.FormatBool(settings.ATSNotificationEnabled)
	requested := strconv.FormatBool(settings.OptionRealTimeTick.Requested)
	serviced := strconv.FormatBool(settings.OptionRealTimeTick.Serviced)
	rawShouldCharged := strconv.FormatBool(settings.OptionRealTimeTick.RawShouldCharged)

	switch format {
	case FormatJSON:
		return writeJSON(w, settings)
	case FormatCSV:
		return writeCSV(w,
			[]string{"account_scope", "simple_trade_enabled", "investor_exchange_choice", "ats_notification_enabled", "option_tick_requested", "option_tick_serviced", "option_tick_raw_should_charged"},
			[][]string{{settings.AccountScope, simpleTrade, settings.InvestorExchangeChoice, atsNotification, requested, serviced, rawShouldCharged}},
		)
	case FormatTable:
		return renderTable(w, []string{"SETTING", "VALUE"}, [][]string{
			{"account scope", settings.AccountScope},
			{"simple trade enabled", simpleTrade},
			{"investor exchange choice", settings.InvestorExchangeChoice},
			{"ATS notification enabled", atsNotification},
			{"option tick requested", requested},
			{"option tick serviced", serviced},
			{"option tick raw shouldCharged", rawShouldCharged},
		})
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
