// SPDX-License-Identifier: AGPL-3.0-only

package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	return len(value) - 1, nil
}

func TestProtocolFramingRoundTripAndLimits(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	writer := &frameWriter{output: &output}
	if err := writer.write(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: map[string]bool{"ok": true}}); err != nil {
		t.Fatal(err)
	}
	payload, err := newFrameReader(&output).read()
	if err != nil {
		t.Fatal(err)
	}
	var decoded rpcMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded.ID) != "1" {
		t.Fatalf("response ID = %s", decoded.ID)
	}

	invalid := "Content-Length: 999999999\r\n\r\n"
	if _, err := newFrameReader(strings.NewReader(invalid)).read(); err == nil {
		t.Fatal("excessive frame length was accepted")
	}
	duplicate := "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}"
	if _, err := newFrameReader(strings.NewReader(duplicate)).read(); err == nil {
		t.Fatal("duplicate Content-Length was accepted")
	}
	var excessive bytes.Buffer
	for range maxHeaderLines + 1 {
		excessive.WriteString("X-Test: value\r\n")
	}
	excessive.WriteString("Content-Length: 2\r\n\r\n{}")
	if _, err := newFrameReader(&excessive).read(); err == nil {
		t.Fatal("excessive header count was accepted")
	}
	if err := (&frameWriter{output: shortWriter{}}).write(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Result: true}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short writer error = %v, want io.ErrShortWrite", err)
	}
}

func TestMalformedJSONProducesProtocolErrorAndContinues(t *testing.T) {
	t.Parallel()
	input := bytes.NewBuffer(nil)
	input.WriteString("Content-Length: 1\r\n\r\n{")
	exit, err := json.Marshal(rpcMessage{JSONRPC: "2.0", Method: "exit"})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(input, "Content-Length: %d\r\n\r\n", len(exit))
	input.Write(exit)
	var output bytes.Buffer
	if err := Run(context.Background(), Options{Input: input, Output: &output}); err != nil {
		t.Fatal(err)
	}
	payload, err := newFrameReader(&output).read()
	if err != nil {
		t.Fatal(err)
	}
	var response rpcMessage
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if response.Error == nil || response.Error.Code != errParse {
		t.Fatalf("parse error response = %#v", response)
	}
}

func TestRequestIDValidation(t *testing.T) {
	t.Parallel()
	for _, id := range []string{`1`, `-2`, `"request"`} {
		if !validRequestID(json.RawMessage(id)) {
			t.Errorf("valid request ID rejected: %s", id)
		}
	}
	for _, id := range []string{`null`, `true`, `1.5`, `{}`, `[]`, `not-json`} {
		if validRequestID(json.RawMessage(id)) {
			t.Errorf("invalid request ID accepted: %s", id)
		}
	}
}

func TestUTF16PositionsWithUnicodeAndCRLF(t *testing.T) {
	t.Parallel()
	text := []byte("a😀b\r\n雪c\n")
	tests := []struct {
		offset   int
		position Position
	}{
		{offset: 0, position: Position{Line: 0, Character: 0}},
		{offset: 1, position: Position{Line: 0, Character: 1}},
		{offset: 5, position: Position{Line: 0, Character: 3}},
		{offset: 8, position: Position{Line: 1, Character: 0}},
		{offset: 11, position: Position{Line: 1, Character: 1}},
	}
	for _, test := range tests {
		if got := offsetToPosition(text, test.offset); got != test.position {
			t.Errorf("offsetToPosition(%d) = %#v, want %#v", test.offset, got, test.position)
		}
		if got, ok := positionToOffset(text, test.position); !ok || got != test.offset {
			t.Errorf("positionToOffset(%#v) = %d, %v; want %d, true", test.position, got, ok, test.offset)
		}
	}
	if _, ok := positionToOffset(text, Position{Line: 0, Character: 2}); ok {
		t.Fatal("position inside UTF-16 surrogate pair was accepted")
	}
}

func FuzzFrameReaderNeverPanics(f *testing.F) {
	f.Add([]byte("Content-Length: 2\r\n\r\n{}"))
	f.Add([]byte("Content-Length: nope\r\n\r\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		_, _ = newFrameReader(bytes.NewReader(data)).read()
	})
}

func FuzzDocumentPositionNeverPanics(f *testing.F) {
	f.Add("hello 😀\r\nworld", 0, 7)
	f.Add("雪", 0, 1)
	f.Fuzz(func(t *testing.T, text string, line, character int) {
		if len(text) > 64<<10 || line < -10000 || line > 10000 || character < -10000 || character > 100000 {
			t.Skip()
		}
		offset, ok := positionToOffset([]byte(text), Position{Line: line, Character: character})
		if ok {
			_ = offsetToPosition([]byte(text), offset)
		}
	})
}
