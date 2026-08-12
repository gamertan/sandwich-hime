// SPDX-License-Identifier: Apache-2.0

package sando

import (
	"errors"
	"fmt"
	"html"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrUnsafeURL is the sentinel wrapped by URLSafetyError when an ordinary URL
// value is ambiguous or uses a scheme outside the v1 allowlist.
var ErrUnsafeURL = errors.New("sando: unsafe URL")

// URLSafetyError reports why an ordinary URL was rejected. It intentionally
// does not retain or print the complete value, which may contain sensitive
// application data.
type URLSafetyError struct {
	Scheme string
	Reason string
}

// Error implements error.
func (e *URLSafetyError) Error() string {
	if e.Scheme != "" {
		return fmt.Sprintf("%v: scheme %q is not allowed", ErrUnsafeURL, e.Scheme)
	}
	if e.Reason != "" {
		return fmt.Sprintf("%v: %s", ErrUnsafeURL, e.Reason)
	}
	return ErrUnsafeURL.Error()
}

// Unwrap permits errors.Is(err, ErrUnsafeURL).
func (e *URLSafetyError) Unwrap() error { return ErrUnsafeURL }

// WriteString writes a compiler-owned static literal and reports both writer
// errors and contract-violating short writes. Application data must use the
// context-specific helpers below instead.
func WriteString(w io.Writer, value string) error {
	return writeString(w, value)
}

// WriteText writes value escaped for an HTML text node.
func WriteText(w io.Writer, value any) error {
	if trusted, ok := value.(TrustedHTML); ok {
		return writeString(w, trusted.value)
	}
	return writeString(w, html.EscapeString(normalizeText(stringValue(value))))
}

// WriteRCDATA writes value escaped for an HTML RCDATA element such as title
// or textarea. Trusted wrappers are intentionally not honored in this context:
// their contents are escaped like every other value so they cannot close the
// containing element.
func WriteRCDATA(w io.Writer, value any) error {
	return writeString(w, html.EscapeString(normalizeText(stringValue(value))))
}

// WriteAttr writes value escaped for a quoted HTML attribute. Generated code
// must always place this output inside a syntactically complete quoted value.
func WriteAttr(w io.Writer, value any) error {
	return writeString(w, html.EscapeString(normalizeText(stringValue(value))))
}

// WriteURL writes value escaped for a quoted URL-bearing HTML attribute.
// Ordinary values are canonicalized and checked before any bytes are written.
// TrustedURL bypasses the scheme check but never attribute escaping.
func WriteURL(w io.Writer, value any) error {
	if trusted, ok := value.(TrustedURL); ok {
		return writeString(w, html.EscapeString(normalizeText(trusted.value)))
	}

	canonical, err := canonicalURL(stringValue(value))
	if err != nil {
		return err
	}
	return writeString(w, html.EscapeString(canonical))
}

// WriteHTML writes deliberately trusted HTML without escaping.
func WriteHTML(w io.Writer, value TrustedHTML) error {
	return writeString(w, value.value)
}

// WriteJS writes deliberately trusted JavaScript without escaping.
func WriteJS(w io.Writer, value TrustedJS) error {
	return writeString(w, value.value)
}

// WriteCSS writes deliberately trusted CSS without escaping.
func WriteCSS(w io.Writer, value TrustedCSS) error {
	return writeString(w, value.value)
}

func writeString(w io.Writer, value string) error {
	if isNil(w) {
		return ErrNilWriter
	}

	n, err := io.WriteString(w, value)
	if err != nil {
		return err
	}
	if n != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func stringValue(value any) string {
	switch value := value.(type) {
	case TrustedHTML:
		return value.value
	case TrustedURL:
		return value.value
	case TrustedJS:
		return value.value
	case TrustedCSS:
		return value.value
	default:
		return fmt.Sprint(value)
	}
}

func normalizeText(value string) string {
	if !utf8.ValidString(value) {
		value = strings.ToValidUTF8(value, "\uFFFD")
	}
	return strings.ReplaceAll(value, "\x00", "\uFFFD")
}

func canonicalURL(value string) (string, error) {
	value = normalizeText(value)
	value = strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\uFEFF'
	})

	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", &URLSafetyError{Reason: "control characters are not allowed"}
		}
	}

	colon := strings.IndexByte(value, ':')
	boundary := strings.IndexAny(value, "/?#")
	if colon < 0 || boundary >= 0 && boundary < colon {
		return value, nil
	}

	scheme := value[:colon]
	if !validScheme(scheme) {
		return "", &URLSafetyError{Reason: "ambiguous scheme syntax"}
	}

	scheme = strings.ToLower(scheme)
	switch scheme {
	case "http", "https", "mailto", "tel":
		return value, nil
	default:
		return "", &URLSafetyError{Scheme: scheme}
	}
}

func validScheme(value string) bool {
	if value == "" || !isASCIIAlpha(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		c := value[i]
		if !isASCIIAlpha(c) && (c < '0' || c > '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func isASCIIAlpha(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
