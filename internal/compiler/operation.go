// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"sort"
)

// Generate compiles all discovered .sando files in memory, then atomically
// replaces only changed, Hime-san-owned .sando.go outputs. Any parse, context,
// format, cycle, or ownership error prevents every output write.
func Generate(ctx context.Context, paths []string) (Result, error) {
	compiled, result := compileOperation(ctx, paths)
	if hasErrors(result.Diagnostics) {
		return result, errorFromDiagnostics(result.Diagnostics)
	}

	// Validate every destination before performing the first mutation.
	for _, file := range compiled {
		info, err := os.Lstat(file.OutputPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2101", "cannot inspect generated output: "+err.Error()))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2102", "refusing to replace a non-regular or symlink output"))
			continue
		}
		existing, readErr := os.ReadFile(file.OutputPath)
		if readErr != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2103", "cannot read generated output: "+readErr.Error()))
			continue
		}
		if !bytes.HasPrefix(existing, []byte(generatedPrefix+"\n")) {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2104", "refusing to overwrite a file not owned by Hime-san"))
		}
	}
	sortDiagnostics(result.Diagnostics)
	if hasErrors(result.Diagnostics) {
		return result, errorFromDiagnostics(result.Diagnostics)
	}

	for index, file := range compiled {
		if err := ctx.Err(); err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.SourcePath, sourcePosition{Line: 1, Column: 1}, "HIM2001", "operation canceled: "+err.Error()))
			break
		}
		existing, readErr := os.ReadFile(file.OutputPath)
		if readErr == nil && generatedCodeEqual(existing, file.Code) {
			result.Files[index].Changed = false
			result.Unchanged++
			continue
		}
		// Generated Go can contain every literal present in its source. A new
		// output therefore must not be more permissive than the source file.
		// Execute bits are never meaningful for Go source and are stripped.
		mode := os.FileMode(file.sourceMode) & 0o666
		if info, statErr := os.Stat(file.OutputPath); statErr == nil {
			mode = info.Mode().Perm()
		}
		if writeErr := atomicWrite(file.OutputPath, file.Code, mode); writeErr != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2110", "atomic output replacement failed: "+writeErr.Error()))
			break
		}
		result.Files[index].Changed = true
		result.Changed++
	}
	sortDiagnostics(result.Diagnostics)
	return result, errorFromDiagnostics(result.Diagnostics)
}

// Check validates sources and reports missing or stale generated output without
// writing to the filesystem. Warnings, including trusted-value audit findings,
// do not cause Check to fail by themselves.
func Check(ctx context.Context, paths []string) (Result, error) {
	compiled, result := compileOperation(ctx, paths)
	if hasErrors(result.Diagnostics) {
		return result, errorFromDiagnostics(result.Diagnostics)
	}
	for index, file := range compiled {
		if err := ctx.Err(); err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.SourcePath, sourcePosition{Line: 1, Column: 1}, "HIM2001", "operation canceled: "+err.Error()))
			break
		}
		info, lstatErr := os.Lstat(file.OutputPath)
		if lstatErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2205", "generated output is a symlink or non-regular file"))
			continue
		}
		if lstatErr != nil && !os.IsNotExist(lstatErr) {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2202", "cannot inspect generated output: "+lstatErr.Error()))
			continue
		}
		existing, err := os.ReadFile(file.OutputPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.Files[index].Missing = true
				result.Missing++
				result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2201", "generated output is missing; run himesan generate"))
			} else {
				result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2202", "cannot read generated output: "+err.Error()))
			}
			continue
		}
		if generatedCodeEqual(existing, file.Code) {
			result.Unchanged++
			continue
		}
		result.Files[index].Stale = true
		result.Stale++
		if !bytes.HasPrefix(existing, []byte(generatedPrefix+"\n")) {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2203", "expected output exists but is not owned by Hime-san"))
		} else {
			result.Diagnostics = append(result.Diagnostics, diagnostic(file.OutputPath, sourcePosition{Line: 1, Column: 1}, "HIM2204", "generated output is stale; run himesan generate"))
		}
	}
	sortDiagnostics(result.Diagnostics)
	return result, errorFromDiagnostics(result.Diagnostics)
}

func compileOperation(ctx context.Context, paths []string) ([]CompiledFile, Result) {
	discovered, discoveryDiagnostics := discover(ctx, paths)
	result := Result{Discovered: len(discovered), Diagnostics: discoveryDiagnostics}
	compiled := make([]CompiledFile, 0, len(discovered))
	for _, sourcePath := range discovered {
		if err := ctx.Err(); err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(sourcePath, sourcePosition{Line: 1, Column: 1}, "HIM2001", "operation canceled: "+err.Error()))
			break
		}
		info, lstatErr := os.Lstat(sourcePath)
		if lstatErr != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(sourcePath, sourcePosition{Line: 1, Column: 1}, "HIM2010", "cannot inspect source before compilation: "+lstatErr.Error()))
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			result.Diagnostics = append(result.Diagnostics, diagnostic(sourcePath, sourcePosition{Line: 1, Column: 1}, "HIM2012", "source changed into a symlink or non-regular file during discovery"))
			continue
		}
		source, err := os.ReadFile(sourcePath)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic(sourcePath, sourcePosition{Line: 1, Column: 1}, "HIM2010", "cannot read source: "+err.Error()))
			continue
		}
		output, diagnostics := compileWithMapping(sourcePath, source, moduleRelativeSourcePath(sourcePath))
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
		if output.Code != nil {
			output.sourceMode = uint32(info.Mode().Perm())
			compiled = append(compiled, output)
			result.Files = append(result.Files, FileResult{SourcePath: output.SourcePath, OutputPath: output.OutputPath})
		}
	}
	result.Diagnostics = append(result.Diagnostics, detectComponentCycles(compiled)...)
	sort.SliceStable(compiled, func(i, j int) bool { return compiled[i].SourcePath < compiled[j].SourcePath })
	sort.SliceStable(result.Files, func(i, j int) bool { return result.Files[i].SourcePath < result.Files[j].SourcePath })
	sortDiagnostics(result.Diagnostics)
	return compiled, result
}

func moduleRelativeSourcePath(sourcePath string) string {
	absolute, err := filepath.Abs(sourcePath)
	if err != nil {
		return filepath.ToSlash(filepath.Base(sourcePath))
	}
	directory := filepath.Dir(absolute)
	for {
		modulePath := filepath.Join(directory, "go.mod")
		if info, statErr := os.Lstat(modulePath); statErr == nil && info.Mode().IsRegular() {
			if relative, relativeErr := filepath.Rel(directory, absolute); relativeErr == nil {
				return filepath.ToSlash(relative)
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return filepath.ToSlash(filepath.Base(sourcePath))
}

func atomicWrite(path string, content []byte, mode os.FileMode) (returnErr error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".himesan-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	closed := false
	defer func() {
		var closeErr error
		if !closed {
			closeErr = temporary.Close()
		}
		removeErr := os.Remove(temporaryPath)
		if returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
		if returnErr == nil && removeErr != nil && !os.IsNotExist(removeErr) {
			returnErr = removeErr
		}
	}()
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Chmod(mode.Perm()); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	closed = true
	if err := replaceFile(temporaryPath, path); err != nil {
		return err
	}
	if directoryHandle, err := os.Open(directory); err == nil {
		_ = directoryHandle.Sync()
		_ = directoryHandle.Close()
	}
	return nil
}

type componentKey struct {
	directory string
	packageID string
	name      string
}

type componentEdge struct {
	target   componentKey
	position sourcePosition
}

func detectComponentCycles(files []CompiledFile) []Diagnostic {
	byKey := make(map[componentKey]CompiledFile, len(files))
	var diagnostics []Diagnostic
	for _, file := range files {
		key := componentKey{directory: filepath.Clean(filepath.Dir(file.SourcePath)), packageID: file.Package, name: file.Component}
		if previous, exists := byKey[key]; exists {
			diagnostics = append(diagnostics,
				diagnostic(previous.SourcePath, sourcePosition{Line: 1, Column: 1}, "HIM1500", fmt.Sprintf("component %s is also declared by %s", file.Component, file.SourcePath)),
				diagnostic(file.SourcePath, sourcePosition{Line: 1, Column: 1}, "HIM1500", fmt.Sprintf("component %s is also declared by %s", file.Component, previous.SourcePath)),
			)
			continue
		}
		byKey[key] = file
	}
	edges := make(map[componentKey][]componentEdge)
	for key, file := range byKey {
		if file.source == nil {
			continue
		}
		for _, node := range file.source.Nodes {
			if node.Kind != nodeComponent {
				continue
			}
			expression, err := parser.ParseExpr(node.Text)
			if err != nil {
				continue
			}
			calledName := rootCalledIdentifier(expression)
			if calledName == "" {
				continue
			}
			target := componentKey{directory: key.directory, packageID: key.packageID, name: calledName}
			if _, exists := byKey[target]; exists {
				edges[key] = append(edges[key], componentEdge{target: target, position: node.Pos})
			}
		}
		sort.SliceStable(edges[key], func(i, j int) bool { return edges[key][i].target.name < edges[key][j].target.name })
	}

	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[componentKey]int)
	stack := make([]componentKey, 0)
	reported := make(map[componentKey]bool)
	var visit func(componentKey)
	visit = func(key componentKey) {
		state[key] = visiting
		stack = append(stack, key)
		for _, edge := range edges[key] {
			target := edge.target
			if state[target] == unvisited {
				visit(target)
				continue
			}
			if state[target] != visiting {
				continue
			}
			cycleStart := 0
			for cycleStart < len(stack) && stack[cycleStart] != target {
				cycleStart++
			}
			cycle := append(append([]componentKey(nil), stack[cycleStart:]...), target)
			names := make([]string, 0, len(cycle))
			for _, member := range cycle {
				names = append(names, member.name)
			}
			for memberIndex, member := range cycle[:len(cycle)-1] {
				if reported[member] {
					continue
				}
				reported[member] = true
				file := byKey[member]
				position := sourcePosition{Line: 1, Column: 1}
				next := cycle[memberIndex+1]
				for _, memberEdge := range edges[member] {
					if memberEdge.target == next {
						position = memberEdge.position
						break
					}
				}
				diagnostics = append(diagnostics, diagnostic(file.SourcePath, position, "HIM1501", "static component cycle detected: "+fmt.Sprint(names)))
			}
		}
		stack = stack[:len(stack)-1]
		state[key] = visited
	}
	keys := make([]componentKey, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].directory != keys[j].directory {
			return keys[i].directory < keys[j].directory
		}
		if keys[i].packageID != keys[j].packageID {
			return keys[i].packageID < keys[j].packageID
		}
		return keys[i].name < keys[j].name
	})
	for _, key := range keys {
		if state[key] == unvisited {
			visit(key)
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

func rootCalledIdentifier(expression ast.Expr) string {
	for {
		switch typed := expression.(type) {
		case *ast.ParenExpr:
			expression = typed.X
		case *ast.CallExpr:
			expression = typed.Fun
		case *ast.IndexExpr:
			expression = typed.X
		case *ast.IndexListExpr:
			expression = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}
