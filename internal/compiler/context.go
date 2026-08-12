// SPDX-License-Identifier: AGPL-3.0-only

package compiler

import (
	"fmt"
	"html"
	"strings"
)

type htmlState uint8

const (
	htmlData htmlState = iota
	htmlAfterLT
	htmlDeclarationStart
	htmlDeclarationDash
	htmlDeclaration
	htmlComment
	htmlTagName
	htmlBeforeAttribute
	htmlAttributeName
	htmlAfterAttributeName
	htmlBeforeAttributeValue
	htmlAttributeDoubleQuoted
	htmlAttributeSingleQuoted
	htmlSelfClosing
	htmlEndTagName
	htmlAfterEndTagName
	htmlRawText
	htmlRawAfterLT
	htmlRawEndTagName
	htmlRawAfterEndTagName
)

type contextAnalyzer struct {
	file      *sourceFile
	positions positionTable

	state       htmlState
	currentTag  string
	currentAttr string
	stack       []string
	selfClosing bool
	commentDash int
	rawTag      string
	rawEndName  string
	scriptTail  string
	attributes  map[string]string
	seenAttrs   map[string]bool

	attrLiteral       strings.Builder
	attrDynamicNodes  []int
	attrFirstDynamic  int
	attrDynamicPrefix string
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

var urlAttributes = map[string]bool{
	"action": true, "background": true, "cite": true, "classid": true,
	"code": true, "codebase": true, "data": true, "dynsrc": true,
	"formaction": true, "href": true, "icon": true, "itemid": true,
	"longdesc": true, "lowsrc": true, "manifest": true, "poster": true,
	"profile": true, "src": true, "usemap": true, "xlink:href": true,
	"xmlns": true,
}

var unsupportedDynamicAttributes = map[string]string{
	"archive":     "URL-list",
	"imagesrcset": "responsive-image URL-list",
	"itemtype":    "URL-list",
	"ping":        "URL-list",
	"srcdoc":      "nested HTML",
	"srcset":      "responsive-image URL-list",
}

func analyzeContexts(file *sourceFile) []Diagnostic {
	analyzer := &contextAnalyzer{
		file:             file,
		positions:        newPositionTable(file.Source),
		state:            htmlData,
		attrFirstDynamic: -1,
	}
	var diagnostics []Diagnostic
	for nodeIndex := range file.Nodes {
		node := &file.Nodes[nodeIndex]
		switch node.Kind {
		case nodeText:
			if d := analyzer.consumeText(node.Text, node.Pos); d != nil {
				diagnostics = append(diagnostics, *d)
				return diagnostics
			}
		case nodeComment:
			// Hime-san comments emit no bytes and cannot change HTML state.
		case nodeStatement:
			if analyzer.state != htmlData {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1301", "Go statements are only allowed at HTML content boundaries"))
				return diagnostics
			}
			node.Context = ContextNone
		case nodeComponent:
			if analyzer.state != htmlData {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1302", "component rendering (<?~) is only allowed at HTML content boundaries"))
				return diagnostics
			}
			node.Context = ContextHTMLText
		case nodeExpression:
			if analyzer.state == htmlBeforeAttributeValue {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1328", "attribute values must be quoted"))
				return diagnostics
			}
			if analyzer.inAttribute() && unsupportedDynamicAttributes[analyzer.currentAttr] != "" {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1345", fmt.Sprintf("dynamic %s attributes require an unsupported %s context in v1", analyzer.currentAttr, unsupportedDynamicAttributes[analyzer.currentAttr])))
				return diagnostics
			}
			if analyzer.inAttribute() && analyzer.currentTag == "meta" && analyzer.currentAttr == "http-equiv" {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1346", "dynamic meta http-equiv values are not supported in v1"))
				return diagnostics
			}
			if analyzer.inAttribute() && analyzer.currentAttr == "style" {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1343", "dynamic style attributes are not supported in v1; use a static style attribute or a class"))
				return diagnostics
			}
			context, ok := analyzer.dynamicContext()
			if !ok {
				diagnostics = append(diagnostics, diagnostic(file.Path, node.Pos, "HIM1303", "dynamic output is not allowed while constructing markup; use HTML text or a quoted attribute value"))
				return diagnostics
			}
			node.Context = context
			if analyzer.inAttribute() {
				if analyzer.attrFirstDynamic < 0 {
					analyzer.attrFirstDynamic = nodeIndex
					analyzer.attrDynamicPrefix = analyzer.attrLiteral.String()
				}
				analyzer.attrDynamicNodes = append(analyzer.attrDynamicNodes, nodeIndex)
			}
		}
	}

	end := analyzer.positions.at(len(file.Source))
	if analyzer.state != htmlData {
		diagnostics = append(diagnostics, diagnostic(file.Path, end, "HIM1310", "template ends in an incomplete or ambiguous HTML parser context"))
	}
	if len(analyzer.stack) != 0 {
		diagnostics = append(diagnostics, diagnostic(file.Path, end, "HIM1311", fmt.Sprintf("component must finish in its starting HTML context; unclosed <%s>", analyzer.stack[len(analyzer.stack)-1])))
	}
	return diagnostics
}

func (a *contextAnalyzer) consumeText(text string, start sourcePosition) *Diagnostic {
	for index := 0; index < len(text); index++ {
		b := text[index]
		position := a.positions.at(start.Offset + index)
		if a.rawTag == "script" {
			a.scriptTail += string(b)
			if len(a.scriptTail) > len("<!--") {
				a.scriptTail = a.scriptTail[len(a.scriptTail)-len("<!--"):]
			}
			if a.scriptTail == "<!--" {
				return a.problem(position, "HIM1357", "HTML comment syntax inside <script> is not supported because it enters ambiguous escaped script parser states; remove the <!-- sequence")
			}
		}
		reprocess := true
		for reprocess {
			reprocess = false
			switch a.state {
			case htmlData:
				if b == '<' {
					a.state = htmlAfterLT
				}
			case htmlAfterLT:
				switch {
				case b == '!':
					a.state = htmlDeclarationStart
				case b == '/':
					a.currentTag = ""
					a.state = htmlEndTagName
				case isTagNameStart(b):
					a.currentTag = strings.ToLower(string(b))
					a.attributes = make(map[string]string)
					a.seenAttrs = make(map[string]bool)
					a.selfClosing = false
					a.state = htmlTagName
				default:
					return a.problem(position, "HIM1320", "malformed HTML after '<'; dynamic tag construction is not supported")
				}
			case htmlDeclarationStart:
				if b == '-' {
					a.state = htmlDeclarationDash
				} else if isASCIILetter(b) {
					a.state = htmlDeclaration
				} else {
					return a.problem(position, "HIM1321", "unsupported HTML declaration")
				}
			case htmlDeclarationDash:
				if b != '-' {
					return a.problem(position, "HIM1322", "malformed HTML comment opening")
				}
				a.commentDash = 0
				a.state = htmlComment
			case htmlDeclaration:
				if b == '>' {
					a.state = htmlData
				} else if b == '<' {
					return a.problem(position, "HIM1323", "malformed HTML declaration")
				}
			case htmlComment:
				if b == '-' {
					a.commentDash++
				} else if b == '>' && a.commentDash >= 2 {
					a.commentDash = 0
					a.state = htmlData
				} else {
					a.commentDash = 0
				}
			case htmlTagName:
				switch {
				case isTagNameChar(b):
					a.currentTag += strings.ToLower(string(b))
				case isHTMLSpace(b):
					a.state = htmlBeforeAttribute
				case b == '>':
					if d := a.finishOpenTag(position); d != nil {
						return d
					}
				case b == '/':
					a.selfClosing = true
					a.state = htmlSelfClosing
				default:
					return a.problem(position, "HIM1324", "unsupported character in HTML tag name")
				}
			case htmlBeforeAttribute:
				switch {
				case isHTMLSpace(b):
				case isAttributeNameStart(b):
					a.beginAttribute(b)
					a.state = htmlAttributeName
				case b == '>':
					if d := a.finishOpenTag(position); d != nil {
						return d
					}
				case b == '/':
					a.selfClosing = true
					a.state = htmlSelfClosing
				default:
					return a.problem(position, "HIM1325", "dynamic or malformed attribute names are not supported")
				}
			case htmlAttributeName:
				switch {
				case isAttributeNameChar(b):
					a.currentAttr += strings.ToLower(string(b))
				case isHTMLSpace(b):
					if d := a.validateAttributeName(position); d != nil {
						return d
					}
					a.state = htmlAfterAttributeName
				case b == '=':
					if d := a.validateAttributeName(position); d != nil {
						return d
					}
					a.state = htmlBeforeAttributeValue
				case b == '>':
					if d := a.validateAttributeName(position); d != nil {
						return d
					}
					if d := a.finishOpenTag(position); d != nil {
						return d
					}
				case b == '/':
					if d := a.validateAttributeName(position); d != nil {
						return d
					}
					a.selfClosing = true
					a.state = htmlSelfClosing
				default:
					return a.problem(position, "HIM1326", "unsupported character in attribute name")
				}
			case htmlAfterAttributeName:
				switch {
				case isHTMLSpace(b):
				case b == '=':
					a.state = htmlBeforeAttributeValue
				case isAttributeNameStart(b):
					a.beginAttribute(b)
					a.state = htmlAttributeName
				case b == '>':
					if d := a.finishOpenTag(position); d != nil {
						return d
					}
				case b == '/':
					a.selfClosing = true
					a.state = htmlSelfClosing
				default:
					return a.problem(position, "HIM1327", "expected '=' or another attribute")
				}
			case htmlBeforeAttributeValue:
				switch {
				case isHTMLSpace(b):
				case b == '"':
					a.resetAttributeValue()
					a.state = htmlAttributeDoubleQuoted
				case b == '\'':
					a.resetAttributeValue()
					a.state = htmlAttributeSingleQuoted
				default:
					return a.problem(position, "HIM1328", "attribute values must be quoted")
				}
			case htmlAttributeDoubleQuoted:
				if b == '"' {
					if d := a.finishAttributeValue(position); d != nil {
						return d
					}
					a.state = htmlBeforeAttribute
				} else if b == '<' {
					return a.problem(position, "HIM1329", "'<' is not supported inside attribute values")
				} else {
					a.attrLiteral.WriteByte(b)
				}
			case htmlAttributeSingleQuoted:
				if b == '\'' {
					if d := a.finishAttributeValue(position); d != nil {
						return d
					}
					a.state = htmlBeforeAttribute
				} else if b == '<' {
					return a.problem(position, "HIM1329", "'<' is not supported inside attribute values")
				} else {
					a.attrLiteral.WriteByte(b)
				}
			case htmlSelfClosing:
				if isHTMLSpace(b) {
					continue
				}
				if b != '>' {
					return a.problem(position, "HIM1330", "expected '>' after '/' in a tag")
				}
				if d := a.finishOpenTag(position); d != nil {
					return d
				}
			case htmlEndTagName:
				switch {
				case isTagNameChar(b):
					a.currentTag += strings.ToLower(string(b))
				case isHTMLSpace(b) && a.currentTag != "":
					a.state = htmlAfterEndTagName
				case b == '>' && a.currentTag != "":
					if d := a.finishCloseTag(position); d != nil {
						return d
					}
				default:
					return a.problem(position, "HIM1331", "malformed closing tag")
				}
			case htmlAfterEndTagName:
				if isHTMLSpace(b) {
					continue
				}
				if b != '>' {
					return a.problem(position, "HIM1332", "unexpected content in closing tag")
				}
				if d := a.finishCloseTag(position); d != nil {
					return d
				}
			case htmlRawText:
				if b == '<' {
					a.state = htmlRawAfterLT
				}
			case htmlRawAfterLT:
				if b == '/' {
					a.rawEndName = ""
					a.state = htmlRawEndTagName
				} else if b != '<' {
					a.state = htmlRawText
				}
			case htmlRawEndTagName:
				if isTagNameChar(b) {
					a.rawEndName += strings.ToLower(string(b))
					continue
				}
				if a.rawEndName != a.rawTag {
					a.state = htmlRawText
					if b == '<' {
						a.state = htmlRawAfterLT
					}
					continue
				}
				if b == '>' {
					if d := a.finishRawClose(position); d != nil {
						return d
					}
				} else if isHTMLSpace(b) {
					a.state = htmlRawAfterEndTagName
				} else {
					a.state = htmlRawText
				}
			case htmlRawAfterEndTagName:
				if isHTMLSpace(b) {
					continue
				}
				if b != '>' {
					return a.problem(position, "HIM1333", "malformed script/style closing tag")
				}
				if d := a.finishRawClose(position); d != nil {
					return d
				}
			}
		}
	}
	return nil
}

func (a *contextAnalyzer) beginAttribute(first byte) {
	a.currentAttr = strings.ToLower(string(first))
	a.resetAttributeValue()
}

func (a *contextAnalyzer) resetAttributeValue() {
	a.attrLiteral.Reset()
	a.attrDynamicNodes = a.attrDynamicNodes[:0]
	a.attrFirstDynamic = -1
	a.attrDynamicPrefix = ""
}

func (a *contextAnalyzer) validateAttributeName(position sourcePosition) *Diagnostic {
	if a.seenAttrs[a.currentAttr] {
		return a.problem(position, "HIM1347", fmt.Sprintf("duplicate attribute %q creates ambiguous browser parsing", a.currentAttr))
	}
	a.seenAttrs[a.currentAttr] = true
	if strings.HasPrefix(a.currentAttr, "on") {
		return a.problem(position, "HIM1340", fmt.Sprintf("event-handler attribute %q is not supported", a.currentAttr))
	}
	return nil
}

func (a *contextAnalyzer) finishAttributeValue(position sourcePosition) *Diagnostic {
	if len(a.attrDynamicNodes) == 0 {
		a.attributes[a.currentAttr] = html.UnescapeString(a.attrLiteral.String())
	}
	if !urlAttributes[a.currentAttr] {
		return nil
	}
	if len(a.attrDynamicNodes) == 0 {
		if !safeStaticURL(a.attrLiteral.String()) {
			return a.problem(position, "HIM1344", "static URL uses a dangerous or ambiguous scheme; use a safe URL or an explicit full TrustURL value")
		}
		return nil
	}
	prefix := a.attrDynamicPrefix
	allLiteral := a.attrLiteral.String()
	suffix := strings.TrimPrefix(allLiteral, prefix)
	if prefix == "" && (len(a.attrDynamicNodes) != 1 || suffix != "") {
		return a.problem(position, "HIM1341", "a dynamic URL without a static safe prefix must occupy the entire quoted attribute")
	}
	if prefix != "" && !safeURLPrefix(prefix) {
		return a.problem(position, "HIM1342", "mixed static/dynamic URL attributes require a relative or explicit safe-scheme static prefix")
	}
	return nil
}

func safeStaticURL(value string) bool {
	decoded := strings.TrimSpace(html.UnescapeString(value))
	if decoded == "" {
		return true
	}
	for _, r := range decoded {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	lower := strings.ToLower(decoded)
	if strings.HasPrefix(lower, "/") || strings.HasPrefix(lower, "./") || strings.HasPrefix(lower, "../") || strings.HasPrefix(lower, "#") || strings.HasPrefix(lower, "?") {
		return true
	}
	colon := strings.IndexByte(lower, ':')
	boundary := len(lower)
	for _, separator := range []byte{'/', '?', '#'} {
		if index := strings.IndexByte(lower, separator); index >= 0 && index < boundary {
			boundary = index
		}
	}
	if colon < 0 || colon > boundary {
		return true
	}
	scheme := lower[:colon+1]
	return scheme == "http:" || scheme == "https:" || scheme == "mailto:" || scheme == "tel:"
}

func safeURLPrefix(prefix string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(prefix))
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "./") || strings.HasPrefix(trimmed, "../") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "?") {
		return true
	}
	for _, scheme := range []string{"http:", "https:", "mailto:", "tel:"} {
		if strings.HasPrefix(trimmed, scheme) {
			return true
		}
	}
	return false
}

func (a *contextAnalyzer) finishOpenTag(position sourcePosition) *Diagnostic {
	if a.currentTag == "" {
		return a.problem(position, "HIM1350", "empty HTML tag name")
	}
	if a.currentTag == "svg" || a.currentTag == "math" {
		return a.problem(position, "HIM1355", fmt.Sprintf("foreign-content element <%s> is not supported by the v1 HTML context analyzer", a.currentTag))
	}
	if a.currentTag == "plaintext" || a.currentTag == "noscript" {
		return a.problem(position, "HIM1356", fmt.Sprintf("HTML element <%s> has environment-dependent or non-terminating parsing and is not supported in v1", a.currentTag))
	}
	if a.currentTag == "meta" && strings.EqualFold(strings.TrimSpace(a.attributes["http-equiv"]), "refresh") {
		return a.problem(position, "HIM1346", "meta refresh is an unsupported navigation context in v1")
	}
	if a.selfClosing && !voidElements[a.currentTag] {
		return a.problem(position, "HIM1354", fmt.Sprintf("self-closing syntax is not valid for non-void HTML element <%s>; use an explicit closing tag", a.currentTag))
	}
	if !a.selfClosing && !voidElements[a.currentTag] {
		a.stack = append(a.stack, a.currentTag)
	}
	if !a.selfClosing && (a.currentTag == "script" || a.currentTag == "style" || a.currentTag == "textarea" || a.currentTag == "title" || a.currentTag == "iframe" || a.currentTag == "noembed" || a.currentTag == "noframes" || a.currentTag == "xmp") {
		a.rawTag = a.currentTag
		a.scriptTail = ""
		a.state = htmlRawText
	} else {
		a.state = htmlData
	}
	a.currentAttr = ""
	return nil
}

func (a *contextAnalyzer) finishCloseTag(position sourcePosition) *Diagnostic {
	if voidElements[a.currentTag] {
		return a.problem(position, "HIM1351", fmt.Sprintf("void element <%s> cannot have a closing tag", a.currentTag))
	}
	if len(a.stack) == 0 || a.stack[len(a.stack)-1] != a.currentTag {
		expected := "no closing tag"
		if len(a.stack) != 0 {
			expected = fmt.Sprintf("</%s>", a.stack[len(a.stack)-1])
		}
		return a.problem(position, "HIM1352", fmt.Sprintf("unbalanced closing tag </%s>; expected %s", a.currentTag, expected))
	}
	a.stack = a.stack[:len(a.stack)-1]
	a.state = htmlData
	a.currentTag = ""
	return nil
}

func (a *contextAnalyzer) finishRawClose(position sourcePosition) *Diagnostic {
	if len(a.stack) == 0 || a.stack[len(a.stack)-1] != a.rawTag {
		return a.problem(position, "HIM1353", fmt.Sprintf("unbalanced </%s>", a.rawTag))
	}
	a.stack = a.stack[:len(a.stack)-1]
	a.state = htmlData
	a.currentTag = ""
	a.rawTag = ""
	a.rawEndName = ""
	a.scriptTail = ""
	return nil
}

func (a *contextAnalyzer) dynamicContext() (Context, bool) {
	switch a.state {
	case htmlData:
		return ContextHTMLText, true
	case htmlAttributeDoubleQuoted, htmlAttributeSingleQuoted:
		if urlAttributes[a.currentAttr] {
			return ContextURL, true
		}
		return ContextAttr, true
	case htmlRawText:
		if a.rawTag == "script" {
			return ContextJS, true
		}
		if a.rawTag == "style" {
			return ContextCSS, true
		}
		if a.rawTag == "textarea" || a.rawTag == "title" {
			return ContextRCDATA, true
		}
	}
	return ContextNone, false
}

func (a *contextAnalyzer) inAttribute() bool {
	return a.state == htmlAttributeDoubleQuoted || a.state == htmlAttributeSingleQuoted
}

func (a *contextAnalyzer) problem(position sourcePosition, code, message string) *Diagnostic {
	d := diagnostic(a.file.Path, position, code, message)
	return &d
}

func isASCIILetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isTagNameStart(b byte) bool {
	return isASCIILetter(b)
}

func isTagNameChar(b byte) bool {
	return isASCIILetter(b) || b >= '0' && b <= '9' || b == ':' || b == '-'
}

func isAttributeNameStart(b byte) bool {
	return isASCIILetter(b) || b == '_' || b == ':'
}

func isAttributeNameChar(b byte) bool {
	return isAttributeNameStart(b) || b >= '0' && b <= '9' || b == '-' || b == '.'
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f'
}
