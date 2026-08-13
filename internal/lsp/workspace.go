// SPDX-License-Identifier: AGPL-3.0-only

package lsp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"gamertan.com/sandwich-hime/internal/compiler"
)

const (
	maxDocumentBytes  = 16 << 20
	maxWorkspaceBytes = 64 << 20
	maxWorkspaceFiles = 10000
)

type document struct {
	URI     string
	Path    string
	Text    []byte
	Version int
	Open    bool
}

type workspaceSnapshot struct {
	documents  map[string]document
	analyses   map[string]compiler.DocumentAnalysis
	moduleRoot string
	modulePath string
}

func (server *Server) reindex(ctx context.Context) error {
	server.mu.RLock()
	root := server.root
	overlays := make(map[string]document, len(server.overlays))
	for path, item := range server.overlays {
		overlays[path] = item
	}
	server.mu.RUnlock()
	if root == "" {
		return nil
	}

	paths, discoveryDiagnostics := compiler.DiscoverSources(ctx, []string{root})
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(paths) > maxWorkspaceFiles {
		return fmt.Errorf("workspace contains more than %d .sando files", maxWorkspaceFiles)
	}
	documents := make(map[string]document, len(paths)+len(overlays))
	total := 0
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		absolute = filepath.Clean(absolute)
		if overlay, ok := overlays[absolute]; ok {
			documents[absolute] = overlay
			total += len(overlay.Text)
			continue
		}
		info, err := os.Lstat(absolute)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxDocumentBytes {
			continue
		}
		content, err := os.ReadFile(absolute)
		if err != nil || len(content) > maxDocumentBytes {
			continue
		}
		documents[absolute] = document{URI: pathToURI(absolute), Path: absolute, Text: content}
		total += len(content)
		if total > maxWorkspaceBytes {
			return fmt.Errorf("workspace .sando sources exceed %d bytes", maxWorkspaceBytes)
		}
	}
	for path, overlay := range overlays {
		if _, ok := documents[path]; ok {
			continue
		}
		if overlay.Open && editorPathAllowed(root, path) {
			documents[path] = overlay
			total += len(overlay.Text)
		}
	}
	if len(documents) > maxWorkspaceFiles {
		return fmt.Errorf("workspace contains more than %d .sando files", maxWorkspaceFiles)
	}
	if total > maxWorkspaceBytes {
		return fmt.Errorf("workspace .sando sources exceed %d bytes", maxWorkspaceBytes)
	}
	inputs := make([]compiler.SourceInput, 0, len(documents))
	for _, item := range documents {
		inputs = append(inputs, compiler.SourceInput{Path: item.Path, Source: item.Text})
	}
	analysed := compiler.AnalyzeSources(ctx, inputs)
	if err := ctx.Err(); err != nil {
		return err
	}
	analyses := make(map[string]compiler.DocumentAnalysis, len(analysed))
	for _, item := range analysed {
		analyses[filepath.Clean(item.Path)] = item
	}
	moduleRoot, modulePath := moduleIdentity(root)

	server.mu.Lock()
	previous := server.snapshot.documents
	server.snapshot = workspaceSnapshot{documents: documents, analyses: analyses, moduleRoot: moduleRoot, modulePath: modulePath}
	server.mu.Unlock()

	all := make(map[string]bool, len(previous)+len(documents))
	for path := range previous {
		all[path] = true
	}
	for path := range documents {
		all[path] = true
	}
	ordered := make([]string, 0, len(all))
	for path := range all {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		item, exists := documents[path]
		uri := pathToURI(path)
		diagnostics := make([]lspDiagnostic, 0)
		if exists {
			uri = item.URI
			for _, diagnostic := range analyses[path].Diagnostics {
				diagnostics = append(diagnostics, diagnosticToLSP(item.Text, diagnostic))
			}
		}
		if err := server.notify("textDocument/publishDiagnostics", struct {
			URI         string          `json:"uri"`
			Diagnostics []lspDiagnostic `json:"diagnostics"`
		}{URI: uri, Diagnostics: diagnostics}); err != nil {
			return err
		}
	}
	if len(discoveryDiagnostics) != 0 {
		server.log("workspace discovery reported boundary diagnostics", len(discoveryDiagnostics))
	}
	server.log("analysis completed", len(documents))
	return nil
}

func (server *Server) scheduleReindex(delay bool) {
	server.mu.Lock()
	if server.analysisCancel != nil {
		server.analysisCancel()
	}
	if server.analysisTimer != nil {
		server.analysisTimer.Stop()
	}
	generation := server.analysisGeneration + 1
	server.analysisGeneration = generation
	wait := server.debounce
	if !delay {
		wait = 0
	}
	server.analysisTimer = server.afterFunc(wait, func() {
		ctx, cancel := context.WithCancel(server.context)
		server.mu.Lock()
		if server.analysisGeneration != generation {
			server.mu.Unlock()
			cancel()
			return
		}
		server.analysisWait.Add(1)
		server.analysisCancel = cancel
		server.mu.Unlock()
		defer server.analysisWait.Done()
		err := server.reindex(ctx)
		cancel()
		server.mu.Lock()
		if server.analysisGeneration == generation {
			server.analysisCancel = nil
		}
		server.mu.Unlock()
		if err != nil && !errors.Is(err, context.Canceled) {
			server.log("analysis failed", 1)
		}
	})
	server.mu.Unlock()
}

func (server *Server) snapshotDocument(uri string) (document, compiler.DocumentAnalysis, bool) {
	path, err := fileURIToPath(uri)
	if err != nil {
		return document{}, compiler.DocumentAnalysis{}, false
	}
	server.mu.RLock()
	defer server.mu.RUnlock()
	item, ok := server.snapshot.documents[path]
	if !ok {
		return document{}, compiler.DocumentAnalysis{}, false
	}
	return item, server.snapshot.analyses[path], true
}

func moduleIdentity(root string) (string, string) {
	candidate := filepath.Join(root, "go.mod")
	content, err := os.ReadFile(candidate)
	if err != nil || len(content) > 1<<20 {
		return "", ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[0] == "module" {
			return filepath.Clean(root), fields[1]
		}
	}
	return "", ""
}

func (snapshot workspaceSnapshot) packageImportPath(directory string) string {
	if snapshot.moduleRoot == "" || snapshot.modulePath == "" || !withinRoot(snapshot.moduleRoot, directory) {
		return ""
	}
	relative, err := filepath.Rel(snapshot.moduleRoot, directory)
	if err != nil || relative == "." {
		return snapshot.modulePath
	}
	return strings.TrimSuffix(snapshot.modulePath, "/") + "/" + filepath.ToSlash(relative)
}

func (snapshot workspaceSnapshot) componentsFor(path string, analysis compiler.DocumentAnalysis) []componentTarget {
	directory := filepath.Dir(path)
	var targets []componentTarget
	for targetPath, targetAnalysis := range snapshot.analyses {
		if targetAnalysis.Component == "" {
			continue
		}
		targetDirectory := filepath.Dir(targetPath)
		if targetDirectory == directory && targetAnalysis.Package == analysis.Package {
			targets = append(targets, componentTarget{Name: targetAnalysis.Component, Signature: targetAnalysis.Signature, Path: targetPath, Analysis: targetAnalysis})
			continue
		}
		importPath := snapshot.packageImportPath(targetDirectory)
		for _, imported := range analysis.Imports {
			if imported.Path != importPath || imported.Alias == "_" || imported.Alias == "." {
				continue
			}
			alias := imported.Alias
			if alias == "" {
				alias = targetAnalysis.Package
			}
			targets = append(targets, componentTarget{Qualifier: alias, Name: targetAnalysis.Component, Signature: targetAnalysis.Signature, Path: targetPath, Analysis: targetAnalysis})
		}
	}
	sort.SliceStable(targets, func(i, j int) bool {
		return targets[i].Label() < targets[j].Label()
	})
	return targets
}

type componentTarget struct {
	Qualifier string
	Name      string
	Signature string
	Path      string
	Analysis  compiler.DocumentAnalysis
}

func (target componentTarget) Label() string {
	if target.Qualifier == "" {
		return target.Name
	}
	return target.Qualifier + "." + target.Name
}

func withinRoot(root, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func editorPathAllowed(root, path string) bool {
	if filepath.Ext(path) != ".sando" || !withinRoot(root, path) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	current := filepath.Clean(root)
	parts := strings.Split(relative, string(filepath.Separator))
	for index, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if index < len(parts)-1 && (part == ".git" || part == ".hg" || part == ".svn" || part == "vendor") {
			return false
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return false
		}
		if index < len(parts)-1 && current != filepath.Clean(root) {
			moduleInfo, moduleErr := os.Lstat(filepath.Join(current, "go.mod"))
			if moduleErr == nil || moduleInfo != nil {
				return false
			}
			if moduleErr != nil && !os.IsNotExist(moduleErr) {
				return false
			}
		}
	}
	return true
}

func fileURIToPath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "file" || (parsed.Host != "" && parsed.Host != "localhost") {
		return "", errors.New("only local file URIs are supported")
	}
	path, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	absolute, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func pathToURI(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = filepath.Clean(path)
	}
	slashed := filepath.ToSlash(absolute)
	if runtime.GOOS == "windows" && !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return (&url.URL{Scheme: "file", Path: slashed}).String()
}

func offsetToPosition(text []byte, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	line, character := 0, 0
	for index := 0; index < offset; {
		if text[index] == '\n' {
			line++
			character = 0
			index++
			continue
		}
		r, size := utf8.DecodeRune(text[index:])
		if r == utf8.RuneError && size == 1 {
			character++
			index++
			continue
		}
		character += len(utf16.Encode([]rune{r}))
		index += size
	}
	return Position{Line: line, Character: character}
}

func positionToOffset(text []byte, position Position) (int, bool) {
	if position.Line < 0 || position.Character < 0 {
		return 0, false
	}
	line := 0
	start := 0
	for start < len(text) && line < position.Line {
		if text[start] == '\n' {
			line++
		}
		start++
	}
	if line != position.Line {
		return 0, false
	}
	units := 0
	for index := start; index < len(text) && text[index] != '\n'; {
		if units == position.Character {
			return index, true
		}
		r, size := utf8.DecodeRune(text[index:])
		if r == utf8.RuneError && size == 1 {
			units++
			index++
		} else {
			units += len(utf16.Encode([]rune{r}))
			index += size
		}
		if units > position.Character {
			return 0, false
		}
	}
	if units == position.Character {
		index := start
		for index < len(text) && text[index] != '\n' {
			index++
		}
		return index, true
	}
	return 0, false
}

func compilerPositionOffset(text []byte, line, column int) int {
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	start := 0
	for current := 1; current < line && start < len(text); current++ {
		newline := strings.IndexByte(string(text[start:]), '\n')
		if newline < 0 {
			return len(text)
		}
		start += newline + 1
	}
	offset := start + column - 1
	if offset > len(text) {
		offset = len(text)
	}
	return offset
}

func diagnosticToLSP(text []byte, diagnostic compiler.Diagnostic) lspDiagnostic {
	startOffset := compilerPositionOffset(text, diagnostic.Line, diagnostic.Column)
	endOffset := startOffset
	if endOffset < len(text) {
		_, size := utf8.DecodeRune(text[endOffset:])
		if size < 1 {
			size = 1
		}
		endOffset += size
	}
	severity := 1
	if diagnostic.Severity == compiler.SeverityWarning {
		severity = 2
	}
	return lspDiagnostic{
		Range:    Range{Start: offsetToPosition(text, startOffset), End: offsetToPosition(text, endOffset)},
		Severity: severity, Code: diagnostic.Code, Source: "himesan", Message: diagnostic.Message,
	}
}
