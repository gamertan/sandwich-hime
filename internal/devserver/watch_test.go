// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotWatchesSourcesAssetsAndStopsAtNestedModules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeWatchFile(t, filepath.Join(root, "main.go"), "package main")
	writeWatchFile(t, filepath.Join(root, "views", "home.sando"), "<h1>home</h1>")
	writeWatchFile(t, filepath.Join(root, "views", "home.sando.go"), "// generated")
	writeWatchFile(t, filepath.Join(root, "notes.txt"), "not watched")
	writeWatchFile(t, filepath.Join(root, "assets", "site.css"), "body{}")
	writeWatchFile(t, filepath.Join(root, "nested", "go.mod"), "module nested.test")
	writeWatchFile(t, filepath.Join(root, "nested", "ignored.go"), "package ignored")

	cfg := DefaultConfig()
	cfg.SourceRoots = []string{"views"}
	cfg.AdditionalWatchRoots = []string{"assets"}
	snapshot, err := takeSnapshot(makeWatchRoots(root, cfg))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"main.go", filepath.Join("views", "home.sando"), filepath.Join("assets", "site.css")} {
		if _, ok := snapshot[filepath.Join(root, want)]; !ok {
			t.Errorf("snapshot does not contain %s", want)
		}
	}
	for _, unwanted := range []string{"notes.txt", filepath.Join("views", "home.sando.go"), filepath.Join("nested", "go.mod"), filepath.Join("nested", "ignored.go")} {
		if _, ok := snapshot[filepath.Join(root, unwanted)]; ok {
			t.Errorf("snapshot unexpectedly contains %s", unwanted)
		}
	}

	before := snapshot
	writeWatchFile(t, filepath.Join(root, "assets", "site.css"), "body{color:green}")
	after, err := takeSnapshot(makeWatchRoots(root, cfg))
	if err != nil {
		t.Fatal(err)
	}
	if snapshotsEqual(before, after) {
		t.Fatal("asset change was not detected")
	}
}

func writeWatchFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
