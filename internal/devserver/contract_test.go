// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestV1ConfigSchemaMatchesConfigTypeAndDefaults(t *testing.T) {
	t.Parallel()
	contents, err := os.ReadFile(filepath.Join("..", "..", "contracts", "himesan-config-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		AdditionalProperties bool                       `json:"additionalProperties"`
		Required             []string                   `json:"required"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(contents, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.AdditionalProperties {
		t.Fatal("v1 config schema must reject unknown fields")
	}
	if !reflect.DeepEqual(schema.Required, []string{"version"}) {
		t.Fatalf("required config fields = %v, want [version]", schema.Required)
	}
	typeOfConfig := reflect.TypeOf(Config{})
	fields := make([]string, 0, typeOfConfig.NumField())
	for index := 0; index < typeOfConfig.NumField(); index++ {
		name := strings.Split(typeOfConfig.Field(index).Tag.Get("json"), ",")[0]
		fields = append(fields, name)
	}
	sort.Strings(fields)
	properties := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		properties = append(properties, name)
	}
	sort.Strings(properties)
	if !reflect.DeepEqual(fields, properties) {
		t.Fatalf("Config JSON fields %v do not match schema properties %v", fields, properties)
	}

	defaults := DefaultConfig()
	wantDefaults := map[string]any{
		"version":              float64(defaults.Version),
		"sourceRoots":          []any{"."},
		"goPackage":            defaults.GoPackage,
		"appArgs":              []any{},
		"listenAddressEnv":     defaults.ListenAddressEnv,
		"healthPath":           defaults.HealthPath,
		"proxyAddress":         defaults.ProxyAddress,
		"additionalWatchRoots": []any{},
	}
	for name, want := range wantDefaults {
		var property map[string]any
		if err := json.Unmarshal(schema.Properties[name], &property); err != nil {
			t.Fatal(err)
		}
		if name == "version" {
			if !reflect.DeepEqual(property["const"], want) {
				t.Fatalf("schema %s const = %#v, want %#v", name, property["const"], want)
			}
			continue
		}
		if !reflect.DeepEqual(property["default"], want) {
			t.Fatalf("schema %s default = %#v, want %#v", name, property["default"], want)
		}
	}
}
