// SPDX-License-Identifier: AGPL-3.0-only

package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
)

const (
	maxMessageBytes = 16 << 20
	maxHeaderBytes  = 64 << 10
	maxHeaderLines  = 64
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func validRequestID(id json.RawMessage) bool {
	if len(id) == 0 {
		return true
	}
	var value any
	if err := json.Unmarshal(id, &value); err != nil {
		return false
	}
	switch value := value.(type) {
	case string:
		return true
	case float64:
		return value == math.Trunc(value)
	default:
		return false
	}
}

const (
	errParse            = -32700
	errInvalidRequest   = -32600
	errMethodNotFound   = -32601
	errInvalidParams    = -32602
	errInternal         = -32603
	errRequestCancelled = -32800
)

type frameReader struct{ reader *bufio.Reader }

func newFrameReader(input io.Reader) *frameReader {
	return &frameReader{reader: bufio.NewReaderSize(input, 64<<10)}
}

func (reader *frameReader) read() ([]byte, error) {
	contentLength := -1
	headerBytes := 0
	headerLines := 0
	for {
		line, err := reader.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		headerBytes += len(line)
		headerLines++
		if headerBytes > maxHeaderBytes || headerLines > maxHeaderLines {
			return nil, errors.New("LSP headers exceed configured limits")
		}
		if len(line) > 8<<10 {
			return nil, errors.New("LSP header line exceeds 8 KiB")
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, errors.New("malformed LSP header")
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if contentLength >= 0 {
				return nil, errors.New("duplicate Content-Length header")
			}
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(value))
			if parseErr != nil || parsed < 0 || parsed > maxMessageBytes {
				return nil, errors.New("invalid or excessive Content-Length")
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, errors.New("missing Content-Length header")
	}
	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(reader.reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type frameWriter struct {
	mu     sync.Mutex
	output io.Writer
}

func (writer *frameWriter) write(message rpcMessage) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	var frame bytes.Buffer
	fmt.Fprintf(&frame, "Content-Length: %d\r\n\r\n", len(payload))
	frame.Write(payload)
	writer.mu.Lock()
	defer writer.mu.Unlock()
	written, err := writer.output.Write(frame.Bytes())
	if err == nil && written != frame.Len() {
		return io.ErrShortWrite
	}
	return err
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type lspDiagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}
