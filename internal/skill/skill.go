// Package skill ships the tossctl agent skill inside the binary and installs it
// into a Claude skills directory.
//
// The skill (an agent-facing guide to driving tossctl) is version-controlled at
// skills/tossctl/ in the repo and is already installable with a curl|sh
// one-liner. Embedding it here adds the CLI-native path the project set out to
// have — `tossctl skill install` — which works offline and always ships the
// skill that matches the running binary, with no network fetch and no second
// tool.
//
// The files under assets/ are a mirror of the canonical skills/tossctl/ source,
// kept in sync by tools/build_skill.py; CI (`build_skill.py --check`) fails any
// drift, so the embedded copy can never silently fall behind the distributable
// one.
package skill

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

//go:embed assets/SKILL.md assets/references/operations.md
var assets embed.FS

// Name is the skill's directory name inside a skills directory.
const Name = "tossctl"

const assetRoot = "assets"

// Files returns the skill's files keyed by their path relative to the skill
// root (e.g. "SKILL.md", "references/operations.md"), using forward slashes.
func Files() (map[string][]byte, error) {
	out := map[string][]byte{}
	err := fs.WalkDir(assets, assetRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(assetRoot, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DefaultDir resolves the skills directory: $CLAUDE_SKILLS_DIR when set,
// otherwise ~/.claude/skills. It matches skills/install.sh so the two installers
// land in the same place.
func DefaultDir() (string, error) {
	if dir := os.Getenv("CLAUDE_SKILLS_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills"), nil
}

// InstalledDir is where this skill lives under skillsDir once installed.
func InstalledDir(skillsDir string) string {
	return filepath.Join(skillsDir, Name)
}

// IsInstalled reports whether the skill's SKILL.md is present under skillsDir.
func IsInstalled(skillsDir string) bool {
	_, err := os.Stat(filepath.Join(InstalledDir(skillsDir), "SKILL.md"))
	return err == nil
}

// Install writes the embedded skill files into skillsDir/tossctl, creating
// parent directories as needed, and returns the written absolute-or-relative
// paths sorted for stable output. An existing install is overwritten so a
// reinstall picks up a newer binary's skill.
func Install(skillsDir string) ([]string, error) {
	files, err := Files()
	if err != nil {
		return nil, err
	}
	dest := InstalledDir(skillsDir)
	written := make([]string, 0, len(files))
	for rel, data := range files {
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return nil, err
		}
		written = append(written, target)
	}
	sort.Strings(written)
	return written, nil
}
