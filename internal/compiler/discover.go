// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

var excludedDirectories = map[string]bool{
	".git":   true,
	".hg":    true,
	".svn":   true,
	"vendor": true,
}

func discover(ctx context.Context, paths []string) ([]string, []Diagnostic) {
	return discoverWithOptions(ctx, paths, true)
}

// DiscoverSources finds .sando sources using the same filesystem, symlink,
// nested-module, VCS, and vendor boundaries as Generate and Check. It omits
// generated-output inspection because editor analysis does not own freshness.
func DiscoverSources(ctx context.Context, paths []string) ([]string, []Diagnostic) {
	return discoverWithOptions(ctx, paths, false)
}

func discoverWithOptions(ctx context.Context, paths []string, inspectGenerated bool) ([]string, []Diagnostic) {
	if len(paths) == 0 {
		paths = []string{"."}
	}
	filesByAbsolutePath := make(map[string]string)
	var diagnostics []Diagnostic

	for _, requested := range paths {
		if err := ctx.Err(); err != nil {
			diagnostics = append(diagnostics, diagnostic(requested, sourcePosition{Line: 1, Column: 1}, "HIM2001", "operation canceled: "+err.Error()))
			break
		}
		clean := filepath.Clean(requested)
		if symlinkParent, symlinkErr := firstSymlinkComponent(clean); symlinkErr != nil {
			diagnostics = append(diagnostics, diagnostic(clean, sourcePosition{Line: 1, Column: 1}, "HIM2002", "cannot inspect path ancestry: "+symlinkErr.Error()))
			continue
		} else if symlinkParent != "" {
			diagnostics = append(diagnostics, diagnostic(clean, sourcePosition{Line: 1, Column: 1}, "HIM2003", fmt.Sprintf("symlink paths are not followed (through %s)", symlinkParent)))
			continue
		}
		info, err := os.Lstat(clean)
		if err != nil {
			diagnostics = append(diagnostics, diagnostic(clean, sourcePosition{Line: 1, Column: 1}, "HIM2002", "cannot inspect path: "+err.Error()))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, diagnostic(clean, sourcePosition{Line: 1, Column: 1}, "HIM2003", "symlink paths are not followed"))
			continue
		}
		if !info.IsDir() {
			if filepath.Ext(clean) != ".sando" {
				diagnostics = append(diagnostics, diagnostic(clean, sourcePosition{Line: 1, Column: 1}, "HIM2004", "explicit source file must use the .sando extension"))
				continue
			}
			absolute, absoluteErr := filepath.Abs(clean)
			if absoluteErr != nil {
				diagnostics = append(diagnostics, diagnostic(clean, sourcePosition{Line: 1, Column: 1}, "HIM2005", "cannot resolve source path: "+absoluteErr.Error()))
				continue
			}
			filesByAbsolutePath[absolute] = clean
			continue
		}

		root := clean
		rootInfo := info
		walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				diagnostics = append(diagnostics, diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2006", "cannot inspect path during discovery: "+walkErr.Error()))
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if path != root && entry.IsDir() && excludedDirectories[entry.Name()] {
				return filepath.SkipDir
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if path != root && entry.IsDir() {
				entryInfo, statErr := entry.Info()
				if statErr != nil {
					diagnostics = append(diagnostics, diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2007", "cannot inspect directory: "+statErr.Error()))
					return filepath.SkipDir
				}
				if !sameFilesystem(rootInfo, entryInfo) {
					diagnostics = append(diagnostics, Diagnostic{Path: path, Line: 1, Column: 1, Code: "HIM2901", Severity: SeverityWarning, Message: "skipped mounted filesystem boundary"})
					return filepath.SkipDir
				}
				for _, marker := range []string{".git", ".hg", ".svn"} {
					markerPath := filepath.Join(path, marker)
					if _, markerErr := os.Lstat(markerPath); markerErr == nil {
						return filepath.SkipDir
					} else if !os.IsNotExist(markerErr) {
						diagnostics = append(diagnostics, diagnostic(markerPath, sourcePosition{Line: 1, Column: 1}, "HIM2011", "cannot inspect nested VCS boundary: "+markerErr.Error()))
						return filepath.SkipDir
					}
				}
				modulePath := filepath.Join(path, "go.mod")
				if moduleInfo, moduleErr := os.Lstat(modulePath); moduleErr == nil {
					if !moduleInfo.Mode().IsRegular() {
						diagnostics = append(diagnostics, diagnostic(modulePath, sourcePosition{Line: 1, Column: 1}, "HIM2008", "nested go.mod boundary is not a regular file; directory was skipped"))
					}
					return filepath.SkipDir
				} else if !os.IsNotExist(moduleErr) {
					diagnostics = append(diagnostics, diagnostic(modulePath, sourcePosition{Line: 1, Column: 1}, "HIM2008", "cannot inspect nested module boundary: "+moduleErr.Error()))
					return filepath.SkipDir
				}
			}
			if entry.IsDir() {
				return nil
			}
			if strings.HasSuffix(entry.Name(), ".sando.go") {
				if !inspectGenerated {
					return nil
				}
				entryInfo, statErr := entry.Info()
				if statErr != nil {
					diagnostics = append(diagnostics, diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2013", "cannot inspect possible generated output: "+statErr.Error()))
					return nil
				}
				if !entryInfo.Mode().IsRegular() {
					return nil
				}
				if orphanDiagnostic := inspectOwnedGeneratedOutput(path); orphanDiagnostic != nil {
					diagnostics = append(diagnostics, *orphanDiagnostic)
				}
				return nil
			}
			if filepath.Ext(entry.Name()) != ".sando" {
				return nil
			}
			entryInfo, statErr := entry.Info()
			if statErr != nil {
				diagnostics = append(diagnostics, diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2009", "cannot inspect source: "+statErr.Error()))
				return nil
			}
			if !entryInfo.Mode().IsRegular() {
				return nil
			}
			absolute, absoluteErr := filepath.Abs(path)
			if absoluteErr != nil {
				diagnostics = append(diagnostics, diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2005", "cannot resolve source path: "+absoluteErr.Error()))
				return nil
			}
			filesByAbsolutePath[absolute] = path
			return nil
		})
		if walkErr != nil && ctx.Err() != nil {
			diagnostics = append(diagnostics, diagnostic(root, sourcePosition{Line: 1, Column: 1}, "HIM2001", "operation canceled: "+ctx.Err().Error()))
		}
	}

	absolutePaths := make([]string, 0, len(filesByAbsolutePath))
	for absolute := range filesByAbsolutePath {
		absolutePaths = append(absolutePaths, absolute)
	}
	sort.Strings(absolutePaths)
	discovered := make([]string, 0, len(absolutePaths))
	for _, absolute := range absolutePaths {
		discovered = append(discovered, filesByAbsolutePath[absolute])
	}
	sort.SliceStable(discovered, func(i, j int) bool {
		left, _ := filepath.Abs(discovered[i])
		right, _ := filepath.Abs(discovered[j])
		return filepath.ToSlash(left) < filepath.ToSlash(right)
	})
	sortDiagnostics(diagnostics)
	return discovered, diagnostics
}

func inspectOwnedGeneratedOutput(path string) *Diagnostic {
	owned, err := hasGeneratedMarker(path)
	if err != nil {
		item := diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2013", "cannot inspect possible generated output: "+err.Error())
		return &item
	}
	if !owned {
		return nil
	}

	sourcePath := strings.TrimSuffix(path, ".go")
	info, err := os.Lstat(sourcePath)
	if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return nil
	}

	message := "owned generated output is orphaned because its adjacent .sando source is missing; review and remove the output explicitly"
	if err == nil {
		message = "owned generated output is orphaned because its adjacent .sando source is not a regular file; review and remove the output explicitly"
	} else if !os.IsNotExist(err) {
		message = "cannot inspect the adjacent .sando source for an owned generated output: " + err.Error()
	}
	item := diagnostic(path, sourcePosition{Line: 1, Column: 1}, "HIM2014", message)
	return &item
}

func hasGeneratedMarker(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	marker := []byte(generatedPrefix + "\n")
	prefix := make([]byte, len(marker))
	if _, err := io.ReadFull(file, prefix); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return string(prefix) == string(marker), nil
}

func firstSymlinkComponent(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	volume := filepath.VolumeName(absolute)
	remainder := strings.TrimPrefix(absolute, volume)
	remainder = strings.TrimPrefix(remainder, string(filepath.Separator))
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, lstatErr := os.Lstat(current)
		if lstatErr != nil {
			return "", lstatErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return current, nil
		}
	}
	return "", nil
}

func sameFilesystem(root, candidate fs.FileInfo) bool {
	rootDevice, rootOK := deviceNumber(root.Sys())
	candidateDevice, candidateOK := deviceNumber(candidate.Sys())
	return !rootOK || !candidateOK || rootDevice == candidateDevice
}

func deviceNumber(system any) (uint64, bool) {
	if system == nil {
		return 0, false
	}
	value := reflect.Indirect(reflect.ValueOf(system))
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return 0, false
	}
	field := value.FieldByName("Dev")
	if !field.IsValid() {
		return 0, false
	}
	switch field.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return field.Uint(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		device := field.Int()
		if device < 0 {
			return 0, false
		}
		return uint64(device), true
	default:
		return 0, false
	}
}
