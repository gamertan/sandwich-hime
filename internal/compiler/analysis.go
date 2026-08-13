// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
)

// SourceInput supplies one in-memory .sando document for editor analysis.
type SourceInput struct {
	Path   string
	Source []byte
}

// AnalysisImport describes one import already present in a .sando header.
type AnalysisImport struct {
	Alias string `json:"alias,omitempty"`
	Path  string `json:"path"`
}

// AnalysisRegionKind identifies an author-visible template region.
type AnalysisRegionKind string

const (
	AnalysisStatement  AnalysisRegionKind = "statement"
	AnalysisExpression AnalysisRegionKind = "expression"
	AnalysisComponent  AnalysisRegionKind = "component"
	AnalysisComment    AnalysisRegionKind = "comment"
)

// AnalysisRegion describes a Hime-san tag body using zero-based byte offsets.
// Line and Column retain the compiler's one-based byte-coordinate convention;
// protocol adapters convert them to UTF-16 where required.
type AnalysisRegion struct {
	Kind    AnalysisRegionKind `json:"kind"`
	Text    string             `json:"text,omitempty"`
	Context Context            `json:"context,omitempty"`
	Offset  int                `json:"offset"`
	Length  int                `json:"length"`
	Line    int                `json:"line"`
	Column  int                `json:"column"`
}

// DocumentAnalysis is the compiler-owned semantic description consumed by
// read-only tools such as the language server.
type DocumentAnalysis struct {
	Path            string           `json:"path"`
	Package         string           `json:"package,omitempty"`
	Component       string           `json:"component,omitempty"`
	TypeParams      string           `json:"type_params,omitempty"`
	Params          string           `json:"params,omitempty"`
	Signature       string           `json:"signature,omitempty"`
	ComponentOffset int              `json:"component_offset,omitempty"`
	ComponentLine   int              `json:"component_line,omitempty"`
	ComponentColumn int              `json:"component_column,omitempty"`
	Imports         []AnalysisImport `json:"imports,omitempty"`
	Regions         []AnalysisRegion `json:"regions,omitempty"`
	Diagnostics     []Diagnostic     `json:"diagnostics,omitempty"`
}

// AnalyzeSources applies the normal parser, HTML-context analyzer, trust
// audit, backend validation, duplicate-component checks, and statically
// knowable cycle checks to an in-memory source set. It performs no I/O.
func AnalyzeSources(ctx context.Context, inputs []SourceInput) []DocumentAnalysis {
	ordered := append([]SourceInput(nil), inputs...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return filepath.Clean(ordered[i].Path) < filepath.Clean(ordered[j].Path)
	})
	analyses := make([]DocumentAnalysis, 0, len(ordered))
	compiled := make([]CompiledFile, 0, len(ordered))
	for _, input := range ordered {
		if err := ctx.Err(); err != nil {
			analyses = append(analyses, DocumentAnalysis{
				Path:        filepath.Clean(input.Path),
				Diagnostics: []Diagnostic{diagnostic(input.Path, sourcePosition{Line: 1, Column: 1}, "HIM2001", "operation canceled: "+err.Error())},
			})
			continue
		}
		analysis, output := analyzeSource(input.Path, input.Source)
		analyses = append(analyses, analysis)
		if output.Code != nil {
			compiled = append(compiled, output)
		}
	}
	byPath := make(map[string][]Diagnostic)
	for _, item := range detectComponentCycles(compiled) {
		path := filepath.Clean(item.Path)
		byPath[path] = append(byPath[path], item)
	}
	for index := range analyses {
		path := filepath.Clean(analyses[index].Path)
		analyses[index].Diagnostics = append(analyses[index].Diagnostics, byPath[path]...)
		sortDiagnostics(analyses[index].Diagnostics)
	}
	return analyses
}

func analyzeSource(path string, source []byte) (DocumentAnalysis, CompiledFile) {
	cleanPath := filepath.Clean(path)
	analysis := DocumentAnalysis{Path: cleanPath}
	file, diagnostics := parseSource(cleanPath, source)
	if file == nil {
		sortDiagnostics(diagnostics)
		analysis.Diagnostics = diagnostics
		return analysis, CompiledFile{}
	}
	analysis.Package = file.Package
	analysis.Component = file.Name
	analysis.TypeParams = file.TypeParams
	analysis.Params = file.Params
	analysis.Signature = fmt.Sprintf("func %s%s%s", file.Name, file.TypeParams, file.Params)
	analysis.ComponentOffset = file.FunctionPos.Offset
	analysis.ComponentLine = file.FunctionPos.Line
	analysis.ComponentColumn = file.FunctionPos.Column
	for _, imported := range file.Imports {
		analysis.Imports = append(analysis.Imports, AnalysisImport{Alias: imported.Alias, Path: imported.Path})
	}
	diagnostics = append(diagnostics, analyzeContexts(file)...)
	diagnostics = append(diagnostics, auditTrustCalls(file)...)
	for _, node := range file.Nodes {
		kind := AnalysisRegionKind("")
		switch node.Kind {
		case nodeStatement:
			kind = AnalysisStatement
		case nodeExpression:
			kind = AnalysisExpression
		case nodeComponent:
			kind = AnalysisComponent
		case nodeComment:
			kind = AnalysisComment
		default:
			continue
		}
		analysis.Regions = append(analysis.Regions, AnalysisRegion{
			Kind: kind, Text: node.Text, Context: node.Context,
			Offset: node.Pos.Offset, Length: len(node.Text),
			Line: node.Pos.Line, Column: node.Pos.Column,
		})
	}
	if hasErrors(diagnostics) {
		sortDiagnostics(diagnostics)
		analysis.Diagnostics = diagnostics
		return analysis, CompiledFile{}
	}
	code, backendDiagnostics := generateGo(file)
	diagnostics = append(diagnostics, backendDiagnostics...)
	sortDiagnostics(diagnostics)
	analysis.Diagnostics = diagnostics
	if hasErrors(diagnostics) {
		return analysis, CompiledFile{}
	}
	return analysis, CompiledFile{
		SourcePath: cleanPath,
		OutputPath: cleanPath + ".go",
		Package:    file.Package,
		Component:  file.Name,
		Code:       code,
		source:     file,
	}
}
