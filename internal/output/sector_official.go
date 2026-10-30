package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// Official sector taxonomy writers. JSON keeps the raw decimal ratio and nil
// for "no data"; table/CSV show the ratio as a percent and leave nil blank so
// an absent figure is never read as zero.

func ratioPct(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v*100, 'f', 2, 64) + "%"
}

func optFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return formatFloat(*v)
}

// WriteSectorCatalog renders the whole taxonomy, indented by depth in table form.
func WriteSectorCatalog(w io.Writer, format Format, c domain.SectorCatalog) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, c)
	case FormatCSV:
		rows := make([][]string, 0, len(c.Sectors))
		for _, s := range c.Sectors {
			rows = append(rows, []string{s.ID, s.Name, strconv.Itoa(s.Depth), s.ParentID})
		}
		return writeCSV(w, []string{"id", "name", "depth", "parent_id"}, rows)
	case FormatTable:
		if len(c.Sectors) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.sector.empty"))
			return err
		}
		headers := []string{i18n.T("output.sector.header.id"), i18n.T("output.sector.header.name"), i18n.T("output.sector.header.depth"), i18n.T("output.sector.header.parent")}
		rows := make([][]string, 0, len(c.Sectors))
		for _, s := range c.Sectors {
			rows = append(rows, []string{s.ID, s.Name, strconv.Itoa(s.Depth), s.ParentID})
		}
		return renderTable(w, headers, rows, AlignLeft, AlignLeft, AlignRight, AlignLeft)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSectorProfile renders one sector with its six-period returns.
func WriteSectorProfile(w io.Writer, format Format, p domain.SectorProfile) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, p)
	case FormatCSV:
		rows := make([][]string, 0, len(p.Returns))
		for _, r := range p.Returns {
			rows = append(rows, []string{p.ID, p.Name, p.Market, r.Duration, optFloat(r.ChangeRate)})
		}
		return writeCSV(w, []string{"id", "name", "market", "duration", "change_rate"}, rows)
	case FormatTable:
		fmt.Fprintf(w, "%s  %s  depth=%d  %s\n", p.ID, p.Name, p.Depth, p.Market)
		if p.Summary != "" {
			fmt.Fprintln(w, p.Summary)
		}
		headers := []string{i18n.T("output.sector.header.duration"), i18n.T("output.sector.header.change")}
		rows := make([][]string, 0, len(p.Returns))
		for _, r := range p.Returns {
			rows = append(rows, []string{r.Duration, ratioPct(r.ChangeRate)})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSectorRankings renders a ranking; blank cells mean the server had no figure.
func WriteSectorRankings(w io.Writer, format Format, r domain.SectorRankings) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, r)
	case FormatCSV:
		rows := make([][]string, 0, len(r.Items))
		for _, it := range r.Items {
			rows = append(rows, []string{strconv.Itoa(it.Rank), it.ID, it.Name, optFloat(it.ChangeRate), optFloat(it.TradingAmount), optFloat(it.MarketCap), it.Currency})
		}
		return writeCSV(w, []string{"rank", "id", "name", "change_rate", "trading_amount", "market_cap", "currency"}, rows)
	case FormatTable:
		if len(r.Items) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.sector.empty"))
			return err
		}
		fmt.Fprintf(w, "%s  %s  %s\n", r.Type, r.Market, r.Duration)
		headers := []string{"#", i18n.T("output.sector.header.id"), i18n.T("output.sector.header.name"), i18n.T("output.sector.header.change"), i18n.T("output.sector.header.tradingAmount"), i18n.T("output.sector.header.marketCap")}
		rows := make([][]string, 0, len(r.Items))
		for _, it := range r.Items {
			rows = append(rows, []string{strconv.Itoa(it.Rank), it.ID, it.Name, ratioPct(it.ChangeRate), optFloat(it.TradingAmount), optFloat(it.MarketCap)})
		}
		return renderTable(w, headers, rows, AlignRight, AlignLeft, AlignLeft, AlignRight, AlignRight, AlignRight)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSectorConstituents renders a sector's stocks (server order: market cap desc).
func WriteSectorConstituents(w io.Writer, format Format, s domain.SectorConstituents) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, s)
	case FormatCSV:
		rows := make([][]string, 0, len(s.Stocks))
		for _, st := range s.Stocks {
			rows = append(rows, []string{st.Symbol, st.Name, optFloat(st.MarketCap), optFloat(st.TradingAmount), optFloat(st.Volume), st.Currency})
		}
		return writeCSV(w, []string{"symbol", "name", "market_cap", "trading_amount", "volume", "currency"}, rows)
	case FormatTable:
		if len(s.Stocks) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.sector.empty"))
			return err
		}
		headers := []string{i18n.T("output.sector.header.symbol"), i18n.T("output.sector.header.name"), i18n.T("output.sector.header.marketCap"), i18n.T("output.sector.header.tradingAmount"), i18n.T("output.sector.header.volume")}
		rows := make([][]string, 0, len(s.Stocks))
		for _, st := range s.Stocks {
			rows = append(rows, []string{st.Symbol, st.Name, optFloat(st.MarketCap), optFloat(st.TradingAmount), optFloat(st.Volume)})
		}
		return renderTable(w, headers, rows, AlignLeft, AlignLeft, AlignRight, AlignRight, AlignRight)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteSectorETFs renders a sector's ETFs.
func WriteSectorETFs(w io.Writer, format Format, e domain.SectorETFs) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, e)
	case FormatCSV:
		rows := make([][]string, 0, len(e.ETFs))
		for _, et := range e.ETFs {
			rows = append(rows, []string{et.Symbol, et.Name})
		}
		return writeCSV(w, []string{"symbol", "name"}, rows)
	case FormatTable:
		if len(e.ETFs) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.sector.empty"))
			return err
		}
		rows := make([][]string, 0, len(e.ETFs))
		for _, et := range e.ETFs {
			rows = append(rows, []string{et.Symbol, et.Name})
		}
		return renderTable(w, []string{i18n.T("output.sector.header.symbol"), i18n.T("output.sector.header.name")}, rows, AlignLeft, AlignLeft)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
