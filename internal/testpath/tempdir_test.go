// SPDX-License-Identifier: AGPL-3.0-only

package testpath

import (
	"path/filepath"
	"testing"
)

func TestTempDirReturnsPhysicalPath(t *testing.T) {
	directory := TempDir(t)
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(resolved) != directory {
		t.Fatalf("TempDir() = %q, physical path = %q", directory, resolved)
	}
}
