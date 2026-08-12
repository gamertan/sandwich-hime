// SPDX-License-Identifier: AGPL-3.0-only

// Package compiler implements the Hime-san .sando compiler.
package compiler

import (
	"fmt"
	"sort"
	"strings"
)

// Severity describes the impact of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is a stable, machine-readable compiler message. Line and Column
// are one-based. A diagnostic without a source position uses line and column 1.
type Diagnostic struct {
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Column   int      `json:"column"`
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

func (d Diagnostic) Error() string {
	line, column := d.Line, d.Column
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", d.Path, line, column, d.Code, d.Message)
}

// DiagnosticsError reports one or more error diagnostics.
type DiagnosticsError struct {
	Diagnostics []Diagnostic
}

func (e *DiagnosticsError) Error() string {
	if e == nil || len(e.Diagnostics) == 0 {
		return "himesan: compilation failed"
	}
	if len(e.Diagnostics) == 1 {
		return e.Diagnostics[0].Error()
	}
	return fmt.Sprintf("%s (and %d more diagnostics)", e.Diagnostics[0].Error(), len(e.Diagnostics)-1)
}

func errorFromDiagnostics(ds []Diagnostic) error {
	if !hasErrors(ds) {
		return nil
	}
	copyOfDiagnostics := append([]Diagnostic(nil), ds...)
	sortDiagnostics(copyOfDiagnostics)
	return &DiagnosticsError{Diagnostics: copyOfDiagnostics}
}

func hasErrors(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == SeverityError || d.Severity == "" {
			return true
		}
	}
	return false
}

func sortDiagnostics(ds []Diagnostic) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
}

func diagnostic(path string, pos sourcePosition, code, message string) Diagnostic {
	return Diagnostic{
		Path:     path,
		Line:     pos.Line,
		Column:   pos.Column,
		Code:     code,
		Severity: SeverityError,
		Message:  strings.TrimSpace(message),
	}
}
