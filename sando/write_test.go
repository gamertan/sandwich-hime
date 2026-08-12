// SPDX-License-Identifier: Apache-2.0

package sando_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"gamertan.com/sandwich-hime/sando"
)

func TestWriteText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "quote and entity injection", value: `<script x="'&">alert(1)</script>`, want: `&lt;script x=&#34;&#39;&amp;&#34;&gt;alert(1)&lt;/script&gt;`},
		{name: "unicode preserved", value: "姫 🍞 café", want: "姫 🍞 café"},
		{name: "NUL replaced", value: "left\x00right", want: "left\uFFFDright"},
		{name: "invalid UTF-8 replaced", value: string([]byte{'a', 0xff, 'b'}), want: "a\uFFFDb"},
		{name: "non-string formatted", value: 42, want: "42"},
		{name: "trusted HTML is deliberately raw in text context", value: sando.TrustHTML("<b>explicitly trusted</b>"), want: "<b>explicitly trusted</b>"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := sando.WriteText(&output, test.value); err != nil {
				t.Fatalf("WriteText() error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Fatalf("WriteText() = %q, want %q", got, test.want)
			}
			if !utf8.ValidString(output.String()) {
				t.Fatal("WriteText() emitted invalid UTF-8")
			}
		})
	}
}

func TestWriteAttr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "breakout characters", value: `x" autofocus onfocus="alert(1)&`, want: `x&#34; autofocus onfocus=&#34;alert(1)&amp;`},
		{name: "angle and apostrophe", value: `<'value'>`, want: `&lt;&#39;value&#39;&gt;`},
		{name: "unicode and NUL", value: "姫\x00さん", want: "姫\uFFFDさん"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := sando.WriteAttr(&output, test.value); err != nil {
				t.Fatalf("WriteAttr() error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Fatalf("WriteAttr() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteRCDATAAlwaysEscapesTrustedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "plain", value: `</title><script>alert("x")</script>`, want: `&lt;/title&gt;&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;`},
		{name: "trusted HTML", value: sando.TrustHTML(`</textarea><script>alert(1)</script>`), want: `&lt;/textarea&gt;&lt;script&gt;alert(1)&lt;/script&gt;`},
		{name: "trusted URL", value: sando.TrustURL(`javascript:</title>`), want: `javascript:&lt;/title&gt;`},
		{name: "trusted JavaScript", value: sando.TrustJS(`</title><script>alert(1)</script>`), want: `&lt;/title&gt;&lt;script&gt;alert(1)&lt;/script&gt;`},
		{name: "trusted CSS", value: sando.TrustCSS(`</textarea><style>*{display:none}</style>`), want: `&lt;/textarea&gt;&lt;style&gt;*{display:none}&lt;/style&gt;`},
		{name: "unicode and NUL", value: "姫\x00さん", want: "姫\uFFFDさん"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := sando.WriteRCDATA(&output, test.value); err != nil {
				t.Fatalf("WriteRCDATA() error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Fatalf("WriteRCDATA() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteRCDATAAlwaysEscapesTrustedHTML(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	value := sando.TrustHTML(`</textarea><script>alert(1)</script>`)
	if err := sando.WriteRCDATA(&output, value); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), `&lt;/textarea&gt;&lt;script&gt;alert(1)&lt;/script&gt;`; got != want {
		t.Fatalf("WriteRCDATA() = %q, want %q", got, want)
	}
}

func TestWriteURLAllowsAndCanonicalizesOrdinaryValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "root relative", value: "/items?q=tea&sort=name", want: "/items?q=tea&amp;sort=name"},
		{name: "path relative", value: "../images/姫.png", want: "../images/姫.png"},
		{name: "fragment", value: "#section", want: "#section"},
		{name: "network path", value: "//static.example/assets", want: "//static.example/assets"},
		{name: "HTTP scheme case insensitive", value: "HTTP://example.test/a", want: "HTTP://example.test/a"},
		{name: "HTTPS", value: "https://example.test/", want: "https://example.test/"},
		{name: "mail", value: "mailto:hime@example.test", want: "mailto:hime@example.test"},
		{name: "telephone", value: "tel:+14165550123", want: "tel:+14165550123"},
		{name: "surrounding whitespace trimmed", value: " \n\thttps://example.test/path\r ", want: "https://example.test/path"},
		{name: "colon after query is relative", value: "/search?q=kind:value", want: "/search?q=kind:value"},
		{name: "empty", value: "", want: ""},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := sando.WriteURL(&output, test.value); err != nil {
				t.Fatalf("WriteURL() error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Fatalf("WriteURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteURLRejectsDangerousAndAmbiguousValuesBeforeWriting(t *testing.T) {
	t.Parallel()

	values := []string{
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:msgbox(1)",
		"file:///etc/passwd",
		"ftp://example.test/file",
		"java\nscript:alert(1)",
		"java\tscript:alert(1)",
		"java\x00script:alert(1)",
		"javascript\x7f:alert(1)",
		"java script:alert(1)",
		"%6aavascript:alert(1)",
		":ambiguous",
	}

	for _, value := range values {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			output.WriteString("last-good")
			err := sando.WriteURL(&output, value)
			if !errors.Is(err, sando.ErrUnsafeURL) {
				t.Fatalf("WriteURL() error = %v, want ErrUnsafeURL", err)
			}
			if got := output.String(); got != "last-good" {
				t.Fatalf("WriteURL() modified writer on validation failure: %q", got)
			}
		})
	}
}

func TestTrustedWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		write func(io.Writer) error
		want  string
	}{
		{name: "HTML", write: func(w io.Writer) error {
			return sando.WriteHTML(w, sando.TrustHTML(`<strong data-x="1">Hime</strong>`))
		}, want: `<strong data-x="1">Hime</strong>`},
		{name: "URL bypasses scheme but not attribute escaping", write: func(w io.Writer) error { return sando.WriteURL(w, sando.TrustURL(`custom:"<&`)) }, want: `custom:&#34;&lt;&amp;`},
		{name: "JavaScript", write: func(w io.Writer) error { return sando.WriteJS(w, sando.TrustJS(`window.hime = "<3";`)) }, want: `window.hime = "<3";`},
		{name: "CSS", write: func(w io.Writer) error { return sando.WriteCSS(w, sando.TrustCSS(`.hime::after { content: "<3"; }`)) }, want: `.hime::after { content: "<3"; }`},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := test.write(&output); err != nil {
				t.Fatalf("trusted write error = %v", err)
			}
			if got := output.String(); got != test.want {
				t.Fatalf("trusted write = %q, want %q", got, test.want)
			}
		})
	}
}

func TestWriteHelpersPropagateWriterFailures(t *testing.T) {
	t.Parallel()

	want := errors.New("disk full")
	tests := []struct {
		name  string
		write func(io.Writer) error
	}{
		{name: "static literal", write: func(w io.Writer) error { return sando.WriteString(w, "hello") }},
		{name: "text", write: func(w io.Writer) error { return sando.WriteText(w, "hello") }},
		{name: "RCDATA", write: func(w io.Writer) error { return sando.WriteRCDATA(w, "hello") }},
		{name: "attribute", write: func(w io.Writer) error { return sando.WriteAttr(w, "hello") }},
		{name: "URL", write: func(w io.Writer) error { return sando.WriteURL(w, "/hello") }},
		{name: "HTML", write: func(w io.Writer) error { return sando.WriteHTML(w, sando.TrustHTML("hello")) }},
		{name: "JavaScript", write: func(w io.Writer) error { return sando.WriteJS(w, sando.TrustJS("hello")) }},
		{name: "CSS", write: func(w io.Writer) error { return sando.WriteCSS(w, sando.TrustCSS("hello")) }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.write(errorWriter{err: want}); !errors.Is(got, want) {
				t.Fatalf("write error = %v, want %v", got, want)
			}
			if got := test.write(shortWriter{}); !errors.Is(got, io.ErrShortWrite) {
				t.Fatalf("short write error = %v, want io.ErrShortWrite", got)
			}
			if got := test.write(nil); !errors.Is(got, sando.ErrNilWriter) {
				t.Fatalf("nil writer error = %v, want ErrNilWriter", got)
			}
		})
	}
}

func TestURLSafetyErrorDoesNotEchoSensitiveValue(t *testing.T) {
	t.Parallel()

	const secret = "user:password@example.test"
	err := sando.WriteURL(io.Discard, "custom:"+secret)
	if err == nil {
		t.Fatal("WriteURL() unexpectedly accepted custom scheme")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaks URL contents: %q", err)
	}
	var safetyError *sando.URLSafetyError
	if !errors.As(err, &safetyError) {
		t.Fatalf("error type = %T, want *sando.URLSafetyError", err)
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	return len(value) - 1, nil
}
