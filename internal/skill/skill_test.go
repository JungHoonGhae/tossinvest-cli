package skill

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestFilesContainsSkillAndCatalog(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	want := []string{"SKILL.md", "references/operations.md"}
	for _, name := range want {
		data, ok := files[name]
		if !ok {
			t.Fatalf("Files missing %q (got %v)", name, keys(files))
		}
		if len(data) == 0 {
			t.Fatalf("embedded %q is empty", name)
		}
	}
	if len(files) != len(want) {
		t.Fatalf("expected exactly %d files, got %d: %v", len(want), len(files), keys(files))
	}
}

func TestInstallWritesEmbeddedContent(t *testing.T) {
	dir := t.TempDir()
	written, err := Install(dir)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(written) != 2 {
		t.Fatalf("expected 2 written paths, got %d: %v", len(written), written)
	}
	if !sort.StringsAreSorted(written) {
		t.Fatalf("written paths not sorted (output must be stable): %v", written)
	}

	embedded, _ := Files()
	for rel, want := range embedded {
		got, err := os.ReadFile(filepath.Join(dir, Name, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read installed %q: %v", rel, err)
		}
		if string(got) != string(want) {
			t.Fatalf("installed %q differs from embedded source", rel)
		}
	}
}

func TestIsInstalled(t *testing.T) {
	dir := t.TempDir()
	if IsInstalled(dir) {
		t.Fatal("empty dir reported as installed")
	}
	if _, err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !IsInstalled(dir) {
		t.Fatal("after Install, not reported as installed")
	}
}

func TestInstallOverwritesStale(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, Name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, _ := os.ReadFile(stale)
	if string(got) == "stale" {
		t.Fatal("Install did not overwrite a stale file (reinstall must refresh)")
	}
}

func TestDefaultDirEnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_SKILLS_DIR", filepath.Join("tmp", "custom-skills"))
	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if dir != filepath.Join("tmp", "custom-skills") {
		t.Fatalf("env override ignored: %q", dir)
	}
}

func TestDefaultDirFallsBackToClaudeHome(t *testing.T) {
	t.Setenv("CLAUDE_SKILLS_DIR", "")
	dir, err := DefaultDir()
	if err != nil {
		t.Skipf("no home dir in this environment: %v", err)
	}
	if filepath.Base(dir) != "skills" || filepath.Base(filepath.Dir(dir)) != ".claude" {
		t.Fatalf("expected ~/.claude/skills, got %q", dir)
	}
}

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
