package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteSectorDetailCSVUsesOneStableSchema(t *testing.T) {
	t.Parallel()
	closePrice := 101.5
	detail := domain.SectorDetail{
		StockTotalCount: 4,
		ETFTotalCount:   3,
		NewsTotalCount:  2,
		Stocks:          []domain.SectorStock{{Rank: 1, ProductCode: "A000001", Name: "예시 종목", ChangeRate: 1.5, Price: domain.SectorPrice{Close: &closePrice}}},
		ETFs:            []domain.SectorETF{{Rank: 2, ProductCode: "ETF001", Symbol: "EXM", Name: "예시 ETF", ChangeRate: -0.5}},
		News:            []domain.SectorNews{{ID: "news-1", Title: "산업 뉴스", Source: "source", CreatedAt: "2026-09-03T00:00:00Z"}},
	}
	var out bytes.Buffer
	if err := WriteSectorDetail(&out, FormatCSV, detail); err != nil {
		t.Fatal(err)
	}
	want := "type,rank,code,name,change_rate,close,source,created_at,total_count\n" +
		"stock,1,A000001,예시 종목,1.5,101.5,,,4\n" +
		"etf,2,EXM,예시 ETF,-0.5,,,,3\n" +
		"news,,news-1,산업 뉴스,,,source,2026-09-03T00:00:00Z,2\n"
	if got := out.String(); got != want {
		t.Fatalf("CSV mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestWriteSectorDetailTableShowsReturnedAndTotalCounts(t *testing.T) {
	t.Parallel()
	detail := domain.SectorDetail{
		Name: "예시 업종", Summary: "요약", CompanyCount: 20, ETFCount: 12,
		StockTotalCount: 20, ETFTotalCount: 12, NewsTotalCount: 8,
		Stocks: []domain.SectorStock{{Name: "한 종목"}},
		ETFs:   []domain.SectorETF{{Name: "한 ETF"}},
		News:   []domain.SectorNews{{Title: "한 뉴스"}},
	}
	var out bytes.Buffer
	if err := WriteSectorDetail(&out, FormatTable, detail); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"예시 업종", "1/20", "1/12", "1/8", "요약"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("table missing %q:\n%s", want, out.String())
		}
	}
}
