// SPDX-License-Identifier: AGPL-3.0-only

// Package testpath provides filesystem helpers for tests that exercise
// Hime-san's deliberate symlink boundaries.
package testpath

import (
	"os"
	"path/filepath"
	"testing"
)

// TempDir returns the physical path to a fresh directory owned by the test.
//
// macOS commonly exposes its temporary directory through /var even though
// /var is a root-owned system symlink to /private/var. Resolving a directory
// immediately after testing.TB creates it keeps tests portable without
// teaching production path validation to follow user-controlled symlinks.
func TempDir(t testing.TB) string {
	t.Helper()

	directory := t.TempDir()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatalf("resolve test temporary directory: %v", err)
	}
	resolved = filepath.Clean(resolved)
	info, err := os.Lstat(resolved)
	if err != nil {
		t.Fatalf("inspect resolved test temporary directory: %v", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("resolved test temporary directory is not a physical directory: %s", resolved)
	}
	return resolved
}
