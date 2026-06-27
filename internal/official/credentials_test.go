package official_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

func TestLoadCredentialsEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.json")
	if err := official.SaveCredentials(file, official.Credentials{APIKey: "filek", SecretKey: "files"}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"TOSSCTL_OPENAPI_KEY": "envk", "TOSSCTL_OPENAPI_SECRET": "envs"}
	c, err := official.LoadCredentials(func(k string) string { return env[k] }, file)
	if err != nil {
		t.Fatal(err)
	}
	if c.APIKey != "envk" {
		t.Fatalf("env should win, got %q", c.APIKey)
	}
}

func TestSaveCredentialsIs0600(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.json")
	if err := official.SaveCredentials(file, official.Credentials{APIKey: "k", SecretKey: "s"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("want 0600, got %v", fi.Mode().Perm())
	}
}

func TestLoadCredentialsNoneReturnsNil(t *testing.T) {
	c, err := official.LoadCredentials(func(string) string { return "" }, filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || c != nil {
		t.Fatalf("want nil,nil got %v,%v", c, err)
	}
}

func TestMaskedKey(t *testing.T) {
	c := official.Credentials{APIKey: "tsck_live_9I24L3TIMVgiFfakZJaVLA"}
	if got := c.MaskedKey(); got != "tsck_live_…aVLA" {
		t.Fatalf("got %q", got)
	}
}

func TestLoadCredentialsMissingEnvFallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.json")
	if err := official.SaveCredentials(file, official.Credentials{APIKey: "filek", SecretKey: "files"}); err != nil {
		t.Fatal(err)
	}
	// Only one env var set — incomplete, must fall back to file.
	env := map[string]string{"TOSSCTL_OPENAPI_KEY": "envk"}
	c, err := official.LoadCredentials(func(k string) string { return env[k] }, file)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected credentials from file, got nil")
	}
	if c.APIKey != "filek" {
		t.Fatalf("expected file key, got %q", c.APIKey)
	}
}

func TestSaveCredentialsCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "nested", "deep", "c.json")
	if err := official.SaveCredentials(file, official.Credentials{APIKey: "k", SecretKey: "s"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("file not created: %v", err)
	}
	// Check parent dir perms.
	fi, err := os.Stat(filepath.Join(dir, "nested"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("want dir 0700, got %v", fi.Mode().Perm())
	}
}

func TestDeleteCredentials(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.json")
	if err := official.SaveCredentials(file, official.Credentials{APIKey: "k", SecretKey: "s"}); err != nil {
		t.Fatal(err)
	}
	if err := official.DeleteCredentials(file); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
	// Deleting absent file is not an error.
	if err := official.DeleteCredentials(file); err != nil {
		t.Fatalf("delete absent should be nil, got %v", err)
	}
}

func TestLoadCredentialsMalformedReturnsError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.json")
	if err := os.WriteFile(file, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyEnv := func(string) string { return "" }
	c, err := official.LoadCredentials(emptyEnv, file)
	if err == nil {
		t.Fatal("expected error for malformed file, got nil")
	}
	if c != nil {
		t.Fatalf("expected nil credentials on error, got %v", c)
	}
}

func TestMaskedKeyShort(t *testing.T) {
	c := official.Credentials{APIKey: "short"}
	if got := c.MaskedKey(); got != "…" {
		t.Fatalf("short key should return …, got %q", got)
	}
}
