// SPDX-License-Identifier: AGPL-3.0-only

package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gamertan.com/sandwich-hime/internal/compiler"
)

type protocolClient struct {
	input       *io.PipeWriter
	output      *frameReader
	nextID      int
	lastPayload []byte
}

func newProtocolClient(t *testing.T, root string) (*protocolClient, <-chan error) {
	t.Helper()
	serverInput, clientInput := io.Pipe()
	clientOutput, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), Options{Input: serverInput, Output: serverOutput, LogOutput: io.Discard, Debounce: 10 * time.Millisecond})
		_ = serverOutput.Close()
	}()
	client := &protocolClient{input: clientInput, output: newFrameReader(clientOutput)}
	response := client.call(t, "initialize", map[string]any{"rootUri": pathToURI(root)})
	if response.Error != nil {
		t.Fatalf("initialize: %#v", response.Error)
	}
	client.notify(t, "initialized", map[string]any{})
	return client, done
}

func (client *protocolClient) send(t *testing.T, message rpcMessage) {
	t.Helper()
	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	frame := append([]byte("Content-Length: "+itoa(len(payload))+"\r\n\r\n"), payload...)
	if _, err := client.input.Write(frame); err != nil {
		t.Fatal(err)
	}
}

func (client *protocolClient) call(t *testing.T, method string, params any) rpcMessage {
	t.Helper()
	client.nextID++
	id := client.nextID
	payload, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	client.send(t, rpcMessage{JSONRPC: "2.0", ID: json.RawMessage(itoa(id)), Method: method, Params: payload})
	for {
		message := client.read(t)
		if string(message.ID) == itoa(id) {
			return message
		}
	}
}

func (client *protocolClient) notify(t *testing.T, method string, params any) {
	t.Helper()
	payload, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	client.send(t, rpcMessage{JSONRPC: "2.0", Method: method, Params: payload})
}

func (client *protocolClient) read(t *testing.T) rpcMessage {
	t.Helper()
	type result struct {
		message rpcMessage
		payload []byte
		err     error
	}
	ready := make(chan result, 1)
	go func() {
		payload, err := client.output.read()
		if err != nil {
			ready <- result{err: err}
			return
		}
		var message rpcMessage
		err = json.Unmarshal(payload, &message)
		ready <- result{message: message, payload: payload, err: err}
	}()
	select {
	case got := <-ready:
		if got.err != nil {
			t.Fatal(got.err)
		}
		client.lastPayload = got.payload
		return got.message
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for language-server response")
		return rpcMessage{}
	}
}

func (client *protocolClient) waitDiagnostics(t *testing.T, uri string, wantCode string) []lspDiagnostic {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		message := client.read(t)
		if message.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var published struct {
			URI         string          `json:"uri"`
			Diagnostics []lspDiagnostic `json:"diagnostics"`
		}
		if json.Unmarshal(message.Params, &published) != nil || published.URI != uri {
			continue
		}
		if wantCode == "" {
			return published.Diagnostics
		}
		for _, item := range published.Diagnostics {
			if item.Code == wantCode {
				return published.Diagnostics
			}
		}
	}
	t.Fatalf("timed out waiting for %s diagnostic", wantCode)
	return nil
}

func TestServerOverlayFeaturesAndNoWrites(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/project\n\ngo 1.25\n")
	homePath := filepath.Join(root, "home.sando")
	badgePath := filepath.Join(root, "cards", "badge.sando")
	writeTestFile(t, homePath, "<?sando go\npackage views\nfunc Home(visitor string)\n?>\n<p><?= visitor ?></p>\n")
	writeTestFile(t, badgePath, "<?sando go\npackage cards\nfunc Badge(label string)\n?>\n<strong><?= label ?></strong>\n")

	client, done := newProtocolClient(t, root)
	homeURI := pathToURI(homePath)
	client.waitDiagnostics(t, homeURI, "")
	overlay := "<?sando go\npackage views\nimport \"example.test/project/cards\"\nfunc Home(visitor string)\n?>\n<p>😀 <?= visitor ?></p>\n<?~ cards.Badge(\"new\") ?>\n"
	client.notify(t, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": homeURI, "languageId": "sando", "version": 1, "text": overlay}})
	if diagnostics := client.waitDiagnostics(t, homeURI, ""); len(diagnostics) != 0 {
		t.Fatalf("valid overlay diagnostics = %#v", diagnostics)
	}

	completionOffset := strings.Index(overlay, "cards.Badge") + len("cards.B")
	completion := client.call(t, "textDocument/completion", textDocumentPositionParams{TextDocument: textDocumentIdentifier{URI: homeURI}, Position: offsetToPosition([]byte(overlay), completionOffset)})
	assertJSONContains(t, completion.Result, `"label":"cards.Badge"`)

	definitionOffset := strings.Index(overlay, "Badge") + 2
	definition := client.call(t, "textDocument/definition", textDocumentPositionParams{TextDocument: textDocumentIdentifier{URI: homeURI}, Position: offsetToPosition([]byte(overlay), definitionOffset)})
	assertJSONContains(t, definition.Result, pathToURI(badgePath))

	hoverOffset := strings.Index(overlay, "visitor ?></p>") + 2
	hover := client.call(t, "textDocument/hover", textDocumentPositionParams{TextDocument: textDocumentIdentifier{URI: homeURI}, Position: offsetToPosition([]byte(overlay), hoverOffset)})
	assertJSONContains(t, hover.Result, "html-text")

	symbols := client.call(t, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]string{"uri": homeURI}})
	assertJSONContains(t, symbols.Result, `"name":"Home"`)

	if err := os.Remove(badgePath); err != nil {
		t.Fatal(err)
	}
	client.notify(t, "workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{{"uri": pathToURI(badgePath), "type": 3}}})
	client.waitDiagnostics(t, pathToURI(badgePath), "")
	completion = client.call(t, "textDocument/completion", textDocumentPositionParams{TextDocument: textDocumentIdentifier{URI: homeURI}, Position: offsetToPosition([]byte(overlay), completionOffset)})
	if payload, _ := json.Marshal(completion.Result); strings.Contains(string(payload), `"label":"cards.Badge"`) {
		t.Fatalf("deleted component remained in completion index: %s", payload)
	}

	broken := strings.Replace(overlay, "<?~ cards.Badge(\"new\") ?>", "<div>", 1)
	client.notify(t, "textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": homeURI, "version": 2},
		"contentChanges": []map[string]string{{"text": broken}},
	})
	client.waitDiagnostics(t, homeURI, "HIM1311")
	if _, err := os.Stat(homePath + ".go"); !os.IsNotExist(err) {
		t.Fatalf("language server wrote generated output: %v", err)
	}

	shutdown := client.call(t, "shutdown", map[string]any{})
	if shutdown.Error != nil {
		t.Fatalf("shutdown: %#v", shutdown.Error)
	}
	if !bytes.Contains(client.lastPayload, []byte(`"result":null`)) {
		t.Fatalf("shutdown response omitted JSON-RPC null result: %s", client.lastPayload)
	}
	client.notify(t, "exit", map[string]any{})
	_ = client.input.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("language server did not exit")
	}
}

func TestOverlayHonorsNestedModuleAndSymlinkBoundaries(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/root\n")
	nestedPath := filepath.Join(root, "nested", "view.sando")
	writeTestFile(t, filepath.Join(root, "nested", "go.mod"), "module example.test/nested\n")
	writeTestFile(t, nestedPath, "<?sando go\npackage nested\nfunc View()\n?>\n<p>no</p>\n")
	server := &Server{root: root, overlays: make(map[string]document)}
	server.updateOverlay(pathToURI(nestedPath), 1, "ignored", true)
	if len(server.overlays) != 0 {
		t.Fatalf("nested-module overlay was accepted: %#v", server.overlays)
	}

	if runtime.GOOS != "windows" {
		realDirectory := filepath.Join(root, "real")
		if err := os.Mkdir(realDirectory, 0o700); err != nil {
			t.Fatal(err)
		}
		linkDirectory := filepath.Join(root, "linked")
		if err := os.Symlink(realDirectory, linkDirectory); err != nil {
			t.Fatal(err)
		}
		linkedPath := filepath.Join(linkDirectory, "view.sando")
		server.updateOverlay(pathToURI(linkedPath), 1, "ignored", true)
		if len(server.overlays) != 0 {
			t.Fatalf("symlink overlay was accepted: %#v", server.overlays)
		}
	}
}

func TestServerRejectsMultipleRootsAndCanceledRequest(t *testing.T) {
	root := t.TempDir()
	server := &Server{initialized: true, snapshot: workspaceSnapshot{documents: map[string]document{}, analyses: map[string]compiler.DocumentAnalysis{}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, rpcErr := server.handleRequest(ctx, "textDocument/completion", json.RawMessage(`{}`))
	if rpcErr == nil || rpcErr.Code != errRequestCancelled {
		t.Fatalf("canceled request error = %#v", rpcErr)
	}
	input, writer := io.Pipe()
	reader, serverOutput := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), Options{Input: input, Output: serverOutput}) }()
	client := &protocolClient{input: writer, output: newFrameReader(reader)}
	response := client.call(t, "initialize", map[string]any{"workspaceFolders": []map[string]string{{"uri": pathToURI(root)}, {"uri": pathToURI(root)}}})
	if response.Error == nil || response.Error.Code != errInvalidParams {
		t.Fatalf("multiple-root response = %#v", response)
	}
	client.notify(t, "exit", map[string]any{})
	_ = writer.Close()
	<-done
}

func TestReindexCountsOpenOverlaysInWorkspaceLimit(t *testing.T) {
	root := t.TempDir()
	server := &Server{
		root:     root,
		overlays: make(map[string]document, maxWorkspaceFiles+1),
		snapshot: workspaceSnapshot{documents: map[string]document{}, analyses: map[string]compiler.DocumentAnalysis{}},
		writer:   &frameWriter{output: io.Discard},
		logs:     io.Discard,
	}
	for index := range maxWorkspaceFiles + 1 {
		path := filepath.Join(root, "overlay-"+itoa(index)+".sando")
		server.overlays[path] = document{URI: pathToURI(path), Path: path, Text: []byte("<?sando go\npackage views\nfunc View()\n?>\n"), Open: true}
	}
	err := server.reindex(context.Background())
	if err == nil || !strings.Contains(err.Error(), "more than 10000") {
		t.Fatalf("reindex overlay limit error = %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertJSONContains(t *testing.T, value any, marker string) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), marker) {
		t.Fatalf("JSON %s does not contain %q", payload, marker)
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
