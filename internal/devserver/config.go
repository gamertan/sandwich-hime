// SPDX-License-Identifier: AGPL-3.0-only

// Package devserver implements Hime-san's local-only development supervisor.
// It is intentionally independent from the template compiler and production
// runtime.
package devserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	// ConfigVersion is the himesan.json schema version understood by this
	// package.
	ConfigVersion = 1

	defaultListenAddressEnv = "HIMESAN_LISTEN_ADDR"
	defaultHealthPath       = "/"
	defaultProxyAddress     = "127.0.0.1:7331"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Config is the versioned, non-secret himesan.json development configuration.
// Arguments are passed directly to the application; they are never interpreted
// by a shell.
type Config struct {
	Version              int      `json:"version"`
	SourceRoots          []string `json:"sourceRoots"`
	GoPackage            string   `json:"goPackage"`
	AppArgs              []string `json:"appArgs,omitempty"`
	ListenAddressEnv     string   `json:"listenAddressEnv"`
	HealthPath           string   `json:"healthPath"`
	ProxyAddress         string   `json:"proxyAddress"`
	AdditionalWatchRoots []string `json:"additionalWatchRoots,omitempty"`
}

// DefaultConfig returns safe defaults for a simple, single-module project.
func DefaultConfig() Config {
	return Config{
		Version:          ConfigVersion,
		SourceRoots:      []string{"."},
		GoPackage:        ".",
		ListenAddressEnv: defaultListenAddressEnv,
		HealthPath:       defaultHealthPath,
		ProxyAddress:     defaultProxyAddress,
	}
}

// LoadConfig reads a himesan.json file, applies defaults for omitted optional
// fields, rejects unknown fields, and validates the result. Paths remain
// relative to the project root supplied later through Options.RootDir.
func LoadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open development config: %w", err)
	}
	defer f.Close()

	cfg := DefaultConfig()
	// Unlike optional fields, the schema version must be written explicitly so
	// future defaults cannot silently reinterpret an old file.
	cfg.Version = 0
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode development config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("decode development config: multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode development config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate development config: %w", err)
	}
	return cfg, nil
}

// Validate checks the schema and all values that do not require filesystem
// access. In particular, the stable proxy is restricted to loopback.
func (c Config) Validate() error {
	if c.Version != ConfigVersion {
		return fmt.Errorf("unsupported config version %d (want %d)", c.Version, ConfigVersion)
	}
	if len(c.SourceRoots) == 0 {
		return errors.New("sourceRoots must contain at least one path")
	}
	for _, root := range append(append([]string(nil), c.SourceRoots...), c.AdditionalWatchRoots...) {
		if err := validatePathValue(root); err != nil {
			return err
		}
	}
	if strings.TrimSpace(c.GoPackage) == "" {
		return errors.New("goPackage must not be empty")
	}
	if strings.ContainsAny(c.GoPackage, "\x00\r\n") {
		return errors.New("goPackage contains a control character")
	}
	for _, arg := range c.AppArgs {
		if strings.ContainsRune(arg, '\x00') {
			return errors.New("appArgs contains a NUL byte")
		}
	}
	if !environmentNamePattern.MatchString(c.ListenAddressEnv) {
		return fmt.Errorf("listenAddressEnv %q is not a valid environment variable name", c.ListenAddressEnv)
	}
	if !strings.HasPrefix(c.HealthPath, "/") || strings.HasPrefix(c.HealthPath, "//") {
		return errors.New("healthPath must be an absolute URL path")
	}
	if strings.ContainsAny(c.HealthPath, "\x00\r\n?#") {
		return errors.New("healthPath must not contain controls, a query, or a fragment")
	}
	if err := ValidateLoopbackAddress(c.ProxyAddress); err != nil {
		return fmt.Errorf("proxyAddress: %w", err)
	}
	return nil
}

func validatePathValue(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("watch paths must not be empty")
	}
	if strings.ContainsRune(path, '\x00') {
		return errors.New("watch path contains a NUL byte")
	}
	return nil
}

// ValidateLoopbackAddress rejects wildcard, public, malformed, and
// hostname-based proxy bindings. Requiring a literal loopback IP prevents a
// hosts-file or DNS change from broadening the development server's exposure.
func ValidateLoopbackAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("must be host:port: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		return fmt.Errorf("port %q is not numeric or is outside 0-65535", port)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("host %q is not a loopback IP", host)
	}
	return nil
}

func resolveProjectPath(rootDir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(rootDir, filepath.Clean(path))
}
