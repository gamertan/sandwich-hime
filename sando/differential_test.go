// SPDX-License-Identifier: Apache-2.0

package sando

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHTMLTemplateDifferentialCorpus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
		prefix string
		suffix string
		write  func(*bytes.Buffer, any) error
		values []string
	}{
		{
			name: "HTML text", source: `<p>{{.}}</p>`, prefix: `<p>`, suffix: `</p>`,
			write:  func(output *bytes.Buffer, value any) error { return WriteText(output, value) },
			values: differentialTextValues(),
		},
		{
			name: "quoted attribute", source: `<p title="{{.}}">x</p>`, prefix: `<p title="`, suffix: `">x</p>`,
			write:  func(output *bytes.Buffer, value any) error { return WriteAttr(output, value) },
			values: differentialTextValues(),
		},
		{
			name: "RCDATA", source: `<textarea>{{.}}</textarea>`, prefix: `<textarea>`, suffix: `</textarea>`,
			write:  func(output *bytes.Buffer, value any) error { return WriteRCDATA(output, value) },
			values: differentialTextValues(),
		},
		{
			name: "safe URL", source: `<a href="{{.}}">x</a>`, prefix: `<a href="`, suffix: `">x</a>`,
			write:  func(output *bytes.Buffer, value any) error { return WriteURL(output, value) },
			values: []string{"", "/", "./relative", "?q=a&next=b", "#section", "https://example.test/a?x=1&y=2", "HTTP://example.test/", "mailto:reader@example.test"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			parsed := template.Must(template.New(test.name).Parse(test.source))
			for _, value := range test.values {
				var baseline bytes.Buffer
				if err := parsed.Execute(&baseline, value); err != nil {
					t.Fatalf("html/template value %q: %v", value, err)
				}
				var output bytes.Buffer
				output.WriteString(test.prefix)
				if err := test.write(&output, value); err != nil {
					t.Fatalf("Sandwich Hime value %q: %v", value, err)
				}
				output.WriteString(test.suffix)
				if output.String() != baseline.String() {
					t.Fatalf("differential mismatch for %q\nhtml/template: %q\nSandwich Hime: %q", value, baseline.String(), output.String())
				}
			}
		})
	}
}

func TestHTMLTemplateDifferentialUnsafeURLPolicy(t *testing.T) {
	t.Parallel()
	parsed := template.Must(template.New("url").Parse(`<a href="{{.}}">x</a>`))
	values := []string{
		"javascript:alert(1)",
		" JAVASCRIPT:alert(1) ",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox(1)",
		"unknown:opaque",
		"java%73cript:alert(1)",
	}
	for _, value := range values {
		var baseline bytes.Buffer
		if err := parsed.Execute(&baseline, value); err != nil {
			t.Fatalf("html/template value %q: %v", value, err)
		}
		if !strings.Contains(baseline.String(), "#ZgotmplZ") {
			t.Fatalf("html/template did not block corpus URL %q: %q", value, baseline.String())
		}
		var output bytes.Buffer
		err := WriteURL(&output, value)
		if !errors.Is(err, ErrUnsafeURL) {
			t.Fatalf("Sandwich Hime accepted corpus URL %q: output=%q err=%v", value, output.String(), err)
		}
		if output.Len() != 0 {
			t.Fatalf("Sandwich Hime wrote bytes before rejecting %q: %q", value, output.String())
		}
	}
}

func TestHTMLTemplateDifferentialDocumentedStrictness(t *testing.T) {
	t.Parallel()

	t.Run("invalid UTF-8", func(t *testing.T) {
		value := "invalid UTF-8: \xff:end"
		parsed := template.Must(template.New("text").Parse(`<p>{{.}}</p>`))
		var baseline bytes.Buffer
		if err := parsed.Execute(&baseline, value); err != nil {
			t.Fatal(err)
		}
		if utf8.Valid(baseline.Bytes()) {
			t.Fatalf("baseline unexpectedly normalized invalid UTF-8: %q", baseline.Bytes())
		}
		var output bytes.Buffer
		if err := WriteText(&output, value); err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(output.Bytes()) || !strings.Contains(output.String(), "\uFFFD") {
			t.Fatalf("Sandwich Hime did not normalize invalid UTF-8: %q", output.Bytes())
		}
	})

	t.Run("control in otherwise allowed URL", func(t *testing.T) {
		value := "https:\n//example.test/"
		parsed := template.Must(template.New("url").Parse(`<a href="{{.}}">x</a>`))
		var baseline bytes.Buffer
		if err := parsed.Execute(&baseline, value); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(baseline.String()), "%0a") {
			t.Fatalf("baseline did not visibly encode the control: %q", baseline.String())
		}
		var output bytes.Buffer
		if err := WriteURL(&output, value); !errors.Is(err, ErrUnsafeURL) {
			t.Fatalf("Sandwich Hime did not fail closed: output=%q err=%v", output.String(), err)
		}
		if output.Len() != 0 {
			t.Fatalf("Sandwich Hime wrote before rejecting the control: %q", output.String())
		}
	})

	t.Run("explicit tel allowlist", func(t *testing.T) {
		value := "tel:+15555550100"
		parsed := template.Must(template.New("url").Parse(`<a href="{{.}}">x</a>`))
		var baseline bytes.Buffer
		if err := parsed.Execute(&baseline, value); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(baseline.String(), "#ZgotmplZ") {
			t.Fatalf("baseline URL policy changed: %q", baseline.String())
		}
		var output bytes.Buffer
		if err := WriteURL(&output, value); err != nil {
			t.Fatalf("Sandwich Hime rejected its documented tel scheme: %v", err)
		}
		if output.String() != value {
			t.Fatalf("Sandwich Hime tel output = %q", output.String())
		}
	})
}

func TestHTMLTemplateDifferentialExplicitTrustedHTML(t *testing.T) {
	t.Parallel()
	value := `<strong data-note="reviewed & trusted">ok</strong>`
	parsed := template.Must(template.New("trusted HTML").Parse(`<div>{{.}}</div>`))
	var baseline bytes.Buffer
	if err := parsed.Execute(&baseline, template.HTML(value)); err != nil { // #nosec G203 -- the test is the explicit trust-boundary comparison.
		t.Fatal(err)
	}
	var output bytes.Buffer
	output.WriteString("<div>")
	if err := WriteText(&output, TrustHTML(value)); err != nil {
		t.Fatal(err)
	}
	output.WriteString("</div>")
	if output.String() != baseline.String() {
		t.Fatalf("trusted HTML mismatch\nhtml/template: %q\nSandwich Hime: %q", baseline.String(), output.String())
	}
}

func differentialTextValues() []string {
	return []string{
		"",
		"ordinary text",
		`<script>alert("x")</script>`,
		`quotes: "double" and 'single' & ampersand`,
		"Unicode: 雪 🥪 e\u0301",
		"NUL:\x00:end",
		"line separators: \u2028\u2029",
	}
}

func FuzzWriteURLPolicy(f *testing.F) {
	for _, seed := range []string{
		"",
		"/relative?one=1&two=2",
		"https://example.test/path",
		" JAVASCRIPT:alert(1) ",
		"https:\n//example.test/",
		"tel:+15555550100",
		"invalid:\xff",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 64<<10 {
			t.Skip()
		}
		var first, second bytes.Buffer
		firstErr := WriteURL(&first, value)
		secondErr := WriteURL(&second, value)
		if first.String() != second.String() || fmt.Sprint(firstErr) != fmt.Sprint(secondErr) {
			t.Fatal("URL policy was not deterministic")
		}
		if firstErr != nil {
			if !errors.Is(firstErr, ErrUnsafeURL) || first.Len() != 0 {
				t.Fatalf("URL rejection was not fail-closed: output=%q err=%v", first.String(), firstErr)
			}
			return
		}
		if !utf8.ValidString(first.String()) || strings.ContainsAny(first.String(), "\x00\r\n") {
			t.Fatalf("accepted URL output is not valid single-line UTF-8: %q", first.String())
		}
	})
}
