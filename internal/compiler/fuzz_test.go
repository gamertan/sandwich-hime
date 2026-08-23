// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

func FuzzCompileNeverPanics(f *testing.F) {
	for _, seed := range []string{
		profileSource,
		"",
		"<?sando go\npackage p\nfunc F()\n?>",
		"<?sando go\npackage p\nfunc F(v string)\n?>\n<a href=\"<?= v ?>\">x</a>",
		"<?sando go\npackage p\nfunc F()\n?>\n<script><?= `?>` ?></script>",
		"<?sando go\npackage p\nfunc F(v string)\n?>\n<script><!--<script></script>\n<?= v ?>\n<!--\n</script>\n-->",
		"\xef\xbb\xbf\r\n<?sando go\r\npackage p\r\nfunc F()\r\n?>\r\n<p>x</p>",
	} {
		f.Add(seed, "fuzz.sando")
	}
	f.Add("<?sando go\npackage p\nfunc F()\n?>\n<p>x</p>", "path%with\ncontrols\x00.sando")
	f.Fuzz(func(t *testing.T, source, mapping string) {
		if len(source) > 64<<10 || len(mapping) > 4<<10 {
			t.Skip()
		}
		input := []byte(source)
		before := append([]byte(nil), input...)
		first, firstDiagnostics := compileWithMapping("fuzz.sando", input, mapping)
		second, secondDiagnostics := compileWithMapping("fuzz.sando", input, mapping)
		if !bytes.Equal(input, before) {
			t.Fatal("compiler modified its source input")
		}
		if !reflect.DeepEqual(firstDiagnostics, secondDiagnostics) ||
			first.SourcePath != second.SourcePath || first.OutputPath != second.OutputPath ||
			first.Package != second.Package || first.Component != second.Component ||
			first.Digest != second.Digest || !bytes.Equal(first.Code, second.Code) {
			t.Fatal("repeated in-memory compilation was not deterministic")
		}
		for _, diagnostic := range firstDiagnostics {
			if diagnostic.Path != "fuzz.sando" || diagnostic.Line < 1 || diagnostic.Column < 1 {
				t.Fatalf("diagnostic has an invalid location: %#v", diagnostic)
			}
			if !validDiagnosticCode(diagnostic.Code) || strings.TrimSpace(diagnostic.Message) != diagnostic.Message || diagnostic.Message == "" {
				t.Fatalf("diagnostic violates the public shape: %#v", diagnostic)
			}
			if diagnostic.Severity != SeverityError && diagnostic.Severity != SeverityWarning {
				t.Fatalf("diagnostic has an invalid severity: %#v", diagnostic)
			}
		}
		if len(first.Code) == 0 {
			return
		}
		if first.SourcePath != "fuzz.sando" || first.OutputPath != "fuzz.sando.go" {
			t.Fatalf("compiled paths are invalid: %#v", first)
		}
		if first.Digest != fmt.Sprintf("%x", sha256.Sum256(input)) {
			t.Fatalf("source digest is not bound to the exact input: %s", first.Digest)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "fuzz.sando.go", first.Code, parser.AllErrors); err != nil {
			t.Fatalf("successful compilation produced invalid Go: %v\n%s", err, first.Code)
		}
		for _, line := range bytes.Split(first.Code, []byte{'\n'}) {
			if bytes.HasPrefix(line, []byte("//line ")) && (bytes.ContainsAny(line, "\r\x00") || bytes.Count(line, []byte(":")) < 2) {
				t.Fatalf("source-map directive was not safely encoded: %q", line)
			}
		}
	})
}

func validDiagnosticCode(code string) bool {
	if len(code) != 7 || !strings.HasPrefix(code, "HIM") {
		return false
	}
	for _, digit := range code[3:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func FuzzGoDelimiterNeverPanics(f *testing.F) {
	for _, seed := range []string{`?>`, `"?>" ?>`, "`?>` ?>", `/* ?> */ ?>`, "// ?>\n?>", `'?' ?>`} {
		f.Add(seed, uint8(0))
	}
	f.Fuzz(func(t *testing.T, source string, start uint8) {
		offset := int(start)
		if offset > len(source) {
			offset = len(source)
		}
		result := findGoDelimiter([]byte(source), offset)
		if result < -1 || result > len(source) {
			t.Fatalf("invalid delimiter offset %d for %d bytes", result, len(source))
		}
	})
}
