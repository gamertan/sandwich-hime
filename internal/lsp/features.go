// SPDX-License-Identifier: AGPL-3.0-only

package lsp

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"gamertan.com/sandwich-hime/internal/compiler"
)

type completionItem struct {
	Label            string `json:"label"`
	Kind             int    `json:"kind,omitempty"`
	Detail           string `json:"detail,omitempty"`
	Documentation    any    `json:"documentation,omitempty"`
	InsertText       string `json:"insertText,omitempty"`
	InsertTextFormat int    `json:"insertTextFormat,omitempty"`
	SortText         string `json:"sortText,omitempty"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

func (server *Server) completions(request textDocumentPositionParams) any {
	document, analysis, ok := server.snapshotDocument(request.TextDocument.URI)
	if !ok {
		return []completionItem{}
	}
	offset, ok := positionToOffset(document.Text, request.Position)
	if !ok {
		return []completionItem{}
	}
	open := bytes.LastIndex(document.Text[:offset], []byte("<?"))
	close := bytes.LastIndex(document.Text[:offset], []byte("?>"))
	if open >= 0 && open > close && bytes.HasPrefix(document.Text[open:offset], []byte("<?~")) {
		server.mu.RLock()
		snapshot := server.snapshot
		server.mu.RUnlock()
		prefix := strings.TrimSpace(string(document.Text[open+3 : offset]))
		items := make([]completionItem, 0)
		for _, target := range snapshot.componentsFor(document.Path, analysis) {
			label := target.Label()
			if prefix != "" && !strings.HasPrefix(label, prefix) {
				continue
			}
			items = append(items, completionItem{
				Label: label, Kind: 3, Detail: target.Signature,
				Documentation: markupContent{Kind: "markdown", Value: "Typed `.sando` component. Hime-san emits an ordinary Go constructor."},
				InsertText:    label, InsertTextFormat: 1, SortText: "1-" + label,
			})
		}
		return struct {
			IsIncomplete bool             `json:"isIncomplete"`
			Items        []completionItem `json:"items"`
		}{Items: items}
	}
	return struct {
		IsIncomplete bool             `json:"isIncomplete"`
		Items        []completionItem `json:"items"`
	}{Items: tagCompletions()}
}

func tagCompletions() []completionItem {
	return []completionItem{
		{Label: "<?sando go", Kind: 15, Detail: "component file header", InsertText: "<?sando go\npackage ${1:views}\nfunc ${2:Component}(${3:})\n?>", InsertTextFormat: 2, SortText: "0-header"},
		{Label: "<? … ?>", Kind: 15, Detail: "Go statement", InsertText: "<? ${1:if condition {} } ?>", InsertTextFormat: 2, SortText: "0-statement"},
		{Label: "<?= … ?>", Kind: 15, Detail: "contextually escaped expression", InsertText: "<?= ${1:value} ?>", InsertTextFormat: 2, SortText: "0-expression"},
		{Label: "<?~ … ?>", Kind: 15, Detail: "typed component composition", InsertText: "<?~ ${1:Component()} ?>", InsertTextFormat: 2, SortText: "0-component"},
		{Label: "<?# … ?>", Kind: 15, Detail: "Hime-san template comment", InsertText: "<?# ${1:comment} ?>", InsertTextFormat: 2, SortText: "0-comment"},
	}
}

func (server *Server) hover(request textDocumentPositionParams) any {
	document, analysis, ok := server.snapshotDocument(request.TextDocument.URI)
	if !ok {
		return nil
	}
	offset, ok := positionToOffset(document.Text, request.Position)
	if !ok {
		return nil
	}
	for _, region := range analysis.Regions {
		if offset < region.Offset || offset > region.Offset+region.Length {
			continue
		}
		if region.Kind == compiler.AnalysisComponent {
			qualifier, name := referenceAt(region.Text, offset-region.Offset)
			if name != "" {
				if target, found := server.resolveComponent(document.Path, analysis, qualifier, name); found {
					value := "```go\n" + target.Signature + "\n```\n\nTyped component composition. Handwritten `sando.Component` values are trusted output capabilities."
					return hoverResult{Contents: markupContent{Kind: "markdown", Value: value}}
				}
			}
		}
		value := fmt.Sprintf("**Hime-san %s region**\n\nOutput context: `%s`.", region.Kind, region.Context)
		if region.Context == compiler.ContextJS || region.Context == compiler.ContextCSS || strings.Contains(region.Text, "Trust") {
			value += "\n\nTrusted output is an explicit security capability; audit its provenance and parser-state effects."
		}
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: value}}
	}
	if marker, start, end := enclosingTag(document.Text, offset); marker != "" {
		if text := tagDocumentation(marker); text != "" {
			rangeValue := Range{Start: offsetToPosition(document.Text, start), End: offsetToPosition(document.Text, end)}
			return hoverResult{Contents: markupContent{Kind: "markdown", Value: text}, Range: &rangeValue}
		}
	}
	return nil
}

func tagDocumentation(marker string) string {
	switch marker {
	case "<?sando":
		return "**`<?sando go … ?>`** declares the Go package, imports, and one typed component signature. It must be the first non-whitespace content."
	case "<?~":
		return "**`<?~ … ?>`** composes a typed `sando.Component` at an HTML content boundary. It is not template inheritance."
	case "<?=":
		return "**`<?= … ?>`** renders an expression through the helper selected by Hime-san's inferred HTML output context."
	case "<?#":
		return "**`<?# … ?>`** is a Hime-san comment. It emits no bytes and cannot change HTML parser state."
	case "<?":
		return "**`<? … ?>`** contains Go statements and is valid only at an HTML content boundary."
	}
	return ""
}

func enclosingTag(text []byte, offset int) (string, int, int) {
	if offset < 0 || offset > len(text) {
		return "", 0, 0
	}
	open := bytes.LastIndex(text[:offset], []byte("<?"))
	if open < 0 {
		return "", 0, 0
	}
	closeRelative := bytes.Index(text[open:], []byte("?>"))
	if closeRelative < 0 || open+closeRelative+2 < offset {
		return "", 0, 0
	}
	end := open + closeRelative + 2
	marker := "<?"
	for _, candidate := range []string{"<?sando", "<?~", "<?=", "<?#"} {
		if bytes.HasPrefix(text[open:end], []byte(candidate)) {
			marker = candidate
			break
		}
	}
	return marker, open, end
}

func (server *Server) definition(request textDocumentPositionParams) any {
	document, analysis, ok := server.snapshotDocument(request.TextDocument.URI)
	if !ok {
		return nil
	}
	offset, ok := positionToOffset(document.Text, request.Position)
	if !ok {
		return nil
	}
	for _, region := range analysis.Regions {
		if region.Kind != compiler.AnalysisComponent || offset < region.Offset || offset > region.Offset+region.Length {
			continue
		}
		qualifier, name := referenceAt(region.Text, offset-region.Offset)
		if name == "" {
			return nil
		}
		target, found := server.resolveComponent(document.Path, analysis, qualifier, name)
		if !found {
			return nil
		}
		server.mu.RLock()
		targetDocument, exists := server.snapshot.documents[target.Path]
		server.mu.RUnlock()
		if !exists {
			return nil
		}
		start := target.Analysis.ComponentOffset
		end := start + len(target.Analysis.Signature)
		return Location{URI: targetDocument.URI, Range: Range{Start: offsetToPosition(targetDocument.Text, start), End: offsetToPosition(targetDocument.Text, end)}}
	}
	return nil
}

func (server *Server) resolveComponent(path string, analysis compiler.DocumentAnalysis, qualifier, name string) (componentTarget, bool) {
	server.mu.RLock()
	snapshot := server.snapshot
	server.mu.RUnlock()
	for _, target := range snapshot.componentsFor(path, analysis) {
		if target.Qualifier == qualifier && target.Name == name {
			return target, true
		}
	}
	return componentTarget{}, false
}

func referenceAt(expression string, cursor int) (string, string) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(expression) {
		cursor = len(expression)
	}
	if cursor == len(expression) && cursor > 0 {
		cursor--
	}
	for cursor > 0 && cursor < len(expression) && !identifierByte(expression[cursor]) && expression[cursor] != '.' {
		cursor--
	}
	start := cursor
	for start > 0 && (identifierByte(expression[start-1]) || expression[start-1] == '.') {
		start--
	}
	end := cursor
	for end < len(expression) && (identifierByte(expression[end]) || expression[end] == '.') {
		end++
	}
	reference := strings.Trim(expression[start:end], ".")
	parts := strings.Split(reference, ".")
	if len(parts) == 1 && validIdentifier(parts[0]) {
		return "", parts[0]
	}
	if len(parts) == 2 && validIdentifier(parts[0]) && validIdentifier(parts[1]) {
		return parts[0], parts[1]
	}
	return "", ""
}

func identifierByte(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 && !(r == '_' || unicode.IsLetter(r)) {
			return false
		}
		if index != 0 && !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func (server *Server) documentSymbols(uri string) []documentSymbol {
	document, analysis, ok := server.snapshotDocument(uri)
	if !ok || analysis.Component == "" {
		return []documentSymbol{}
	}
	documentRange := Range{Start: Position{}, End: offsetToPosition(document.Text, len(document.Text))}
	selectionStart := analysis.ComponentOffset
	selectionEnd := selectionStart + len(analysis.Component)
	children := make([]documentSymbol, 0, len(analysis.Regions))
	for index, region := range analysis.Regions {
		end := region.Offset + region.Length
		name := fmt.Sprintf("%s %d", region.Kind, index+1)
		if region.Kind == compiler.AnalysisComponent {
			_, componentName := referenceAt(region.Text, 0)
			if componentName != "" {
				name = "component " + componentName
			}
		}
		rangeValue := Range{Start: offsetToPosition(document.Text, region.Offset), End: offsetToPosition(document.Text, end)}
		children = append(children, documentSymbol{Name: name, Detail: string(region.Context), Kind: 13, Range: rangeValue, SelectionRange: rangeValue})
	}
	return []documentSymbol{{
		Name: analysis.Component, Detail: analysis.Signature, Kind: 12,
		Range:          documentRange,
		SelectionRange: Range{Start: offsetToPosition(document.Text, selectionStart), End: offsetToPosition(document.Text, selectionEnd)},
		Children:       children,
	}}
}

func runeEnd(text []byte, offset int) int {
	if offset >= len(text) {
		return len(text)
	}
	_, size := utf8.DecodeRune(text[offset:])
	if size < 1 {
		size = 1
	}
	return offset + size
}
