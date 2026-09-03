package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteSecuritiesTransferAccountsMasksEveryFormatByDefault(t *testing.T) {
	t.Parallel()
	accounts := domain.SecuritiesTransferAccounts{
		AccountScope:   "scope-123",
		OwnAccounts:    []domain.SecuritiesTransferAccount{{BankCode: "092", AccountNo: "123-456-789", AccountID: "own-1"}},
		RecentAccounts: []domain.SecuritiesTransferAccount{{BankCode: "088", AccountNo: "987-654-321"}},
	}
	for _, format := range []Format{FormatTable, FormatJSON, FormatCSV} {
		var out bytes.Buffer
		if err := WriteSecuritiesTransferAccounts(&out, format, accounts, false); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if strings.Contains(out.String(), "123-456-789") || strings.Contains(out.String(), "987-654-321") || strings.Contains(out.String(), "own-1") {
			t.Fatalf("account data leaked in %s: %s", format, out.String())
		}
		for _, want := range []string{"scope-123", "092", "088"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("%s missing %q: %s", format, want, out.String())
			}
		}
	}
}

func TestWriteSecuritiesTransferAccountsFullRevealsNumbers(t *testing.T) {
	t.Parallel()
	accounts := domain.SecuritiesTransferAccounts{
		OwnAccounts:    []domain.SecuritiesTransferAccount{{BankCode: "092", AccountNo: "123-456-789", AccountID: "own-1"}},
		RecentAccounts: []domain.SecuritiesTransferAccount{{BankCode: "088", AccountNo: "987-654-321"}},
	}
	var out bytes.Buffer
	if err := WriteSecuritiesTransferAccounts(&out, FormatJSON, accounts, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "123-456-789") || !strings.Contains(out.String(), "987-654-321") {
		t.Fatalf("full output missing account numbers: %s", out.String())
	}
	if strings.Contains(out.String(), "own-1") || strings.Contains(out.String(), "account_id") {
		t.Fatalf("internal account ID leaked in full output: %s", out.String())
	}
}
