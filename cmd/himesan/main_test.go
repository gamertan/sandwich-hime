// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gamertan.com/sandwich-hime/internal/compiler"
)

func TestRunHelpVersionAndUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Sandwich Hime") || !strings.Contains(stdout.String(), ".san remains exclusively San") {
		t.Fatalf("help does not preserve product boundary: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"version", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version exit code = %d: %s", code, stderr.String())
	}
	var versionResult struct {
		Compiler   string   `json:"compiler"`
		RuntimeABI string   `json:"runtime_abi"`
		Features   []string `json:"features"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &versionResult); err != nil {
		t.Fatalf("version JSON: %v", err)
	}
	if versionResult.Compiler == "" || versionResult.RuntimeABI != compiler.RuntimeABI || len(versionResult.Features) != 1 || versionResult.Features[0] != "lsp-stdio" {
		t.Fatalf("version result = %#v", versionResult)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"lsp"}, &stdout, &stderr); code != 2 {
		t.Fatalf("lsp without --stdio exit code = %d, want 2", code)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"rebuke"}, &stdout, &stderr); code != 2 {
		t.Fatalf("unknown command exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unknown command diagnostic = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"check", "-h"}, &stdout, &stderr); code != 0 {
		t.Fatalf("command help exit code = %d, want 0", code)
	}
}

func TestGenerateCheckBlessAndJSONDiagnostics(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "hello.sando")
	source := "<?sando go\npackage views\nfunc Hello(name string)\n?>\n<p><?= name ?></p>\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"generate", sourcePath}, &stdout, &stderr); code != 0 {
		t.Fatalf("generate exit code = %d\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	generatedPath := sourcePath + ".go"
	before, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"bless", sourcePath}, &stdout, &stderr); code != 0 {
		t.Fatalf("bless exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no writes") {
		t.Fatalf("bless did not identify read-only behavior: %q", stdout.String())
	}
	after, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("bless changed generated output")
	}

	if err := os.WriteFile(sourcePath, []byte("not a component"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"check", "--json", sourcePath}, &stdout, &stderr); code != 1 {
		t.Fatalf("invalid check exit code = %d, want 1", code)
	}
	var payload struct {
		OK     bool `json:"ok"`
		Result struct {
			Diagnostics []compiler.Diagnostic `json:"diagnostics"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("diagnostic JSON: %v\n%s", err, stdout.String())
	}
	if payload.OK || len(payload.Result.Diagnostics) == 0 || payload.Result.Diagnostics[0].Code == "" {
		t.Fatalf("diagnostic payload = %#v", payload)
	}
}

func TestSplitAppArgs(t *testing.T) {
	t.Parallel()

	command, application := splitAppArgs([]string{"--proxy", "127.0.0.1:7444", "./cmd/site", "--", "--verbose", "hello world"})
	if got := strings.Join(command, "|"); got != "--proxy|127.0.0.1:7444|./cmd/site" {
		t.Fatalf("command arguments = %q", got)
	}
	if got := strings.Join(application, "|"); got != "--verbose|hello world" {
		t.Fatalf("application arguments = %q", got)
	}
}
