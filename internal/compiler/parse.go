// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const headerOpen = "<?sando"

type positionTable struct {
	lineStarts []int
	size       int
}

func newPositionTable(source []byte) positionTable {
	starts := []int{0}
	for i, b := range source {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return positionTable{lineStarts: starts, size: len(source)}
}

func (t positionTable) at(offset int) sourcePosition {
	if offset < 0 {
		offset = 0
	}
	if offset > t.size {
		offset = t.size
	}
	lineIndex := sort.Search(len(t.lineStarts), func(i int) bool {
		return t.lineStarts[i] > offset
	}) - 1
	if lineIndex < 0 {
		lineIndex = 0
	}
	return sourcePosition{
		Offset: offset,
		Line:   lineIndex + 1,
		Column: offset - t.lineStarts[lineIndex] + 1,
	}
}

func parseSource(path string, source []byte) (*sourceFile, []Diagnostic) {
	table := newPositionTable(source)
	var diagnostics []Diagnostic

	if !utf8.Valid(source) {
		diagnostics = append(diagnostics, diagnostic(path, table.at(0), "HIM1001", "source is not valid UTF-8"))
		return nil, diagnostics
	}
	if offset := bytes.IndexByte(source, 0); offset >= 0 {
		diagnostics = append(diagnostics, diagnostic(path, table.at(offset), "HIM1002", "NUL bytes are not permitted in .sando sources"))
		return nil, diagnostics
	}
	headerStart := 0
	if bytes.HasPrefix(source, []byte{0xef, 0xbb, 0xbf}) {
		headerStart = 3
	}
	for headerStart < len(source) && isSpace(source[headerStart]) {
		headerStart++
	}
	if !bytes.HasPrefix(source[headerStart:], []byte(headerOpen)) {
		diagnostics = append(diagnostics, diagnostic(path, table.at(headerStart), "HIM1101", "file must begin (after optional UTF-8 BOM and whitespace) with a <?sando go header"))
		return nil, diagnostics
	}
	afterMarker := headerStart + len(headerOpen)
	if afterMarker >= len(source) || !isSpace(source[afterMarker]) {
		diagnostics = append(diagnostics, diagnostic(path, table.at(afterMarker), "HIM1105", "whitespace is required between <?sando and the target name"))
		return nil, diagnostics
	}
	headerClose := findGoDelimiter(source, afterMarker)
	if headerClose < 0 {
		diagnostics = append(diagnostics, diagnostic(path, table.at(headerStart), "HIM1102", "unterminated <?sando header"))
		return nil, diagnostics
	}
	directiveBody := source[afterMarker:headerClose]
	directiveStart := afterMarker
	leading := len(directiveBody) - len(bytes.TrimLeft(directiveBody, " \t\r\n"))
	directiveBody = directiveBody[leading:]
	directiveStart += leading

	if len(directiveBody) < len("go") || string(directiveBody[:2]) != "go" || (len(directiveBody) > 2 && !isSpace(directiveBody[2])) {
		diagnostics = append(diagnostics, diagnostic(path, table.at(directiveStart), "HIM1103", "unsupported or missing header target; v1 requires <?sando go"))
		return nil, diagnostics
	}
	declarations := directiveBody[2:]
	declarationsStart := directiveStart + 2
	declLeading := len(declarations) - len(bytes.TrimLeft(declarations, " \t\r\n"))
	declarations = declarations[declLeading:]
	declarationsStart += declLeading
	if len(declarations) == 0 {
		diagnostics = append(diagnostics, diagnostic(path, table.at(declarationsStart), "HIM1104", "header must declare a package and one component function"))
		return nil, diagnostics
	}

	parsedHeader, headerDiagnostics := parseHeader(path, declarations, declarationsStart, table)
	diagnostics = append(diagnostics, headerDiagnostics...)
	if parsedHeader == nil {
		return nil, diagnostics
	}

	file := &sourceFile{
		Path:        path,
		Mapping:     filepath.ToSlash(filepath.Base(path)),
		Package:     parsedHeader.Package,
		Name:        parsedHeader.Name,
		TypeParams:  parsedHeader.TypeParams,
		Params:      parsedHeader.Params,
		Imports:     parsedHeader.Imports,
		Source:      source,
		HeaderEnd:   headerClose + 2,
		FunctionPos: parsedHeader.FunctionPos,
		AST:         parsedHeader.AST,
	}

	templateDiagnostics := tokenizeTemplate(file, source[headerClose+2:], headerClose+2, table)
	diagnostics = append(diagnostics, templateDiagnostics...)
	if hasErrors(diagnostics) {
		return nil, diagnostics
	}
	return file, diagnostics
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

type parsedHeader struct {
	Package     string
	Name        string
	TypeParams  string
	Params      string
	Imports     []sourceImport
	FunctionPos sourcePosition
	AST         *ast.File
}

func parseHeader(path string, declarations []byte, sourceOffset int, table positionTable) (*parsedHeader, []Diagnostic) {
	parseInput, syntheticBraceOffset := insertSyntheticFunctionBody(declarations)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, filepath.Base(path), parseInput, parser.AllErrors)
	if err != nil {
		return nil, parserDiagnostics(path, err, sourceOffset, table, "HIM1110", "invalid Go header")
	}

	var function *ast.FuncDecl
	var imports []sourceImport
	var diagnostics []Diagnostic
	for _, declaration := range parsed.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			if declaration.Tok != token.IMPORT {
				pos := fset.Position(declaration.Pos())
				diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+pos.Offset), "HIM1111", "header may contain only imports and one component function signature"))
				continue
			}
			for _, spec := range declaration.Specs {
				importSpec, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}
				importPath, unquoteErr := strconv.Unquote(importSpec.Path.Value)
				if unquoteErr != nil || importPath == "" {
					pos := fset.Position(importSpec.Path.Pos())
					diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+pos.Offset), "HIM1112", "invalid import path"))
					continue
				}
				alias := ""
				if importSpec.Name != nil {
					alias = importSpec.Name.Name
				}
				if alias == "." {
					pos := fset.Position(importSpec.Pos())
					diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+pos.Offset), "HIM1113", "dot imports are not supported in .sando headers"))
					continue
				}
				imports = append(imports, sourceImport{Alias: alias, Path: importPath})
			}
		case *ast.FuncDecl:
			if function != nil {
				pos := fset.Position(declaration.Pos())
				diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+pos.Offset), "HIM1114", "a .sando file declares exactly one component"))
				continue
			}
			function = declaration
		default:
			pos := fset.Position(declaration.Pos())
			diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+pos.Offset), "HIM1111", "unsupported declaration in header"))
		}
	}

	if function == nil {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset), "HIM1115", "header must contain one bodyless component function signature"))
		return nil, diagnostics
	}
	functionPosition := fset.Position(function.Pos())
	if function.Name.Name == "init" || parsed.Name.Name == "main" && function.Name.Name == "main" {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+functionPosition.Offset), "HIM1123", fmt.Sprintf("%s.%s is reserved by Go and cannot be a component API", parsed.Name.Name, function.Name.Name)))
	}
	if function.Recv != nil {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+functionPosition.Offset), "HIM1116", "component functions cannot have receivers"))
	}
	if function.Type.Results != nil && len(function.Type.Results.List) != 0 {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+functionPosition.Offset), "HIM1117", "component signatures do not declare results; Hime-san generates sando.Component"))
	}
	if function.Body == nil {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+functionPosition.Offset), "HIM1118", "component signature could not be parsed"))
	} else {
		bodyOffset := fset.Position(function.Body.Lbrace).Offset
		if bodyOffset != syntheticBraceOffset {
			diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+bodyOffset), "HIM1119", "component signature must be bodyless; ?> begins the template body"))
		}
	}
	if hasErrors(diagnostics) {
		return nil, diagnostics
	}

	formattedType, formatErr := formatNode(fset, function.Type)
	if formatErr != nil {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+functionPosition.Offset), "HIM1120", "could not format component parameters: "+formatErr.Error()))
		return nil, diagnostics
	}
	typeParams, params, splitErr := splitFormattedFuncType(formattedType)
	if splitErr != nil {
		diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+functionPosition.Offset), "HIM1121", "could not recover formatted component signature: "+splitErr.Error()))
		return nil, diagnostics
	}

	sort.SliceStable(imports, func(i, j int) bool {
		if imports[i].Path != imports[j].Path {
			return imports[i].Path < imports[j].Path
		}
		return imports[i].Alias < imports[j].Alias
	})
	for index := 1; index < len(imports); index++ {
		if imports[index-1].Path == imports[index].Path {
			diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset), "HIM1122", fmt.Sprintf("import path %q is declared more than once", imports[index].Path)))
		}
	}
	if hasErrors(diagnostics) {
		return nil, diagnostics
	}

	return &parsedHeader{
		Package:     parsed.Name.Name,
		Name:        function.Name.Name,
		TypeParams:  typeParams,
		Params:      params,
		Imports:     imports,
		FunctionPos: table.at(sourceOffset + functionPosition.Offset),
		AST:         parsed,
	}, diagnostics
}

func insertSyntheticFunctionBody(declarations []byte) ([]byte, int) {
	trimmed := bytes.TrimRight(declarations, " \t\r\n")
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("header.go", -1, len(trimmed))
	var lexical scanner.Scanner
	lexical.Init(file, trimmed, nil, scanner.ScanComments)
	seenFunction := false
	squareDepth := 0
	curlyDepth := 0
	parameterDepth := 0
	parameterListStarted := false
	insertion := -1
	for {
		position, tok, _ := lexical.Scan()
		if tok == token.EOF {
			break
		}
		if !seenFunction {
			if tok == token.FUNC {
				seenFunction = true
			}
			continue
		}
		if parameterListStarted {
			switch tok {
			case token.LPAREN:
				parameterDepth++
			case token.RPAREN:
				parameterDepth--
				if parameterDepth == 0 {
					insertion = fileSet.Position(position).Offset + 1
				}
			}
			if insertion >= 0 {
				break
			}
			continue
		}
		switch tok {
		case token.LBRACK:
			squareDepth++
		case token.RBRACK:
			if squareDepth > 0 {
				squareDepth--
			}
		case token.LBRACE:
			curlyDepth++
		case token.RBRACE:
			if curlyDepth > 0 {
				curlyDepth--
			}
		case token.LPAREN:
			if squareDepth == 0 && curlyDepth == 0 {
				parameterListStarted = true
				parameterDepth = 1
			}
		}
	}
	if insertion < 0 || insertion > len(trimmed) {
		insertion = len(trimmed)
	}
	parseInput := make([]byte, 0, len(trimmed)+3)
	parseInput = append(parseInput, trimmed[:insertion]...)
	parseInput = append(parseInput, ' ', '{', '}')
	parseInput = append(parseInput, trimmed[insertion:]...)
	return parseInput, insertion + 1
}

func splitFormattedFuncType(formatted string) (typeParams, params string, err error) {
	remainder := strings.TrimSpace(strings.TrimPrefix(formatted, "func"))
	bracketDepth := 0
	quote := byte(0)
	escaped := false
	for index := 0; index < len(remainder); index++ {
		b := remainder[index]
		if quote != 0 {
			if quote != '`' && escaped {
				escaped = false
				continue
			}
			if quote != '`' && b == '\\' {
				escaped = true
				continue
			}
			if b == quote {
				quote = 0
			}
			continue
		}
		if b == '"' || b == '\'' || b == '`' {
			quote = b
			continue
		}
		switch b {
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '(':
			if bracketDepth == 0 {
				return strings.TrimSpace(remainder[:index]), strings.TrimSpace(remainder[index:]), nil
			}
		}
	}
	return "", "", fmt.Errorf("formatted function type has no parameter list")
}

func formatNode(fset *token.FileSet, node any) (string, error) {
	var output bytes.Buffer
	configuration := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := configuration.Fprint(&output, fset, node); err != nil {
		return "", err
	}
	return output.String(), nil
}

func parserDiagnostics(path string, err error, sourceOffset int, table positionTable, code, prefix string) []Diagnostic {
	var diagnostics []Diagnostic
	if list, ok := err.(scanner.ErrorList); ok {
		for _, parseError := range list {
			offset := sourceOffset + parseError.Pos.Offset
			diagnostics = append(diagnostics, diagnostic(path, table.at(offset), code, prefix+": "+parseError.Msg))
		}
		return diagnostics
	}
	return []Diagnostic{diagnostic(path, table.at(sourceOffset), code, prefix+": "+err.Error())}
}

func tokenizeTemplate(file *sourceFile, template []byte, sourceOffset int, table positionTable) []Diagnostic {
	var diagnostics []Diagnostic
	cursor := 0
	for cursor < len(template) {
		openRelative := bytes.Index(template[cursor:], []byte("<?"))
		if openRelative < 0 {
			if cursor < len(template) {
				file.Nodes = append(file.Nodes, rendererNode{Kind: nodeText, Text: string(template[cursor:]), Pos: table.at(sourceOffset + cursor)})
			}
			break
		}
		open := cursor + openRelative
		if open > cursor {
			file.Nodes = append(file.Nodes, rendererNode{Kind: nodeText, Text: string(template[cursor:open]), Pos: table.at(sourceOffset + cursor)})
		}
		close := -1
		if open+2 < len(template) && template[open+2] == '#' {
			if closeRelative := bytes.Index(template[open+2:], []byte("?>")); closeRelative >= 0 {
				close = open + 2 + closeRelative
			}
		} else {
			close = findGoDelimiter(template, open+2)
		}
		if close < 0 {
			diagnostics = append(diagnostics, diagnostic(file.Path, table.at(sourceOffset+open), "HIM1201", "unterminated template tag"))
			break
		}
		kind := nodeStatement
		contentStart := open + 2
		if contentStart < close {
			switch template[contentStart] {
			case '=':
				kind = nodeExpression
				contentStart++
			case '~':
				kind = nodeComponent
				contentStart++
			case '#':
				kind = nodeComment
				contentStart++
			}
		}
		content := template[contentStart:close]
		trimmed := bytes.TrimSpace(content)
		trimLeading := len(content) - len(bytes.TrimLeft(content, " \t\r\n"))
		position := table.at(sourceOffset + contentStart + trimLeading)
		if bytes.HasPrefix(bytes.TrimSpace(template[open+2:close]), []byte("sando")) {
			diagnostics = append(diagnostics, diagnostic(file.Path, table.at(sourceOffset+open), "HIM1202", "<?sando is only valid as the file header"))
		} else if kind != nodeComment && len(trimmed) == 0 {
			diagnostics = append(diagnostics, diagnostic(file.Path, table.at(sourceOffset+open), "HIM1203", "empty template tag"))
		} else {
			node := rendererNode{Kind: kind, Text: string(trimmed), Context: ContextNone, Pos: position}
			if kind == nodeExpression || kind == nodeComponent {
				if _, err := parser.ParseExprFrom(token.NewFileSet(), filepath.Base(file.Path), trimmed, parser.AllErrors); err != nil {
					diagnostics = append(diagnostics, expressionDiagnostics(file.Path, err, position, table, sourceOffset+contentStart+trimLeading)...)
				} else {
					file.Nodes = append(file.Nodes, node)
				}
			} else {
				file.Nodes = append(file.Nodes, node)
			}
		}
		cursor = close + 2
	}
	return diagnostics
}

// findGoDelimiter returns the first ?> outside Go strings, rune literals, raw
// strings, and comments. Statement tags may be structurally incomplete across
// template regions, so requiring each region to parse independently would
// reject ordinary `if { ?>...<? }` usage.
func findGoDelimiter(source []byte, start int) int {
	type lexicalState uint8
	const (
		lexicalNormal lexicalState = iota
		lexicalString
		lexicalRune
		lexicalRawString
		lexicalLineComment
		lexicalBlockComment
	)
	state := lexicalNormal
	escaped := false
	for index := start; index < len(source); index++ {
		b := source[index]
		next := byte(0)
		if index+1 < len(source) {
			next = source[index+1]
		}
		switch state {
		case lexicalNormal:
			switch {
			case b == '?' && next == '>':
				return index
			case b == '"':
				state = lexicalString
				escaped = false
			case b == '\'':
				state = lexicalRune
				escaped = false
			case b == '`':
				state = lexicalRawString
			case b == '/' && next == '/':
				state = lexicalLineComment
				index++
			case b == '/' && next == '*':
				state = lexicalBlockComment
				index++
			}
		case lexicalString, lexicalRune:
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' {
				escaped = true
				continue
			}
			if (state == lexicalString && b == '"') || (state == lexicalRune && b == '\'') {
				state = lexicalNormal
			}
		case lexicalRawString:
			if b == '`' {
				state = lexicalNormal
			}
		case lexicalLineComment:
			if b == '\n' {
				state = lexicalNormal
			}
		case lexicalBlockComment:
			if b == '*' && next == '/' {
				state = lexicalNormal
				index++
			}
		}
	}
	return -1
}

func expressionDiagnostics(path string, err error, fallback sourcePosition, table positionTable, sourceOffset int) []Diagnostic {
	if list, ok := err.(scanner.ErrorList); ok {
		diagnostics := make([]Diagnostic, 0, len(list))
		for _, parseError := range list {
			diagnostics = append(diagnostics, diagnostic(path, table.at(sourceOffset+parseError.Pos.Offset), "HIM1210", "invalid Go expression: "+parseError.Msg))
		}
		return diagnostics
	}
	return []Diagnostic{diagnostic(path, fallback, "HIM1210", "invalid Go expression: "+strings.TrimSpace(err.Error()))}
}
