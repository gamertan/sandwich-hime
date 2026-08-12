// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type watchRoot struct {
	path       string
	allRegular bool
}

type fileFingerprint struct {
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

type fileSnapshot map[string]fileFingerprint

func makeWatchRoots(rootDir string, cfg Config) []watchRoot {
	roots := make([]watchRoot, 0, 1+len(cfg.SourceRoots)+len(cfg.AdditionalWatchRoots))
	// GoPackage may live outside a narrowly configured template source root.
	// Watching the containing module (while respecting nested module boundaries)
	// makes ordinary Go edits rebuild without asking users to duplicate roots.
	rootDir = filepath.Clean(rootDir)
	roots = append(roots, watchRoot{path: rootDir})
	seen := map[string]bool{rootDir: true}
	for _, root := range cfg.SourceRoots {
		path := resolveProjectPath(rootDir, root)
		if !seen[path] {
			roots = append(roots, watchRoot{path: path})
			seen[path] = true
		}
	}
	for _, root := range cfg.AdditionalWatchRoots {
		path := resolveProjectPath(rootDir, root)
		if seen[path] {
			for index := range roots {
				if roots[index].path == path {
					roots[index].allRegular = true
				}
			}
			continue
		}
		roots = append(roots, watchRoot{path: path, allRegular: true})
		seen[path] = true
	}
	return roots
}

func takeSnapshot(roots []watchRoot) (fileSnapshot, error) {
	snapshot := make(fileSnapshot)
	var problems []error
	for _, root := range roots {
		rootInfo, err := os.Lstat(root.path)
		if err != nil {
			problems = append(problems, fmt.Errorf("watch %s: %w", root.path, err))
			continue
		}
		if rootInfo.Mode()&os.ModeSymlink != 0 {
			problems = append(problems, fmt.Errorf("watch %s: symbolic-link roots are not followed", root.path))
			continue
		}
		err = filepath.WalkDir(root.path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				problems = append(problems, fmt.Errorf("watch %s: %w", path, walkErr))
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if path != root.path && entry.IsDir() {
				if shouldSkipWatchDirectory(entry.Name()) {
					return filepath.SkipDir
				}
				if !root.allRegular {
					if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
						return filepath.SkipDir
					}
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".sando.go") {
				return nil
			}
			if !root.allRegular && !isDevelopmentSource(path) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				problems = append(problems, fmt.Errorf("watch %s: %w", path, err))
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			snapshot[path] = fileFingerprint{
				size:    info.Size(),
				mode:    info.Mode(),
				modTime: info.ModTime(),
			}
			return nil
		})
		if err != nil {
			problems = append(problems, fmt.Errorf("watch %s: %w", root.path, err))
		}
	}
	return snapshot, errors.Join(problems...)
}

func shouldSkipWatchDirectory(name string) bool {
	switch name {
	case ".git", ".hg", ".svn", ".himesan", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func isDevelopmentSource(path string) bool {
	name := filepath.Base(path)
	// Generated output is rebuilt from its .sando source and is therefore not a
	// separate watch trigger. Excluding it prevents generation from causing a
	// redundant build while still preserving edits made during an active build.
	if strings.HasSuffix(strings.ToLower(name), ".sando.go") {
		return false
	}
	switch name {
	case "go.mod", "go.sum", "go.work", "go.work.sum", "himesan.json":
		return true
	}
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".go" || extension == ".sando"
}

func snapshotsEqual(left, right fileSnapshot) bool {
	if len(left) != len(right) {
		return false
	}
	for path, leftFingerprint := range left {
		if rightFingerprint, ok := right[path]; !ok || rightFingerprint != leftFingerprint {
			return false
		}
	}
	return true
}
