package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func performanceFixture() domain.AssetPerformance {
	return domain.AssetPerformance{
		Scope:               "all_accounts",
		Range:               "ONE_MONTH",
		StepUnit:            "DAY",
		EvaluatedAmountDiff: domain.AssetAmount{KRW: 1200, USD: 1.25},
		MaxEvaluated:        domain.AssetSnapshotExtreme{BaseDate: "2026-09-03", Amount: domain.AssetAmount{KRW: 11000, USD: 11}},
		MinEvaluated:        domain.AssetSnapshotExtreme{BaseDate: "2026-09-01", Amount: domain.AssetAmount{KRW: 9000, USD: 9}},
		Points: []domain.AssetSnapshotPoint{{
			BaseDate:        "2026-09-03",
			PrincipalAmount: domain.AssetAmount{KRW: 8000, USD: 8},
			EvaluatedAmount: domain.AssetAmount{KRW: 11000, USD: 11},
			ProfitLossRate:  domain.AssetAmount{KRW: 0.375, USD: 0.375},
			Realtime:        true,
		}},
	}
}

func TestWriteAssetPerformanceJSONPreservesMachineContract(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteAssetPerformance(&out, FormatJSON, performanceFixture()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["scope"] != "all_accounts" || got["range"] != "ONE_MONTH" || got["step_unit"] != "DAY" {
		t.Fatalf("metadata = %#v", got)
	}
	points, ok := got["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("points = %#v", got["points"])
	}
}

func TestWriteAssetPerformanceCSVUsesStablePointSchema(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteAssetPerformance(&out, FormatCSV, performanceFixture()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"date,principal_krw,evaluated_krw,profit_loss_rate_krw,principal_usd,evaluated_usd,profit_loss_rate_usd,realtime",
		"2026-09-03,8000,11000,0.375,8,11,0.375,true",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("CSV missing %q:\n%s", want, out.String())
		}
	}
}

func TestWriteAssetPerformanceTableIncludesSummaryAndPoints(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteAssetPerformance(&out, FormatTable, performanceFixture()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"all_accounts", "ONE_MONTH/DAY", "2026-09-03", "11,000", "37.50%", "1,200"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func snapshotPageFixture() domain.AssetSnapshotPage {
	return domain.AssetSnapshotPage{
		Scope:      "all_accounts",
		PageSize:   20,
		NextCursor: "cursor-2",
		HasNext:    true,
		Snapshots: []domain.AssetSnapshotPoint{{
			BaseDate:           "2026-09-02",
			PrincipalAmount:    domain.AssetAmount{KRW: 8000, USD: 8},
			EvaluatedAmount:    domain.AssetAmount{KRW: 10000, USD: 10},
			ProfitLossAmount:   domain.AssetAmount{KRW: 2000, USD: 2},
			ProfitLossRate:     domain.AssetAmount{KRW: 0.25, USD: 0.25},
			EvaluationComplete: true,
		}},
	}
}

func TestWriteAssetSnapshotsTableIncludesCursorAndValuation(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteAssetSnapshots(&out, FormatTable, snapshotPageFixture()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"all_accounts", "2026-09-02", "10,000", "2,000", "25.00%", "cursor-2"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestWriteAssetSnapshotsMachineFormatsPreservePageAndRows(t *testing.T) {
	t.Parallel()
	var jsonOut bytes.Buffer
	if err := WriteAssetSnapshots(&jsonOut, FormatJSON, snapshotPageFixture()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["next_cursor"] != "cursor-2" || got["has_next"] != true {
		t.Fatalf("JSON page metadata = %#v", got)
	}

	var csvOut bytes.Buffer
	if err := WriteAssetSnapshots(&csvOut, FormatCSV, snapshotPageFixture()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"date,principal_krw,evaluated_krw,profit_loss_krw,profit_loss_rate_krw,principal_usd,evaluated_usd,profit_loss_usd,profit_loss_rate_usd,realtime,evaluation_complete",
		"2026-09-02,8000,10000,2000,0.25,8,10,2,0.25,false,true",
	} {
		if !strings.Contains(csvOut.String(), want) {
			t.Errorf("CSV missing %q:\n%s", want, csvOut.String())
		}
	}
}

func snapshotDetailFixture() domain.AssetSnapshotDetail {
	return domain.AssetSnapshotDetail{
		Scope:              "all_accounts",
		BaseDate:           "2026-09-03",
		EvaluationComplete: true,
		PrincipalAmount:    domain.AssetAmount{KRW: 8000, USD: 8},
		EvaluatedAmount:    domain.AssetAmount{KRW: 11000, USD: 11},
		ProfitLossAmount:   domain.AssetAmount{KRW: 3000, USD: 3},
		ProfitLossRate:     domain.AssetAmount{KRW: 0.375, USD: 0.375},
		Markets: []domain.AssetSnapshotMarket{
			{Market: "kr", Holdings: []domain.AssetSnapshotHolding{}},
			{Market: "option", Holdings: []domain.AssetSnapshotHolding{}},
			{
				Market:           "us",
				PrincipalAmount:  domain.AssetAmount{KRW: 8000, USD: 8},
				EvaluatedAmount:  domain.AssetAmount{KRW: 11000, USD: 11},
				ProfitLossAmount: domain.AssetAmount{KRW: 3000, USD: 3},
				ProfitLossRate:   domain.AssetAmount{KRW: 0.375, USD: 0.375},
				Holdings: []domain.AssetSnapshotHolding{{
					ProductCode:      "US.AAPL",
					ISIN:             "US0378331005",
					Symbol:           "AAPL",
					Name:             "Apple",
					Quantity:         2,
					PurchasePrice:    domain.AssetAmount{KRW: 4000, USD: 4},
					PurchaseAmount:   domain.AssetAmount{KRW: 8000, USD: 8},
					EvaluatedAmount:  domain.AssetAmount{KRW: 11000, USD: 11},
					ProfitLossAmount: domain.AssetAmount{KRW: 3000, USD: 3},
					ProfitLossRate:   domain.AssetAmount{KRW: 0.375, USD: 0.375},
					MarketDivision:   "NASDAQ",
					Type:             "STOCK",
				}},
			},
			{Market: "bond", Holdings: []domain.AssetSnapshotHolding{}},
		},
	}
}

func TestWriteAssetSnapshotDetailTableKeepsSummarySectionsAndHoldings(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := WriteAssetSnapshot(&out, FormatTable, snapshotDetailFixture()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"all_accounts", "2026-09-03", "11,000", "3,000", "37.50%", "kr", "option", "us", "bond", "AAPL", "Apple", "NASDAQ"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

func TestWriteAssetSnapshotMachineFormatsKeepSummaryAndHoldings(t *testing.T) {
	t.Parallel()
	var jsonOut bytes.Buffer
	if err := WriteAssetSnapshot(&jsonOut, FormatJSON, snapshotDetailFixture()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(jsonOut.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	markets, ok := got["markets"].([]any)
	if !ok || len(markets) != 4 {
		t.Fatalf("JSON markets = %#v", got["markets"])
	}

	var csvOut bytes.Buffer
	if err := WriteAssetSnapshot(&csvOut, FormatCSV, snapshotDetailFixture()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"row_type,market,product_code,isin,symbol,name,market_division,type,quantity,purchase_price_krw,purchase_price_usd,principal_or_purchase_krw,principal_or_purchase_usd,evaluated_krw,evaluated_usd,profit_loss_krw,profit_loss_usd,profit_loss_rate_krw,profit_loss_rate_usd",
		"summary,total",
		"summary,us",
		"holding,us,US.AAPL,US0378331005,AAPL,Apple,NASDAQ,STOCK,2,4000,4,8000,8,11000,11,3000,3,0.375,0.375",
	} {
		if !strings.Contains(csvOut.String(), want) {
			t.Errorf("CSV missing %q:\n%s", want, csvOut.String())
		}
	}
}
