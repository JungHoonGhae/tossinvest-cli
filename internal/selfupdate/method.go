// Package selfupdate detects how the running tossctl binary was installed
// and re-invokes the matching, already-tested upgrade mechanism (Homebrew,
// or the install.sh/install.ps1 scripts) — it does not reimplement
// download/checksum/extract logic itself.
package selfupdate

import (
	"path/filepath"
	"strings"
)

// InstallMethod identifies how the running tossctl binary was installed,
// which determines how `tossctl update` should upgrade it.
type InstallMethod int

const (
	// MethodDev is a source build (version.Version == "dev") — nothing to
	// update to.
	MethodDev InstallMethod = iota
	// MethodHomebrew is managed by Homebrew — delegate to `brew upgrade`.
	MethodHomebrew
	// MethodBinary was installed by install.sh or install.ps1 — re-run the
	// matching installer to fetch and swap in the latest release.
	MethodBinary
)

func (m InstallMethod) String() string {
	switch m {
	case MethodDev:
		return "dev"
	case MethodHomebrew:
		return "homebrew"
	case MethodBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// homebrewCellarPrefixes are path prefixes (after symlink resolution) that
// indicate a Homebrew-managed install of the tossctl-cli formula, across the
// three common Homebrew prefix locations (Apple Silicon, Intel, Linuxbrew).
var homebrewCellarPrefixes = []string{
	"/opt/homebrew/Cellar/tossctl-cli/",
	"/usr/local/Cellar/tossctl-cli/",
	"/home/linuxbrew/.linuxbrew/Cellar/tossctl-cli/",
}

// DetectInstallMethod classifies how the running binary at execPath was
// installed. execPath should already be symlink-resolved by the caller
// (typically via filepath.EvalSymlinks(os.Executable()) — Homebrew's `brew
// link` puts a symlink in .../bin, and only the resolved Cellar path
// contains the "Cellar/tossctl-cli/" segment this function looks for).
func DetectInstallMethod(execPath, currentVersion string) InstallMethod {
	if currentVersion == "dev" {
		return MethodDev
	}
	normalized := filepath.ToSlash(execPath)
	for _, prefix := range homebrewCellarPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return MethodHomebrew
		}
	}
	return MethodBinary
}
