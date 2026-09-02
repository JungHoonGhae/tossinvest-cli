package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func sampleAccountOverview() domain.AccountOverview {
	return domain.AccountOverview{
		TotalAssetAmount: 1239000,
		Accounts: []domain.AccountOverviewItem{{
			AccountName: "일반계좌", AccountNo: "137-01-000930", PendingOrderCount: 2, TotalAssetAmount: 1234000,
		}},
		MinorAccounts: []domain.AccountOverviewItem{{
			AccountName: "미성년계좌", AccountNo: "246-02-000123", TotalAssetAmount: 5000,
		}},
	}
}

func TestAccountOverviewMasksEveryFormatByDefault(t *testing.T) {
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		t.Run(string(format), func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteAccountOverview(&buf, format, sampleAccountOverview(), false); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if strings.Contains(out, "137-01-000930") || strings.Contains(out, "246-02-000123") {
				t.Fatalf("account number leaked in %s: %s", format, out)
			}
			if !strings.Contains(out, "***") {
				t.Fatalf("mask missing in %s: %s", format, out)
			}
		})
	}
}

func TestAccountOverviewFullRevealsNumbers(t *testing.T) {
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var buf bytes.Buffer
		if err := WriteAccountOverview(&buf, format, sampleAccountOverview(), true); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "137-01-000930") {
			t.Fatalf("--full did not reveal number in %s", format)
		}
	}
}

func TestAccountOverviewJSONPreservesGroups(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccountOverview(&buf, FormatJSON, sampleAccountOverview(), false); err != nil {
		t.Fatal(err)
	}
	var got domain.AccountOverview
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Accounts) != 1 || len(got.MinorAccounts) != 1 {
		t.Fatalf("groups lost: %#v", got)
	}
}

func TestAccountOverviewJSONUsesEmptyArrays(t *testing.T) {
	var buf bytes.Buffer
	overview := domain.AccountOverview{Accounts: []domain.AccountOverviewItem{}, MinorAccounts: []domain.AccountOverviewItem{}}
	if err := WriteAccountOverview(&buf, FormatJSON, overview, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"accounts": null`) || strings.Contains(buf.String(), `"minor_accounts": null`) {
		t.Fatalf("empty groups must stay arrays: %s", buf.String())
	}
}
