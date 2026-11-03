package atomicfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestWriteFileIsAtomic guards issue #17: concurrent writers to the same path
// must never interleave (the old fixed ".tmp" name truncated siblings) and
// readers must never see a torn file.
func TestWriteFileIsAtomic(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sub", "state.json")

	const writers = 24
	payloads := make([]string, writers)
	for i := range payloads {
		payloads[i] = fmt.Sprintf(`{"n":%d,"pad":%q}`, i, strings.Repeat("x", 4000))
	}

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := WriteFile(path, []byte(payloads[i]), 0o600); err != nil {
				t.Errorf("writer %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("torn file is not valid JSON: %v", err)
	}
	matched := false
	for _, p := range payloads {
		if string(data) == p {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("final content is a mix of concurrent writes, not one whole payload")
	}
	// No stale temp files may be left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "state.json" {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
}

// TestWriteFileEnforcesPermissions guards issue #17: pre-existing wide files
// and directories are tightened, not inherited.
func TestWriteFileEnforcesPermissions(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "sub")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file perm = %o, want 600", fi.Mode().Perm())
	}
	if fi, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	} else if fi.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm = %o, want 700", fi.Mode().Perm())
	}
}
