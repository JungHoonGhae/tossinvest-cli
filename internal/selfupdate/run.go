package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
)

// Options configures Run.
type Options struct {
	// Method is the install method to act on — normally the result of
	// DetectInstallMethod.
	Method InstallMethod
	// Version is the target release version without a "v" prefix (e.g. "0.54.1",
	// as reported by updatecheck). MethodBinary requires it so the installer
	// script and payload are pinned to that release tag instead of mutable
	// main. Empty refuses the update (fail-closed).
	Version string
	// Out and ErrOut receive the child process's stdout/stderr, streamed
	// live so users see brew/curl/install-script progress as it happens.
	Out    io.Writer
	ErrOut io.Writer
}

// ErrDevBuild is returned by Run when Options.Method is MethodDev — there is
// nothing to update a source build to.
var ErrDevBuild = errors.New("source build (dev) — use `git pull && make build` instead of `tossctl update`")

// Run performs the upgrade for opts.Method:
//   - MethodDev: returns ErrDevBuild without any I/O.
//   - MethodHomebrew: runs `brew upgrade tossctl` synchronously,
//     relaying its output, and returns its error (if any) wrapped with
//     context.
//   - MethodBinary on non-Windows: re-runs install.sh via `sh -c 'curl ... |
//     sh'` synchronously — safe because Unix allows replacing a running
//     binary's directory entry without disturbing the currently-executing
//     process. The script URL and the payload it fetches are both pinned to
//     opts.Version's release tag (never mutable main).
//   - MethodBinary on Windows: starts install.ps1 via PowerShell and returns
//     immediately without waiting for it to finish. install.ps1 needs this
//     process's file lock on tossctl.exe released before it can overwrite
//     the binary, so Run hands off and lets the caller exit right after —
//     it does not (and cannot, without duplicating install.ps1's own
//     "tossctl is running" check) confirm the swap actually completed.
//     The script URL and payload are pinned to opts.Version like on Unix.
func Run(ctx context.Context, opts Options) error {
	switch opts.Method {
	case MethodDev:
		return ErrDevBuild
	case MethodHomebrew:
		cmd := exec.CommandContext(ctx, "brew", "upgrade", "tossctl")
		cmd.Stdout = opts.Out
		cmd.Stderr = opts.ErrOut
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("brew upgrade tossctl: %w", err)
		}
		return nil
	case MethodBinary:
		if runtime.GOOS == "windows" {
			return runWindowsInstaller(opts)
		}
		return runUnixInstaller(ctx, opts)
	default:
		return fmt.Errorf("unknown install method %v", opts.Method)
	}
}

// pinnedTag normalizes opts.Version to "v<ver>" form and refuses an empty
// version: running an unpinned installer would execute mutable main.
func pinnedTag(ver string) (string, error) {
	if strings.TrimSpace(ver) == "" {
		return "", errors.New("refusing self-update without a pinned target version (update check returned empty)")
	}
	return "v" + strings.TrimPrefix(strings.TrimSpace(ver), "v"), nil
}

func runUnixInstaller(ctx context.Context, opts Options) error {
	tag, err := pinnedTag(opts.Version)
	if err != nil {
		return err
	}
	script := fmt.Sprintf("curl -fsSL %s | sh", version.RawTagURL("install.sh", tag))
	cmd := exec.CommandContext(ctx, "sh", "-c", script)
	cmd.Env = append(os.Environ(), "TOSSCTL_VERSION="+tag)
	cmd.Stdout = opts.Out
	cmd.Stderr = opts.ErrOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running install.sh: %w", err)
	}
	return nil
}

func runWindowsInstaller(opts Options) error {
	tag, err := pinnedTag(opts.Version)
	if err != nil {
		return err
	}
	script := fmt.Sprintf("irm %s | iex", version.RawTagURL("install.ps1", tag))
	// Intentionally exec.Command, not exec.CommandContext: this process is
	// about to exit so the child can take over the file lock on tossctl.exe;
	// tying its lifetime to our context would risk killing it on cancel.
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.Env = append(os.Environ(), "TOSSCTL_VERSION="+tag)
	cmd.Stdout = opts.Out
	cmd.Stderr = opts.ErrOut
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting install.ps1: %w", err)
	}
	fmt.Fprintln(opts.Out, "Handed off to install.ps1 — this process will exit now so it can replace tossctl.exe.")
	return nil
}
