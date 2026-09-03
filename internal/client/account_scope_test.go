package client

import (
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestAccountScopeIsSessionKeyedStableAndAccountSpecific(t *testing.T) {
	t.Parallel()
	clientA := New(Config{Session: &session.Session{Cookies: map[string]string{"SESSION": "secret-a"}}})
	clientB := New(Config{Session: &session.Session{Cookies: map[string]string{"SESSION": "secret-b"}}})

	one := clientA.accountScope("1")
	if one == "" || one == "1" || one == "unavailable" {
		t.Fatalf("unsafe account scope %q", one)
	}
	if one != clientA.accountScope("1") {
		t.Fatal("account scope changed within one session")
	}
	if one == clientA.accountScope("2") {
		t.Fatal("different accounts received the same scope")
	}
	if one == clientB.accountScope("1") {
		t.Fatal("account scope must change with the session secret")
	}
	if one == confirmation.Token("account:1") {
		t.Fatal("account scope regressed to a reversible unkeyed digest")
	}
}

func TestConfirmationKeyIsStablePurposeAndSessionBound(t *testing.T) {
	t.Parallel()
	clientA := New(Config{Session: &session.Session{Cookies: map[string]string{"SESSION": "secret-a"}}})
	clientB := New(Config{Session: &session.Session{Cookies: map[string]string{"SESSION": "secret-b"}}})
	one := clientA.ConfirmationKey("watchlist")
	if len(one) == 0 || string(one) != string(clientA.ConfirmationKey("watchlist")) {
		t.Fatal("confirmation key must be available and stable within one session")
	}
	if string(one) == string(clientA.ConfirmationKey("price-alert")) {
		t.Fatal("confirmation keys must be purpose-separated")
	}
	if string(one) == string(clientB.ConfirmationKey("watchlist")) {
		t.Fatal("confirmation key must change with the WTS session")
	}
}
