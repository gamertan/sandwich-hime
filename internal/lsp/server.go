// SPDX-License-Identifier: AGPL-3.0-only

// Package lsp implements Hime-san's read-only Language Server Protocol
// adapter. It deliberately owns no generation, Go toolchain, HTTP, network,
// or project-execution behavior.
package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gamertan.com/sandwich-hime/internal/compiler"
)

// Options configures one stdio language-server process.
type Options struct {
	Input     io.Reader
	Output    io.Writer
	LogOutput io.Writer
	Debounce  time.Duration
}

// Server serves exactly one workspace root.
type Server struct {
	context context.Context
	cancel  context.CancelFunc
	reader  *frameReader
	writer  *frameWriter
	logs    io.Writer

	mu                 sync.RWMutex
	root               string
	initialized        bool
	shutdown           bool
	overlays           map[string]document
	snapshot           workspaceSnapshot
	analysisCancel     context.CancelFunc
	analysisTimer      *time.Timer
	analysisGeneration uint64
	debounce           time.Duration
	afterFunc          func(time.Duration, func()) *time.Timer
	requests           map[string]context.CancelFunc
	wait               sync.WaitGroup
	analysisWait       sync.WaitGroup
}

// Run serves LSP JSON-RPC until the client sends exit, closes stdin, or the
// parent context is canceled.
func Run(parent context.Context, options Options) error {
	if options.Input == nil || options.Output == nil {
		return errors.New("LSP stdin and stdout are required")
	}
	if options.LogOutput == nil {
		options.LogOutput = io.Discard
	}
	if options.Debounce <= 0 {
		options.Debounce = 200 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(parent)
	server := &Server{
		context: ctx, cancel: cancel,
		reader: newFrameReader(options.Input), writer: &frameWriter{output: options.Output}, logs: options.LogOutput,
		overlays: make(map[string]document), snapshot: workspaceSnapshot{documents: make(map[string]document), analyses: make(map[string]compiler.DocumentAnalysis)},
		debounce: options.Debounce, afterFunc: time.AfterFunc, requests: make(map[string]context.CancelFunc),
	}
	defer func() {
		cancel()
		server.mu.Lock()
		server.analysisGeneration++
		if server.analysisTimer != nil {
			server.analysisTimer.Stop()
		}
		if server.analysisCancel != nil {
			server.analysisCancel()
		}
		for _, requestCancel := range server.requests {
			requestCancel()
		}
		server.mu.Unlock()
		server.wait.Wait()
		server.analysisWait.Wait()
	}()

	for {
		payload, err := server.reader.read()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read LSP frame: %w", err)
		}
		var message rpcMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			_ = server.writer.write(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: errParse, Message: "invalid JSON"}})
			continue
		}
		if message.JSONRPC != "2.0" || message.Method == "" || !validRequestID(message.ID) {
			_ = server.writer.write(rpcMessage{JSONRPC: "2.0", ID: responseID(message.ID), Error: &rpcError{Code: errInvalidRequest, Message: "invalid JSON-RPC request"}})
			continue
		}
		if len(message.ID) == 0 {
			if message.Method == "exit" {
				server.cancel()
				return nil
			}
			server.handleNotification(message.Method, message.Params)
			continue
		}
		server.startRequest(message)
	}
}

func responseID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return json.RawMessage("null")
	}
	return append(json.RawMessage(nil), id...)
}

func (server *Server) startRequest(message rpcMessage) {
	key := string(message.ID)
	ctx, cancel := context.WithCancel(server.context)
	server.mu.Lock()
	server.requests[key] = cancel
	server.mu.Unlock()
	server.wait.Add(1)
	go func() {
		defer server.wait.Done()
		defer cancel()
		result, rpcErr := server.handleRequest(ctx, message.Method, message.Params)
		if ctx.Err() != nil && rpcErr == nil {
			rpcErr = &rpcError{Code: errRequestCancelled, Message: "request canceled"}
		}
		if rpcErr == nil && result == nil {
			result = json.RawMessage("null")
		}
		server.mu.Lock()
		delete(server.requests, key)
		server.mu.Unlock()
		_ = server.writer.write(rpcMessage{JSONRPC: "2.0", ID: responseID(message.ID), Result: result, Error: rpcErr})
	}()
}

func (server *Server) handleRequest(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return server.initialize(params)
	case "shutdown":
		server.mu.Lock()
		server.shutdown = true
		if server.analysisCancel != nil {
			server.analysisCancel()
		}
		server.mu.Unlock()
		return nil, nil
	}
	server.mu.RLock()
	ready := server.initialized && !server.shutdown
	server.mu.RUnlock()
	if !ready {
		return nil, &rpcError{Code: errInvalidRequest, Message: "language server is not initialized"}
	}
	select {
	case <-ctx.Done():
		return nil, &rpcError{Code: errRequestCancelled, Message: "request canceled"}
	default:
	}
	switch method {
	case "textDocument/completion":
		var request textDocumentPositionParams
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: "invalid completion parameters"}
		}
		return server.completions(request), nil
	case "textDocument/hover":
		var request textDocumentPositionParams
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: "invalid hover parameters"}
		}
		return server.hover(request), nil
	case "textDocument/definition":
		var request textDocumentPositionParams
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: "invalid definition parameters"}
		}
		return server.definition(request), nil
	case "textDocument/documentSymbol":
		var request struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
		}
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, &rpcError{Code: errInvalidParams, Message: "invalid document-symbol parameters"}
		}
		return server.documentSymbols(request.TextDocument.URI), nil
	default:
		return nil, &rpcError{Code: errMethodNotFound, Message: "method not supported"}
	}
}

func (server *Server) initialize(params json.RawMessage) (any, *rpcError) {
	var request struct {
		RootURI          string `json:"rootUri"`
		RootPath         string `json:"rootPath"`
		WorkspaceFolders []struct {
			URI string `json:"uri"`
		} `json:"workspaceFolders"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, &rpcError{Code: errInvalidParams, Message: "invalid initialize parameters"}
	}
	if len(request.WorkspaceFolders) > 1 {
		return nil, &rpcError{Code: errInvalidParams, Message: "Hime-san accepts one workspace folder per language-server process"}
	}
	rootURI := request.RootURI
	if len(request.WorkspaceFolders) == 1 {
		rootURI = request.WorkspaceFolders[0].URI
	}
	var root string
	var err error
	if rootURI != "" {
		root, err = fileURIToPath(rootURI)
	} else if request.RootPath != "" {
		root, err = filepath.Abs(request.RootPath)
	} else {
		root, err = os.Getwd()
	}
	if err != nil {
		return nil, &rpcError{Code: errInvalidParams, Message: "workspace root is not a local filesystem path"}
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, &rpcError{Code: errInvalidParams, Message: "workspace root must be an existing non-symlink directory"}
	}
	evaluated, err := filepath.EvalSymlinks(root)
	if err != nil || filepath.Clean(evaluated) != filepath.Clean(root) {
		return nil, &rpcError{Code: errInvalidParams, Message: "workspace roots reached through symlinks are not supported"}
	}
	server.mu.Lock()
	if server.initialized {
		server.mu.Unlock()
		return nil, &rpcError{Code: errInvalidRequest, Message: "initialize may be sent only once"}
	}
	server.root = filepath.Clean(root)
	server.initialized = true
	server.mu.Unlock()
	return struct {
		Capabilities any `json:"capabilities"`
		ServerInfo   any `json:"serverInfo"`
	}{
		Capabilities: map[string]any{
			"positionEncoding":   "utf-16",
			"textDocumentSync":   map[string]any{"openClose": true, "change": 1, "save": map[string]any{"includeText": true}},
			"completionProvider": map[string]any{"triggerCharacters": []string{"<", "?", "~", "."}, "resolveProvider": false},
			"hoverProvider":      true, "definitionProvider": true, "documentSymbolProvider": true,
			"workspace": map[string]any{"workspaceFolders": map[string]any{"supported": false, "changeNotifications": false}},
		},
		ServerInfo: map[string]any{"name": "himesan", "version": compiler.CompilerVersion},
	}, nil
}

func (server *Server) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "initialized":
		server.scheduleReindex(false)
	case "$/cancelRequest":
		var request struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(params, &request) == nil {
			server.mu.RLock()
			cancel := server.requests[string(request.ID)]
			server.mu.RUnlock()
			if cancel != nil {
				cancel()
			}
		}
	case "textDocument/didOpen":
		var request struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
				Text    string `json:"text"`
			} `json:"textDocument"`
		}
		if json.Unmarshal(params, &request) == nil {
			server.updateOverlay(request.TextDocument.URI, request.TextDocument.Version, request.TextDocument.Text, true)
			server.scheduleReindex(false)
		}
	case "textDocument/didChange":
		var request struct {
			TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
			ContentChanges []struct {
				Range *Range `json:"range,omitempty"`
				Text  string `json:"text"`
			} `json:"contentChanges"`
		}
		if json.Unmarshal(params, &request) == nil && len(request.ContentChanges) != 0 {
			change := request.ContentChanges[len(request.ContentChanges)-1]
			if change.Range == nil {
				server.updateOverlay(request.TextDocument.URI, request.TextDocument.Version, change.Text, true)
				server.scheduleReindex(true)
			}
		}
	case "textDocument/didSave":
		var request struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
			Text         *string                `json:"text,omitempty"`
		}
		if json.Unmarshal(params, &request) == nil {
			if request.Text != nil {
				server.updateOverlay(request.TextDocument.URI, -1, *request.Text, true)
			}
			server.scheduleReindex(false)
		}
	case "textDocument/didClose":
		var request struct {
			TextDocument textDocumentIdentifier `json:"textDocument"`
		}
		if json.Unmarshal(params, &request) == nil {
			if path, err := fileURIToPath(request.TextDocument.URI); err == nil {
				server.mu.Lock()
				delete(server.overlays, path)
				server.mu.Unlock()
				server.scheduleReindex(false)
			}
		}
	case "workspace/didChangeWatchedFiles":
		server.scheduleReindex(false)
	}
}

func (server *Server) updateOverlay(uri string, version int, text string, open bool) {
	if len(text) > maxDocumentBytes || !strings.HasSuffix(strings.ToLower(uri), ".sando") {
		return
	}
	path, err := fileURIToPath(uri)
	if err != nil {
		return
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	if !editorPathAllowed(server.root, path) {
		return
	}
	if previous, ok := server.overlays[path]; ok && version < 0 {
		version = previous.Version
	}
	server.overlays[path] = document{URI: uri, Path: path, Text: []byte(text), Version: version, Open: open}
}

func (server *Server) notify(method string, params any) error {
	payload, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return server.writer.write(rpcMessage{JSONRPC: "2.0", Method: method, Params: payload})
}

func (server *Server) log(message string, count int) {
	// Logs deliberately contain only fixed messages and counts. Source text,
	// paths, environment values, and process details never cross this boundary.
	fmt.Fprintf(server.logs, "himesan lsp: %s (%d)\n", message, count)
}
