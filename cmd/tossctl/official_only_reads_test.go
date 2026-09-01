package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestAccountBuyingPowerWithoutOfficialKeyReturnsLoginHint(t *testing.T) {
	t.Setenv("TOSSCTL_OPENAPI_KEY", "")
	t.Setenv("TOSSCTL_OPENAPI_SECRET", "")

	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config-dir", t.TempDir(), "account", "buying-power"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("official key가 없는데 성공했다")
	}
	if !strings.Contains(err.Error(), "requires Toss official Open API access") ||
		!strings.Contains(err.Error(), "tossctl openapi login") {
		t.Fatalf("로그인 방법을 안내해야 한다: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("실패한 명령이 stdout을 오염시켰다: %q", stdout.String())
	}
}

func TestMarketBusinessDaysWithoutOfficialKeyReturnsLoginHint(t *testing.T) {
	t.Setenv("TOSSCTL_OPENAPI_KEY", "")
	t.Setenv("TOSSCTL_OPENAPI_SECRET", "")

	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config-dir", t.TempDir(), "market", "business-days", "KR"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("official key가 없는데 성공했다")
	}
	if !strings.Contains(err.Error(), "requires Toss official Open API access") ||
		!strings.Contains(err.Error(), "tossctl openapi login") {
		t.Fatalf("로그인 방법을 안내해야 한다: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("실패한 명령이 stdout을 오염시켰다: %q", stdout.String())
	}
}
