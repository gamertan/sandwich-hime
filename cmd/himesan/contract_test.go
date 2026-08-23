// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"gamertan.com/sandwich-hime/internal/testpath"
)

type contractSchema struct {
	AdditionalProperties bool                       `json:"additionalProperties"`
	Required             []string                   `json:"required"`
	Properties           map[string]json.RawMessage `json:"properties"`
}

func TestV1CLIHelpContract(t *testing.T) {
	t.Parallel()
	want, err := os.ReadFile(filepath.Join("..", "..", "contracts", "himesan-cli-help-v1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want = bytes.TrimPrefix(want, []byte("# SPDX-License-Identifier: AGPL-3.0-only\n\n"))
	var output bytes.Buffer
	printHelp(&output)
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("CLI help contract drifted\n--- want ---\n%s--- got ---\n%s", want, output.Bytes())
	}
}

func TestV1VersionJSONSchemaMatchesOutput(t *testing.T) {
	t.Parallel()
	schema := readContractSchema(t, "himesan-version-output-v1.schema.json")
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"version", "--json"}, &stdout, &stderr); code != 0 {
		t.Fatalf("version exit code = %d: %s", code, stderr.String())
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	assertObjectShape(t, output, schema, "version output")
}

func TestV1OperationJSONSchemaMatchesSuccessAndDiagnosticOutput(t *testing.T) {
	t.Parallel()
	schema := readContractSchema(t, "himesan-operation-output-v1.schema.json")
	directory := testpath.TempDir(t)
	source := filepath.Join(directory, "page.sando")
	if err := os.WriteFile(source, []byte("<?sando go\npackage views\nfunc Page()\n?>\n<p>page</p>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"check", "--json", source}, &stdout, &stderr); code != 1 {
		t.Fatalf("missing-output check exit code = %d, want 1: %s", code, stderr.String())
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	assertObjectShape(t, output, schema, "operation output")

	resultSchema := nestedSchema(t, schema.Properties["result"])
	result, ok := output["result"].(map[string]any)
	if !ok {
		t.Fatalf("result = %T, want object", output["result"])
	}
	assertObjectShape(t, result, resultSchema, "operation result")
	files, ok := result["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("files = %#v, want one item", result["files"])
	}
	filesProperty := rawObject(t, resultSchema.Properties["files"])
	fileSchema := nestedSchema(t, filesProperty["items"])
	assertObjectShape(t, files[0].(map[string]any), fileSchema, "file result")

	diagnostics, ok := result["diagnostics"].([]any)
	if !ok || len(diagnostics) == 0 {
		t.Fatalf("diagnostics = %#v, want at least one item", result["diagnostics"])
	}
	diagnosticsProperty := rawObject(t, resultSchema.Properties["diagnostics"])
	diagnosticSchema := nestedSchema(t, diagnosticsProperty["items"])
	assertObjectShape(t, diagnostics[0].(map[string]any), diagnosticSchema, "diagnostic")
}

func readContractSchema(t *testing.T, name string) contractSchema {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	var schema contractSchema
	if err := json.Unmarshal(contents, &schema); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	if schema.AdditionalProperties || len(schema.Properties) == 0 {
		t.Fatalf("%s is not a closed object schema", name)
	}
	return schema
}

func nestedSchema(t *testing.T, raw json.RawMessage) contractSchema {
	t.Helper()
	var schema contractSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func rawObject(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func assertObjectShape(t *testing.T, actual map[string]any, schema contractSchema, label string) {
	t.Helper()
	actualKeys := make([]string, 0, len(actual))
	for key := range actual {
		actualKeys = append(actualKeys, key)
		if _, declared := schema.Properties[key]; !declared {
			t.Fatalf("%s emitted undeclared property %q", label, key)
		}
	}
	sort.Strings(actualKeys)
	for _, required := range schema.Required {
		if _, present := actual[required]; !present {
			t.Fatalf("%s omitted required property %q (got %v)", label, required, actualKeys)
		}
	}
	if len(actual) == 0 || reflect.ValueOf(actual).IsNil() {
		t.Fatalf("%s is empty", label)
	}
}
