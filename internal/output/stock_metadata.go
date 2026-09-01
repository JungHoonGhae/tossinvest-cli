package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteStockMetadata renders the official API's exact batch metadata contract.
func WriteStockMetadata(w io.Writer, format Format, stocks []domain.StockMetadata) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, stocks)
	case FormatCSV:
		rows := make([][]string, 0, len(stocks))
		for _, stock := range stocks {
			liquidationTrading, nxtSupported, krxSuspended, nxtSuspended := "", "", "", ""
			if detail := stock.KoreanMarketDetail; detail != nil {
				liquidationTrading = strconv.FormatBool(detail.LiquidationTrading)
				nxtSupported = strconv.FormatBool(detail.NXTSupported)
				krxSuspended = strconv.FormatBool(detail.KRXTradingSuspended)
				nxtSuspended = optionalBool(detail.NXTTradingSuspended)
			}
			rows = append(rows, []string{
				stock.Symbol, stock.Name, stock.EnglishName, stock.ISINCode, stock.MarketCode,
				stock.SecurityType, strconv.FormatBool(stock.CommonShare), stock.Status,
				stock.Currency, stock.SharesOutstanding, optionalString(stock.ListDate),
				optionalString(stock.DelistDate), optionalString(stock.LeverageFactor),
				liquidationTrading, nxtSupported, krxSuspended, nxtSuspended,
				stock.FetchedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
		return writeCSV(w, []string{
			"symbol", "name", "english_name", "isin_code", "market_code",
			"security_type", "common_share", "status", "currency", "shares_outstanding",
			"list_date", "delist_date", "leverage_factor", "liquidation_trading",
			"nxt_supported", "krx_trading_suspended", "nxt_trading_suspended", "fetched_at",
		}, rows)
	case FormatTable:
		if len(stocks) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.stockMetadata.empty"))
			return err
		}
		headers := []string{
			i18n.T("output.stockMetadata.header.symbol"),
			i18n.T("output.stockMetadata.header.name"),
			i18n.T("output.stockMetadata.header.market"),
			i18n.T("output.stockMetadata.header.type"),
			i18n.T("output.stockMetadata.header.status"),
			i18n.T("output.stockMetadata.header.shares"),
		}
		rows := make([][]string, 0, len(stocks))
		for _, stock := range stocks {
			rows = append(rows, []string{
				stock.Symbol, stock.Name, stock.MarketCode, stock.SecurityType,
				stock.Status, stock.SharesOutstanding,
			})
		}
		return renderTable(w, headers, rows,
			AlignLeft, AlignLeft, AlignLeft, AlignLeft, AlignLeft, AlignRight)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalBool(value *bool) string {
	if value == nil {
		return ""
	}
	return strconv.FormatBool(*value)
}
