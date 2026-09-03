package output

import (
	"bytes"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteLendingRevenueRankingCSV(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	ranking := domain.LendingRevenueRanking{Items: []domain.LendingRevenueRank{{
		Rank: 1, UserName: "user-a", Revenue: 12.34, RevenueKRW: 16900,
	}}}
	if err := WriteLendingRevenueRanking(&out, FormatCSV, ranking); err != nil {
		t.Fatal(err)
	}
	const want = "rank,user_name,revenue,revenue_krw\n1,user-a,12.34,16900\n"
	if out.String() != want {
		t.Fatalf("CSV = %q, want %q", out.String(), want)
	}
}
