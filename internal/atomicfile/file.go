// Package atomicfile persists small files atomically with private
// permissions. Every writer goes through a uniquely-named temp file in the
// same directory (no shared ".tmp" name for concurrent processes to truncate
// into each other), chmods it, fsyncs it, and renames it over the target — so
// readers never see a torn file and a crash never leaves a half-written one.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFile writes data to path atomically with the given permissions.
// The parent directory is created 0700 and chmodded to 0700 when present, so
// pre-existing wide directories (or umask effects) cannot leave secrets
// world-readable.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("atomicfile: chmod dir %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup: a crash may still leave the temp file, but its
	// unique name can never collide with another writer's.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("atomicfile: chmod %q: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	// Best-effort directory fsync so the rename itself survives a crash.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}
