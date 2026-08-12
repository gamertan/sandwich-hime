// SPDX-License-Identifier: AGPL-3.0-only

// Package version contains build identifiers shared by the compiler and CLI.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
)

const developmentCompilerVersion = "0.1.0-dev"

// Compiler may be replaced in release binaries with
// `-X gamertan.com/sandwich-hime/internal/version.Compiler=vX.Y.Z`.
// A versioned `go install module/package@vX.Y.Z` instead supplies the main
// module version through Go build information; init adopts that version when
// no explicit linker value was provided.
var Compiler = developmentCompilerVersion

var (
	taggedCompilerVersionPattern = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	pseudoVersionSuffixPattern   = regexp.MustCompile(`(?:^|[.-])(?:0\.)?[0-9]{14}-[0-9a-f]{12,}$`)
)

func init() {
	if information, ok := debug.ReadBuildInfo(); ok {
		Compiler = selectCompilerVersion(Compiler, information.Main.Version)
	}
}

func selectCompilerVersion(linkerValue, moduleVersion string) string {
	if linkerValue != developmentCompilerVersion {
		return linkerValue
	}
	moduleVersion = strings.TrimSpace(moduleVersion)
	if !isTaggedCompilerVersion(moduleVersion) {
		return linkerValue
	}
	return moduleVersion
}

func isTaggedCompilerVersion(value string) bool {
	matches := taggedCompilerVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		// Build metadata is deliberately excluded. In particular, local VCS
		// builds can carry +dirty and must remain development builds.
		return false
	}
	prerelease := matches[1]
	if pseudoVersionSuffixPattern.MatchString(prerelease) {
		// Go synthesizes valid-semver pseudo-versions for local VCS builds. They
		// identify source, but they are not signed/tagged Hime-san releases.
		return false
	}
	for _, identifier := range strings.Split(prerelease, ".") {
		if len(identifier) > 1 && identifier[0] == '0' && allDecimal(identifier) {
			return false
		}
	}
	return true
}

func allDecimal(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

const (
	// RuntimeABI identifies the generated-code/runtime contract.
	RuntimeABI = "sando.v1"

	// ConfigSchema is the supported himesan.json schema version.
	ConfigSchema = 1
)
