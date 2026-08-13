// SPDX-License-Identifier: AGPL-3.0-only

package lsp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLanguageServerSourceHasNoExecutionNetworkOrWriteCapability(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbiddenImports := map[string]bool{
		"net": true, "net/http": true, "net/rpc": true,
		"os/exec": true, "syscall": true,
	}
	forbiddenOSCalls := map[string]bool{
		"Create": true, "CreateTemp": true, "Mkdir": true, "MkdirAll": true,
		"OpenFile": true, "Remove": true, "RemoveAll": true, "Rename": true,
		"WriteFile": true, "Chmod": true, "Chown": true,
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if forbiddenImports[path] {
				t.Errorf("%s imports forbidden capability %s", entry.Name(), path)
			}
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !forbiddenOSCalls[selector.Sel.Name] {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && identifier.Name == "os" {
				t.Errorf("%s calls forbidden filesystem mutation os.%s", entry.Name(), selector.Sel.Name)
			}
			return true
		})
	}
}
