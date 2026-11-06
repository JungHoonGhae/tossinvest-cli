package selfupdate

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunDevBuildReturnsErrDevBuild(t *testing.T) {
	err := Run(context.Background(), Options{
		Method: MethodDev,
		Out:    io.Discard,
		ErrOut: io.Discard,
	})
	if !errors.Is(err, ErrDevBuild) {
		t.Fatalf("expected ErrDevBuild, got %v", err)
	}
}

func TestRunUnknownMethodReturnsError(t *testing.T) {
	err := Run(context.Background(), Options{
		Method: InstallMethod(99),
		Out:    io.Discard,
		ErrOut: io.Discard,
	})
	if err == nil {
		t.Fatal("expected an error for an unknown InstallMethod, got nil")
	}
}

func TestRunBinaryWithoutVersionRefuses(t *testing.T) {
	err := Run(context.Background(), Options{
		Method: MethodBinary,
		Out:    io.Discard,
		ErrOut: io.Discard,
	})
	if err == nil {
		t.Fatal("expected an error for MethodBinary without a pinned version, got nil")
	}
}

func TestPinnedTagNormalizes(t *testing.T) {
	for in, want := range map[string]string{"0.54.1": "v0.54.1", "v0.54.1": "v0.54.1"} {
		got, err := pinnedTag(in)
		if err != nil {
			t.Fatalf("pinnedTag(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Fatalf("pinnedTag(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := pinnedTag(""); err == nil {
		t.Fatal("pinnedTag(\"\") should fail closed, got nil error")
	}
}

// A failed or empty download must fail the update instead of piping nothing
// into sh (which exits 0 and would report success).
func TestUnixInstallScriptFailsClosedOnDownloadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell only")
	}
	bin := t.TempDir()
	run := func(curlBody string) error {
		curl := "#!/bin/sh\n" + curlBody + "\n"
		if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(curl), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", "-c", unixInstallScript)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TOSSCTL_INSTALL_URL=https://example.invalid/install.sh")
		return cmd.Run()
	}
	if err := run("exit 22"); err == nil {
		t.Fatal("curl failure must fail the install")
	}
	if err := run("exit 0"); err == nil {
		t.Fatal("empty download must fail the install")
	}
	if err := run("echo 'exit 0'"); err != nil {
		t.Fatalf("a downloaded script must run: %v", err)
	}
}
