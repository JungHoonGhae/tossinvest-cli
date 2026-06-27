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
