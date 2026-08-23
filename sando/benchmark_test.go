// SPDX-License-Identifier: Apache-2.0

package sando

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"testing"
)

type benchmarkView struct {
	Title string
	URL   string
	Admin bool
	Items []string
}

var (
	benchmarkContext = context.Background()
	benchmarkData    = benchmarkView{
		Title: `A typed <view> & its "output"`,
		URL:   "/projects/sandwich-hime/?from=benchmark&mode=equivalent",
		Admin: true,
		Items: []string{"compiler", "runtime", "language server", "editor tooling"},
	}
	benchmarkHTMLTemplate = template.Must(template.New("v1-corpus").Parse(`<article data-title="{{.Title}}"><h1>{{.Title}}</h1>{{if .Admin}}<strong>Admin</strong>{{end}}<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul><a href="{{.URL}}">Open</a></article>`))
)

func benchmarkSandoComponent(view benchmarkView) Component {
	return ComponentFunc(func(_ context.Context, writer io.Writer) error {
		if err := WriteString(writer, `<article data-title="`); err != nil {
			return err
		}
		if err := WriteAttr(writer, view.Title); err != nil {
			return err
		}
		if err := WriteString(writer, `"><h1>`); err != nil {
			return err
		}
		if err := WriteText(writer, view.Title); err != nil {
			return err
		}
		if err := WriteString(writer, `</h1>`); err != nil {
			return err
		}
		if view.Admin {
			if err := WriteString(writer, `<strong>Admin</strong>`); err != nil {
				return err
			}
		}
		if err := WriteString(writer, `<ul>`); err != nil {
			return err
		}
		for _, item := range view.Items {
			if err := WriteString(writer, `<li>`); err != nil {
				return err
			}
			if err := WriteText(writer, item); err != nil {
				return err
			}
			if err := WriteString(writer, `</li>`); err != nil {
				return err
			}
		}
		if err := WriteString(writer, `</ul><a href="`); err != nil {
			return err
		}
		if err := WriteURL(writer, view.URL); err != nil {
			return err
		}
		return WriteString(writer, `">Open</a></article>`)
	})
}

func TestBenchmarkCorpusEquivalent(t *testing.T) {
	t.Parallel()
	var himeOutput bytes.Buffer
	if err := Render(benchmarkContext, &himeOutput, benchmarkSandoComponent(benchmarkData)); err != nil {
		t.Fatal(err)
	}
	var standardOutput bytes.Buffer
	if err := benchmarkHTMLTemplate.Execute(&standardOutput, benchmarkData); err != nil {
		t.Fatal(err)
	}
	if himeOutput.String() != standardOutput.String() {
		t.Fatalf("benchmark corpus is not output-equivalent\nhtml/template: %q\nSandwich Hime: %q", standardOutput.String(), himeOutput.String())
	}
}

func BenchmarkV1CorpusSandwichHime(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(renderedBenchmarkSize(b)))
	for b.Loop() {
		if err := Render(benchmarkContext, io.Discard, benchmarkSandoComponent(benchmarkData)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkV1CorpusHTMLTemplate(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(renderedBenchmarkSize(b)))
	for b.Loop() {
		if err := benchmarkHTMLTemplate.Execute(io.Discard, benchmarkData); err != nil {
			b.Fatal(err)
		}
	}
}

func renderedBenchmarkSize(tb testing.TB) int {
	tb.Helper()
	var output bytes.Buffer
	if err := Render(benchmarkContext, &output, benchmarkSandoComponent(benchmarkData)); err != nil {
		tb.Fatal(err)
	}
	return output.Len()
}
