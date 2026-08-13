// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeSourcesUsesCompilerSemanticsWithoutIO(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	firstPath := filepath.Join(directory, "first.sando")
	secondPath := filepath.Join(directory, "second.sando")
	first := []byte("<?sando go\npackage views\nfunc First(name string)\n?>\n<div><?= name ?><?~ Second() ?></div>\n")
	second := []byte("<?sando go\npackage views\nfunc Second()\n?>\n<section><?~ First(\"again\") ?></section>\n")
	analyses := AnalyzeSources(context.Background(), []SourceInput{{Path: firstPath, Source: first}, {Path: secondPath, Source: second}})
	if len(analyses) != 2 {
		t.Fatalf("analysis count = %d, want 2", len(analyses))
	}
	for _, analysis := range analyses {
		if analysis.Component == "" || analysis.Signature == "" || analysis.ComponentLine < 1 {
			t.Fatalf("missing component metadata: %#v", analysis)
		}
		if !hasDiagnosticCode(analysis.Diagnostics, "HIM1501") {
			t.Fatalf("cycle diagnostic missing for %s: %#v", analysis.Component, analysis.Diagnostics)
		}
		if _, err := os.Stat(analysis.Path + ".go"); !os.IsNotExist(err) {
			t.Fatalf("analysis wrote generated output: %v", err)
		}
	}
	if analyses[0].Regions[0].Context != ContextHTMLText {
		t.Fatalf("expression context = %q, want %q", analyses[0].Regions[0].Context, ContextHTMLText)
	}
}

func TestAnalyzeSourcesDuplicateAndMalformedDocuments(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	duplicate := "<?sando go\npackage views\nfunc Card()\n?>\n<p>card</p>\n"
	malformed := "<?sando go\npackage views\nfunc Broken()\n?>\n<div>\x00"
	analyses := AnalyzeSources(context.Background(), []SourceInput{
		{Path: filepath.Join(directory, "a.sando"), Source: []byte(duplicate)},
		{Path: filepath.Join(directory, "b.sando"), Source: []byte(duplicate)},
		{Path: filepath.Join(directory, "broken.sando"), Source: []byte(malformed)},
	})
	if !hasDiagnosticCode(analyses[0].Diagnostics, "HIM1500") || !hasDiagnosticCode(analyses[1].Diagnostics, "HIM1500") {
		t.Fatalf("duplicate diagnostics missing: %#v", analyses)
	}
	if !hasDiagnosticCode(analyses[2].Diagnostics, "HIM1002") {
		t.Fatalf("NUL diagnostic missing: %#v", analyses[2].Diagnostics)
	}
}

func TestDiscoverSourcesOmitsGeneratedFreshnessButKeepsBoundaries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/root\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "orphan.sando.go"), []byte(generatedPrefix+"\npackage root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example.test/nested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "hidden.sando"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, diagnostics := DiscoverSources(context.Background(), []string{root})
	if len(paths) != 0 {
		t.Fatalf("discovered nested source: %v", paths)
	}
	for _, item := range diagnostics {
		if item.Code == "HIM2014" {
			t.Fatalf("editor discovery reported generated freshness: %#v", diagnostics)
		}
	}
}

func hasDiagnosticCode(diagnostics []Diagnostic, code string) bool {
	for _, item := range diagnostics {
		if item.Code == code || strings.HasPrefix(item.Code, code) {
			return true
		}
	}
	return false
}
