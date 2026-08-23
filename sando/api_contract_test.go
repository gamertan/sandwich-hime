// SPDX-License-Identifier: Apache-2.0

package sando

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestV1PublicAPIContract(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		files = append(files, file)
	}
	configuration := types.Config{Importer: importer.Default()}
	checked, err := configuration.Check("gamertan.com/sandwich-hime/sando", fileSet, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	qualifier := func(pkg *types.Package) string {
		if pkg == nil || pkg.Path() == checked.Path() {
			return ""
		}
		return pkg.Name()
	}
	var actual []string
	for _, name := range checked.Scope().Names() {
		if !token.IsExported(name) {
			continue
		}
		object := checked.Scope().Lookup(name)
		switch object := object.(type) {
		case *types.Const:
			actual = append(actual, fmt.Sprintf("const %s = %s", object.Name(), object.Val().ExactString()))
		case *types.TypeName:
			named, ok := object.Type().(*types.Named)
			if !ok {
				actual = append(actual, types.ObjectString(object, qualifier))
				break
			}
			if structure, ok := named.Underlying().(*types.Struct); ok {
				var fields []string
				for index := 0; index < structure.NumFields(); index++ {
					field := structure.Field(index)
					if field.Exported() {
						fields = append(fields, field.Name()+" "+types.TypeString(field.Type(), qualifier))
					}
				}
				if len(fields) == 0 {
					actual = append(actual, "type "+object.Name()+" struct{ /* opaque */ }")
				} else {
					actual = append(actual, "type "+object.Name()+" struct{"+strings.Join(fields, "; ")+"}")
				}
			} else {
				actual = append(actual, types.ObjectString(object, qualifier))
			}
		default:
			actual = append(actual, types.ObjectString(object, qualifier))
		}
		typeName, ok := object.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		for index := 0; index < named.NumMethods(); index++ {
			method := named.Method(index)
			if method.Exported() {
				actual = append(actual, types.ObjectString(method, qualifier))
			}
		}
	}
	sort.Strings(actual)
	got := strings.Join(actual, "\n") + "\n"
	want, err := os.ReadFile("testdata/public-api-v1.txt")
	if err != nil {
		t.Fatal(err)
	}
	want = []byte(strings.TrimPrefix(string(want), "# SPDX-License-Identifier: Apache-2.0\n\n"))
	if string(want) != got {
		t.Fatalf("v1 public API drifted\n--- committed contract ---\n%s--- observed API ---\n%s", want, got)
	}
}
