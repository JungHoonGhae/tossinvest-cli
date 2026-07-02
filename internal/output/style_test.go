package output

import (
	"bytes"
	"os"
	"testing"
)

func TestColorEnabledGate(t *testing.T) {
	var buf bytes.Buffer // not *os.File → not a terminal
	if colorEnabled(&buf, FormatTable) {
		t.Fatal("buffer must not enable color")
	}
	if colorEnabled(&buf, FormatJSON) {
		t.Fatal("json never colored")
	}
}

func TestColorEnabledRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// even a real terminal must be disabled
	if colorEnabled(os.Stdout, FormatTable) {
		t.Fatal("NO_COLOR must disable")
	}
}

func TestProfitTextDisabledIsPlain(t *testing.T) {
	if got := profitText("+5%", 5, false); got != "+5%" {
		t.Fatalf("disabled must be plain, got %q", got)
	}
}

func TestProfitTextEnabledColors(t *testing.T) {
	pos := profitText("+5%", 5, true)
	neg := profitText("-5%", -5, true)
	if pos == "+5%" || neg == "-5%" {
		t.Fatal("enabled must wrap with ANSI")
	}
	if pos == neg {
		t.Fatal("up/down must differ")
	}
}

func TestStyleSuccessDisabled(t *testing.T) {
	if got := StyleSuccess("ok", false); got != "ok" {
		t.Errorf("StyleSuccess with enabled=false = %q, want unchanged %q", got, "ok")
	}
}

func TestStyleSuccessEnabled(t *testing.T) {
	got := StyleSuccess("ok", true)
	if got == "ok" {
		t.Error("StyleSuccess with enabled=true should add ANSI styling, got unchanged string")
	}
}

func TestStyleFailDisabled(t *testing.T) {
	if got := StyleFail("bad", false); got != "bad" {
		t.Errorf("StyleFail with enabled=false = %q, want unchanged %q", got, "bad")
	}
}

func TestStyleFailEnabled(t *testing.T) {
	got := StyleFail("bad", true)
	if got == "bad" {
		t.Error("StyleFail with enabled=true should add ANSI styling, got unchanged string")
	}
}

func TestStyleHighlightDisabled(t *testing.T) {
	if got := StyleHighlight("0.15.0", false); got != "0.15.0" {
		t.Errorf("StyleHighlight with enabled=false = %q, want unchanged %q", got, "0.15.0")
	}
}

func TestStyleHighlightEnabled(t *testing.T) {
	got := StyleHighlight("0.15.0", true)
	if got == "0.15.0" {
		t.Error("StyleHighlight with enabled=true should add ANSI styling, got unchanged string")
	}
}
