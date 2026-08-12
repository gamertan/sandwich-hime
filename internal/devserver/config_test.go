// SPDX-License-Identifier: AGPL-3.0-only

package devserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigDefaultsAndRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "himesan.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"proxyAddress":"[::1]:0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.GoPackage != "." || cfg.ListenAddressEnv != defaultListenAddressEnv || cfg.HealthPath != "/" {
		t.Fatalf("LoadConfig() did not apply defaults: %#v", cfg)
	}

	if err := os.WriteFile(path, []byte(`{"version":1,"mystery":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadConfig() unknown field error = %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"proxyAddress":"127.0.0.1:0"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("LoadConfig() missing version error = %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"public proxy", func(c *Config) { c.ProxyAddress = "0.0.0.0:7331" }},
		{"hostname proxy", func(c *Config) { c.ProxyAddress = "localhost:7331" }},
		{"bad port", func(c *Config) { c.ProxyAddress = "127.0.0.1:http" }},
		{"bad environment", func(c *Config) { c.ListenAddressEnv = "bad-name" }},
		{"health query", func(c *Config) { c.HealthPath = "/health?full=1" }},
		{"empty source roots", func(c *Config) { c.SourceRoots = nil }},
		{"nul argument", func(c *Config) { c.AppArgs = []string{"a\x00b"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := DefaultConfig()
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}
	for _, address := range []string{"127.0.0.1:0", "127.12.3.4:65535", "[::1]:7331"} {
		if err := ValidateLoopbackAddress(address); err != nil {
			t.Errorf("ValidateLoopbackAddress(%q) = %v", address, err)
		}
	}
}
