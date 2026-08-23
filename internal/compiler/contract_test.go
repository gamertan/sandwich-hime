// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var diagnosticCodePattern = regexp.MustCompile(`^HIM[0-9]{4}$`)

func TestV1DiagnosticCodeContract(t *testing.T) {
	t.Parallel()
	directory := packageDirectory(t)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(map[string]struct{})
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, filepath.Join(directory, entry.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err == nil && diagnosticCodePattern.MatchString(value) {
				codes[value] = struct{}{}
			}
			return true
		})
	}
	actual := make([]string, 0, len(codes))
	for code := range codes {
		actual = append(actual, code)
	}
	sort.Strings(actual)
	assertContractFile(t, filepath.Join(directory, "..", "..", "contracts", "diagnostic-codes-v1.txt"), strings.Join(actual, "\n")+"\n")
}

func TestV1GeneratedProvenanceContract(t *testing.T) {
	t.Parallel()
	compiled, diagnostics := Compile("views/generic.sando", []byte(`<?sando go
package views
func List[T ~string](values []T)
?>
<ul><? for _, value := range values { ?><li><?= value ?></li><? } ?></ul>`))
	assertNoErrorDiagnostics(t, diagnostics)
	lines := strings.Split(string(compiled.Code), "\n")
	if len(lines) < 4 {
		t.Fatalf("generated header has %d lines", len(lines))
	}
	actual := []string{lines[0]}
	if !strings.HasPrefix(lines[1], "// himesan:compiler ") {
		t.Fatalf("compiler provenance line = %q", lines[1])
	}
	actual = append(actual, "// himesan:compiler <compiler-version>")
	if !strings.HasPrefix(lines[2], "// himesan:runtime-abi ") {
		t.Fatalf("runtime provenance line = %q", lines[2])
	}
	actual = append(actual, "// himesan:runtime-abi <runtime-abi>")
	if !regexp.MustCompile(`^// himesan:source-sha256 [0-9a-f]{64}$`).MatchString(lines[3]) {
		t.Fatalf("source provenance line = %q", lines[3])
	}
	actual = append(actual, "// himesan:source-sha256 <lowercase-sha256>")
	generated := string(compiled.Code)
	if !regexp.MustCompile(`(?m)^var _ = [A-Za-z_][A-Za-z0-9_]*\.ABISandoV1$`).MatchString(generated) {
		t.Fatal("generated output is missing the compile-time ABI marker")
	}
	actual = append(actual, "var _ = <sando-import>.ABISandoV1")
	if !regexp.MustCompile(`(?m)^//line [^\r\n]+:[1-9][0-9]*:[1-9][0-9]*$`).MatchString(generated) {
		t.Fatal("generated output is missing source mappings")
	}
	actual = append(actual, "//line <source-path>:<line>:<column>")
	assertContractFile(t, filepath.Join(packageDirectory(t), "..", "..", "contracts", "generated-provenance-v1.txt"), strings.Join(actual, "\n")+"\n")
}

func TestV1GenericComponentSignature(t *testing.T) {
	t.Parallel()
	source := []byte(`<?sando go
package views
func List[T ~string](values []T)
?>
<ul><? for _, value := range values { ?><li><?= value ?></li><? } ?></ul>`)
	compiled, diagnostics := Compile("views/list.sando", source)
	assertNoErrorDiagnostics(t, diagnostics)
	if !bytes.Contains(compiled.Code, []byte("func List[T ~string](values []T)")) {
		t.Fatalf("generic signature was not preserved:\n%s", compiled.Code)
	}
	analyses := AnalyzeSources(context.Background(), []SourceInput{{Path: "views/list.sando", Source: source}})
	if len(analyses) != 1 {
		t.Fatalf("analysis count = %d, want 1", len(analyses))
	}
	analysis := analyses[0]
	if analysis.TypeParams != "[T ~string]" || analysis.Params != "(values []T)" || analysis.Signature != "func List[T ~string](values []T)" {
		t.Fatalf("generic analysis contract = %#v", analysis)
	}
}

func packageDirectory(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func assertContractFile(t *testing.T, path, actual string) {
	t.Helper()
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	expected = bytes.TrimPrefix(expected, []byte("# SPDX-License-Identifier: AGPL-3.0-only\n\n"))
	if string(expected) != actual {
		t.Fatalf("contract drift in %s\n--- expected ---\n%s--- actual ---\n%s", path, expected, actual)
	}
}
