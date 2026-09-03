package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWritePortfolioFoldersPreservesGroupingAndFeeAwareValues(t *testing.T) {
	t.Parallel()
	view := domain.PortfolioFolders{
		PortfolioFolderSummary: domain.PortfolioFolderSummary{
			PrincipalAmount: domain.AssetAmount{KRW: 1000, USD: 1}, EvaluatedAmount: domain.AssetAmount{KRW: 1200, USD: 1.2},
			EvaluatedAmountAfterFees: domain.AssetAmount{KRW: 1190, USD: 1.19}, ProfitLossAmount: domain.AssetAmount{KRW: 200, USD: 0.2},
			ProfitLossAfterFees: domain.AssetAmount{KRW: 190, USD: 0.19}, ProfitLossRate: domain.AssetRate{KRW: 0.2, USD: 0.2},
			ProfitLossRateAfterFees: domain.AssetRate{KRW: 0.19, USD: 0.19},
		},
		SectionType: "FOLDER_OVERVIEW_V2", AccountScope: "opaque-scope", Hidden: domain.PortfolioHiddenSummary{Count: 2, Amount: 300},
		Folders: []domain.PortfolioFolder{{
			PortfolioFolderSummary: domain.PortfolioFolderSummary{
				EvaluatedAmount: domain.AssetAmount{KRW: 1200, USD: 1.2}, EvaluatedAmountAfterFees: domain.AssetAmount{KRW: 1190, USD: 1.19},
				ProfitLossAmount: domain.AssetAmount{KRW: 200, USD: 0.2}, ProfitLossAfterFees: domain.AssetAmount{KRW: 190, USD: 0.19},
				ProfitLossRate: domain.AssetRate{KRW: 0.2, USD: 0.2}, ProfitLossRateAfterFees: domain.AssetRate{KRW: 0.19, USD: 0.19},
			},
			Key: "private-folder-key", Name: "Long term", Type: "DEFAULT", Default: true,
			Items: []domain.PortfolioFolderItem{{
				Key: "private-item-key", ProductCode: "US.TEST", Symbol: "TEST", Name: "Test Corp", Quantity: 2, TradableQuantity: 1,
				EvaluatedAmount: domain.AssetAmount{KRW: 1200, USD: 1.2}, EvaluatedAmountAfterFees: domain.AssetAmount{KRW: 1190, USD: 1.19},
				ProfitLossAmount: domain.AssetAmount{KRW: 200, USD: 0.2}, ProfitLossAfterFees: domain.AssetAmount{KRW: 190, USD: 0.19},
				ProfitLossRate: domain.AssetRate{KRW: 0.2, USD: 0.2}, ProfitLossRateAfterFees: domain.AssetRate{KRW: 0.19, USD: 0.19},
				Commission: domain.AssetAmount{KRW: 10, USD: 0.01}, MarketDivision: "us",
			}},
		}},
	}

	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WritePortfolioFolders(&out, format, view); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		text := out.String()
		value1190 := "1190"
		if format == FormatTable {
			value1190 = "1,190"
		}
		for _, want := range []string{"Long term", "TEST", value1190, "190"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s missing %q: %s", format, want, text)
			}
		}
		if strings.Contains(text, "private-folder-key") || strings.Contains(text, "private-item-key") || strings.Contains(text, "selected-account") {
			t.Errorf("%s leaked internal identifiers: %s", format, text)
		}
	}
}
