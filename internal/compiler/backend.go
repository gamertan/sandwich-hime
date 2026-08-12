// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gamertan.com/sandwich-hime/internal/version"
)

type backendImports struct {
	Context string
	IO      string
	Sando   string
	All     []sourceImport
}

// Compile parses, context-checks, and formats one .sando source entirely in
// memory. It never reads project metadata, writes a file, or executes Go code.
func Compile(path string, source []byte) (CompiledFile, []Diagnostic) {
	return compileWithMapping(path, source, filepath.ToSlash(filepath.Base(path)))
}

func compileWithMapping(path string, source []byte, mapping string) (CompiledFile, []Diagnostic) {
	cleanPath := filepath.Clean(path)
	file, diagnostics := parseSource(cleanPath, source)
	if file == nil {
		sortDiagnostics(diagnostics)
		return CompiledFile{}, diagnostics
	}
	file.Mapping = mapping
	diagnostics = append(diagnostics, analyzeContexts(file)...)
	diagnostics = append(diagnostics, auditTrustCalls(file)...)
	if hasErrors(diagnostics) {
		sortDiagnostics(diagnostics)
		return CompiledFile{}, diagnostics
	}

	code, backendDiagnostics := generateGo(file)
	diagnostics = append(diagnostics, backendDiagnostics...)
	if hasErrors(diagnostics) {
		sortDiagnostics(diagnostics)
		return CompiledFile{}, diagnostics
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(source))
	compiled := CompiledFile{
		SourcePath: cleanPath,
		OutputPath: cleanPath + ".go",
		Package:    file.Package,
		Component:  file.Name,
		Digest:     digest,
		Code:       code,
		source:     file,
	}
	sortDiagnostics(diagnostics)
	return compiled, diagnostics
}

func generateGo(file *sourceFile) ([]byte, []Diagnostic) {
	imports, diagnostics := prepareImports(file)
	if hasErrors(diagnostics) {
		return nil, diagnostics
	}
	usedIdentifiers := sourceIdentifiers(file.Source)
	contextName := uniqueIdentifier("__himesan_render_context", usedIdentifiers)
	writerName := uniqueIdentifier("__himesan_writer", usedIdentifiers)
	errorName := uniqueIdentifier("__himesan_error", usedIdentifiers)

	digest := fmt.Sprintf("%x", sha256.Sum256(file.Source))
	var output strings.Builder
	output.WriteString(generatedPrefix)
	output.WriteByte('\n')
	fmt.Fprintf(&output, "// himesan:compiler %s\n", version.Compiler)
	fmt.Fprintf(&output, "// himesan:runtime-abi %s\n", version.RuntimeABI)
	fmt.Fprintf(&output, "// himesan:source-sha256 %s\n\n", digest)
	fmt.Fprintf(&output, "package %s\n\n", file.Package)
	output.WriteString("import (\n")
	for _, imported := range imports.All {
		if imported.Alias != "" {
			fmt.Fprintf(&output, "\t%s %q\n", imported.Alias, imported.Path)
		} else {
			fmt.Fprintf(&output, "\t%q\n", imported.Path)
		}
	}
	output.WriteString(")\n\n")
	fmt.Fprintf(&output, "var _ = %s.ABI\n\n", imports.Sando)
	fmt.Fprintf(&output, "func %s%s%s %s.Component {\n", file.Name, file.TypeParams, file.Params, imports.Sando)
	fmt.Fprintf(&output, "\treturn %s.ComponentFunc(func(%s %s.Context, %s %s.Writer) error {\n", imports.Sando, contextName, imports.Context, writerName, imports.IO)
	fmt.Fprintf(&output, "\t\t_ = %s\n", contextName)

	directiveName := sanitizeDirectivePath(file.Mapping)
	for _, node := range file.Nodes {
		if node.Kind == nodeComment {
			continue
		}
		fmt.Fprintf(&output, "//line %s:%d:%d\n", directiveName, node.Pos.Line, node.Pos.Column)
		switch node.Kind {
		case nodeText:
			if node.Text == "" {
				continue
			}
			fmt.Fprintf(&output, "if %s := %s.WriteString(%s, %s); %s != nil { return %s }\n", errorName, imports.Sando, writerName, strconv.Quote(node.Text), errorName, errorName)
		case nodeStatement:
			output.WriteString(node.Text)
			output.WriteByte('\n')
		case nodeExpression:
			helper := "WriteText"
			switch node.Context {
			case ContextAttr:
				helper = "WriteAttr"
			case ContextRCDATA:
				helper = "WriteRCDATA"
			case ContextURL:
				helper = "WriteURL"
			case ContextJS:
				helper = "WriteJS"
			case ContextCSS:
				helper = "WriteCSS"
			}
			fmt.Fprintf(&output, "if %s := %s.%s(%s, (%s)); %s != nil { return %s }\n", errorName, imports.Sando, helper, writerName, node.Text, errorName, errorName)
		case nodeComponent:
			fmt.Fprintf(&output, "if %s := %s.Render(%s, %s, (%s)); %s != nil { return %s }\n", errorName, imports.Sando, contextName, writerName, node.Text, errorName, errorName)
		}
	}
	output.WriteString("return nil\n")
	output.WriteString("})\n")
	output.WriteString("}\n")

	formatted, err := format.Source([]byte(output.String()))
	if err != nil {
		position := sourcePosition{Line: 1, Column: 1}
		message := err.Error()
		if list, ok := err.(scanner.ErrorList); ok && len(list) != 0 {
			position.Line = list[0].Pos.Line
			position.Column = list[0].Pos.Column
			message = list[0].Msg
		}
		return nil, []Diagnostic{diagnostic(file.Path, position, "HIM1401", "generated Go is invalid: "+message)}
	}
	return formatted, diagnostics
}

func sanitizeDirectivePath(path string) string {
	path = filepath.ToSlash(path)
	var sanitized strings.Builder
	for index := 0; index < len(path); index++ {
		b := path[index]
		if b < 0x20 || b == 0x7f || b == '%' {
			fmt.Fprintf(&sanitized, "%%%02X", b)
			continue
		}
		sanitized.WriteByte(b)
	}
	if sanitized.Len() == 0 {
		return "source.sando"
	}
	return sanitized.String()
}

func prepareImports(file *sourceFile) (backendImports, []Diagnostic) {
	imports := append([]sourceImport(nil), file.Imports...)
	identifiers := sourceIdentifiers(file.Source)
	for _, imported := range imports {
		if imported.Alias != "" && imported.Alias != "_" {
			identifiers[imported.Alias] = true
		} else if imported.Alias == "" {
			identifiers[defaultImportName(imported.Path)] = true
		}
	}

	var diagnostics []Diagnostic
	ensure := func(path, base string) string {
		for _, imported := range imports {
			if imported.Path != path {
				continue
			}
			if imported.Alias == "_" {
				diagnostics = append(diagnostics, diagnostic(file.Path, sourcePosition{Line: 1, Column: 1}, "HIM1410", fmt.Sprintf("internal dependency %q cannot be imported for side effects", path)))
				return ""
			}
			if imported.Alias != "" {
				return imported.Alias
			}
			return defaultImportName(path)
		}
		alias := uniqueIdentifier(base, identifiers)
		imports = append(imports, sourceImport{Alias: alias, Path: path})
		return alias
	}

	contextAlias := ensure("context", "__himesan_context")
	ioAlias := ensure("io", "__himesan_io")
	sandoAlias := ensure(runtimeImportPath, "__himesan_sando")
	sort.SliceStable(imports, func(i, j int) bool {
		if imports[i].Path != imports[j].Path {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	return backendImports{Context: contextAlias, IO: ioAlias, Sando: sandoAlias, All: imports}, diagnostics
}

func sourceIdentifiers(source []byte) map[string]bool {
	identifiers := make(map[string]bool)
	var lexical scanner.Scanner
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("source.sando", -1, len(source))
	lexical.Init(file, source, nil, scanner.ScanComments)
	for {
		_, tok, literal := lexical.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT {
			identifiers[literal] = true
		}
	}
	return identifiers
}

func uniqueIdentifier(base string, used map[string]bool) string {
	name := base
	for suffix := 2; used[name]; suffix++ {
		name = fmt.Sprintf("%s_%d", base, suffix)
	}
	used[name] = true
	return name
}

func defaultImportName(path string) string {
	base := filepath.Base(path)
	if index := strings.IndexByte(base, '.'); index >= 0 {
		base = base[:index]
	}
	base = strings.ReplaceAll(base, "-", "_")
	return base
}

func auditTrustCalls(file *sourceFile) []Diagnostic {
	trusted := map[string]bool{
		"TrustHTML": true,
		"TrustURL":  true,
		"TrustJS":   true,
		"TrustCSS":  true,
	}
	var diagnostics []Diagnostic
	trustedTypeSeen := make(map[string]bool)
	var headerScanner scanner.Scanner
	headerFileSet := token.NewFileSet()
	headerFile := headerFileSet.AddFile(filepath.Base(file.Path), -1, file.HeaderEnd)
	headerScanner.Init(headerFile, file.Source[:file.HeaderEnd], nil, scanner.ScanComments)
	for {
		_, tok, literal := headerScanner.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT && (literal == "TrustedHTML" || literal == "TrustedURL" || literal == "TrustedJS" || literal == "TrustedCSS") && !trustedTypeSeen[literal] {
			trustedTypeSeen[literal] = true
			diagnostics = append(diagnostics, Diagnostic{
				Path:     file.Path,
				Line:     1,
				Column:   1,
				Code:     "HIM1903",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("component signature names %s; audit every value supplied through this trust boundary", literal),
			})
		}
	}
	for _, node := range file.Nodes {
		if node.Kind != nodeExpression && node.Kind != nodeComponent && node.Kind != nodeStatement {
			continue
		}
		if node.Kind == nodeExpression && (node.Context == ContextJS || node.Context == ContextCSS) {
			diagnostics = append(diagnostics, Diagnostic{
				Path:     file.Path,
				Line:     node.Pos.Line,
				Column:   node.Pos.Column,
				Code:     "HIM1902",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("dynamic %s output requires an explicitly trusted runtime value; audit its provenance", node.Context),
			})
		}
		var lexical scanner.Scanner
		set := token.NewFileSet()
		goFile := set.AddFile(filepath.Base(file.Path), -1, len(node.Text))
		lexical.Init(goFile, []byte(node.Text), nil, scanner.ScanComments)
		for {
			_, tok, literal := lexical.Scan()
			if tok == token.EOF {
				break
			}
			if tok == token.IDENT && trusted[literal] {
				diagnostics = append(diagnostics, Diagnostic{
					Path:     file.Path,
					Line:     node.Pos.Line,
					Column:   node.Pos.Column,
					Code:     "HIM1901",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("conspicuous trusted-value constructor %s is used; audit its provenance", literal),
				})
			}
		}
	}
	return diagnostics
}

func bytesEqual(a, b []byte) bool {
	return bytes.Equal(a, b)
}
