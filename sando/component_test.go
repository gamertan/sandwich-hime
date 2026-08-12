// SPDX-License-Identifier: Apache-2.0

package sando_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"gamertan.com/sandwich-hime/sando"
)

func TestRender(t *testing.T) {
	t.Parallel()

	component := sando.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, ctx.Value(contextKey{}).(string))
		return err
	})
	ctx := context.WithValue(context.Background(), contextKey{}, "rendered")
	var output bytes.Buffer

	if err := sando.Render(ctx, &output, component); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got, want := output.String(), "rendered"; got != want {
		t.Fatalf("Render() output = %q, want %q", got, want)
	}
}

func TestRenderPropagatesComponentError(t *testing.T) {
	t.Parallel()

	want := errors.New("render failed")
	component := sando.ComponentFunc(func(context.Context, io.Writer) error { return want })
	if got := sando.Render(context.Background(), io.Discard, component); !errors.Is(got, want) {
		t.Fatalf("Render() error = %v, want %v", got, want)
	}
}

func TestRenderRejectsNilInputs(t *testing.T) {
	t.Parallel()

	valid := sando.ComponentFunc(func(context.Context, io.Writer) error { return nil })
	var nilFunc sando.ComponentFunc
	var nilPointer *pointerComponent
	var nilWriter *bytes.Buffer

	tests := []struct {
		name      string
		ctx       context.Context
		writer    io.Writer
		component sando.Component
		want      error
	}{
		{name: "nil context", writer: io.Discard, component: valid, want: sando.ErrNilContext},
		{name: "nil writer", ctx: context.Background(), component: valid, want: sando.ErrNilWriter},
		{name: "typed nil writer", ctx: context.Background(), writer: nilWriter, component: valid, want: sando.ErrNilWriter},
		{name: "nil component", ctx: context.Background(), writer: io.Discard, want: sando.ErrNilComponent},
		{name: "nil component func", ctx: context.Background(), writer: io.Discard, component: nilFunc, want: sando.ErrNilComponent},
		{name: "typed nil component", ctx: context.Background(), writer: io.Discard, component: nilPointer, want: sando.ErrNilComponent},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := sando.Render(test.ctx, test.writer, test.component); !errors.Is(got, test.want) {
				t.Fatalf("Render() error = %v, want %v", got, test.want)
			}
		})
	}
}

func TestComponentFuncDirectValidation(t *testing.T) {
	t.Parallel()

	var nilFunc sando.ComponentFunc
	if got := nilFunc.Render(context.Background(), io.Discard); !errors.Is(got, sando.ErrNilComponent) {
		t.Fatalf("nil ComponentFunc.Render() error = %v", got)
	}

	valid := sando.ComponentFunc(func(context.Context, io.Writer) error { return nil })
	if got := valid.Render(nil, io.Discard); !errors.Is(got, sando.ErrNilContext) {
		t.Fatalf("ComponentFunc.Render(nil, writer) error = %v", got)
	}
	if got := valid.Render(context.Background(), nil); !errors.Is(got, sando.ErrNilWriter) {
		t.Fatalf("ComponentFunc.Render(ctx, nil) error = %v", got)
	}
}

type contextKey struct{}

type pointerComponent struct{}

func (*pointerComponent) Render(context.Context, io.Writer) error { return nil }
