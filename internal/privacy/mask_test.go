package privacy

import (
	"fmt"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestMaskingBoundaries(t *testing.T) {
	t.Parallel()
	accountCases := []struct{ in, want string }{
		{"", ""},
		{"1", "*"},
		{"123", "***"},
		{"1234", "*234"},
		{"12-3", "**-*"},
		{"---", "---"},
		{"가나다라", "*나다라"},
	}
	for _, tc := range accountCases {
		t.Run("account_"+fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			if got := AccountNumber(tc.in); got != tc.want {
				t.Fatalf("AccountNumber(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
	nameCases := []struct{ in, want string }{{"", ""}, {"홍", "*"}, {"홍길동", "홍**"}, {"A B", "A**"}}
	for _, tc := range nameCases {
		t.Run("name_"+fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			if got := Name(tc.in); got != tc.want {
				t.Fatalf("Name(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRedactAccountOverviewMasksAllGroupsWithoutMutatingInput(t *testing.T) {
	t.Parallel()
	input := domain.AccountOverview{
		Accounts:      []domain.AccountOverviewItem{{AccountNo: "123-45-678901"}},
		MinorAccounts: []domain.AccountOverviewItem{{AccountNo: "987-65-432109"}},
	}
	got := RedactAccountOverview(input)
	if got.Accounts[0].AccountNo == input.Accounts[0].AccountNo || got.MinorAccounts[0].AccountNo == input.MinorAccounts[0].AccountNo {
		t.Fatalf("numbers not redacted: %#v", got)
	}
	if input.Accounts[0].AccountNo != "123-45-678901" {
		t.Fatalf("input mutated: %#v", input)
	}
}

func TestRedactOpenBankingStatusMasksNameAndAccount(t *testing.T) {
	t.Parallel()
	input := domain.OpenBankingStatus{
		HolderName:       "홍길동",
		ConnectedAccount: &domain.OpenBankingAccount{AccountNo: "123-456-789", BankCode: "088", OpenBankingID: 42},
	}
	got := RedactOpenBankingStatus(input)
	if got.HolderName != "홍**" || got.ConnectedAccount.AccountNo == "123-456-789" {
		t.Fatalf("identity not redacted: %#v", got)
	}
	if got.ConnectedAccount.BankCode != "088" || got.ConnectedAccount.OpenBankingID != 42 {
		t.Fatalf("non-secret connection metadata changed: %#v", got)
	}
	if input.ConnectedAccount.AccountNo != "123-456-789" {
		t.Fatalf("input mutated: %#v", input)
	}
}
