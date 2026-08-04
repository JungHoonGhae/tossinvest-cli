package output

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteScreenerFilterRanges renders each filter's usable value span.
func WriteScreenerFilterRanges(w io.Writer, format Format, r domain.ScreenerFilterRanges) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, r)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"filter_id", "nation", "min", "max", "based_at", "unavailable_reason"}); err != nil {
			return err
		}
		for _, f := range r.Filters {
			min, max := "", ""
			if f.Min != nil {
				min = formatFloat(*f.Min)
			}
			if f.Max != nil {
				max = formatFloat(*f.Max)
			}
			if err := writer.Write([]string{f.FilterID, f.Nation, min, max, f.BasedAt, f.Unavailable}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if len(r.Filters) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.screenerFilters.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.screenerFilters.header"), r.Nation); err != nil {
			return err
		}
		for _, f := range r.Filters {
			// 조건이 더 필요해 서버가 거절한 필터는 범위 대신 사유를 낸다. 빈 범위를
			// 0~0 으로 찍으면 "값이 없는 필터" 로 오독된다.
			if f.Min == nil || f.Max == nil {
				if _, err := fmt.Fprintf(w, i18n.T("output.screenerFilters.unavailable"),
					f.FilterID, f.Unavailable); err != nil {
					return err
				}
				continue
			}
			// 표에서는 유효숫자를 줄인다: 서버가 float32 정밀도를 그대로 흘려
			// -8021.2705078125 처럼 나오는데, 범위를 눈으로 가늠하는 데 쓰는
			// 숫자라 자릿수가 길수록 읽히지 않는다. JSON/CSV 는 원값을 유지한다.
			line, basedAt := i18n.T("output.screenerFilters.line"), f.BasedAt
			if basedAt == "" {
				// 기준일을 못 받았으면 꼬리에 "기준 " 만 남는다 — 값이 있는데
				// 못 읽은 것처럼 보인다.
				line = i18n.T("output.screenerFilters.lineNoDate")
				if _, err := fmt.Fprintf(w, line, f.FilterID, compactNum(*f.Min), compactNum(*f.Max)); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(w, line,
				f.FilterID, compactNum(*f.Min), compactNum(*f.Max), basedAt); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// compactNum trims a float to something a person can compare at a glance.
// %g with 6 significant digits keeps both 0.382436 and 1.50793e+15 readable,
// where the raw float32 spill (-8021.2705078125) is not.
func compactNum(v float64) string {
	return fmt.Sprintf("%.6g", v)
}
