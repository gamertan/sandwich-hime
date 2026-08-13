// SPDX-License-Identifier: AGPL-3.0-only

package version

import "testing"

func TestSelectCompilerVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		linkerValue   string
		moduleVersion string
		want          string
	}{
		{name: "local build", linkerValue: developmentCompilerVersion, moduleVersion: "(devel)", want: developmentCompilerVersion},
		{name: "missing build info", linkerValue: developmentCompilerVersion, moduleVersion: "", want: developmentCompilerVersion},
		{name: "versioned go install", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0", want: "v1.0.0"},
		{name: "beta launch install", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0-beta.1", want: "v1.0.0-beta.1"},
		{name: "beta two install", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0-beta.2", want: "v1.0.0-beta.2"},
		{name: "versioned prerelease install", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0-rc.1", want: "v1.0.0-rc.1"},
		{name: "hyphenated prerelease install", linkerValue: developmentCompilerVersion, moduleVersion: "v1.2.3-beta-2", want: "v1.2.3-beta-2"},
		{name: "pseudo version", linkerValue: developmentCompilerVersion, moduleVersion: "v0.0.0-20260811120000-0123456789ab", want: developmentCompilerVersion},
		{name: "pseudo version after release", linkerValue: developmentCompilerVersion, moduleVersion: "v1.2.4-0.20260811120000-0123456789ab", want: developmentCompilerVersion},
		{name: "pseudo version after prerelease", linkerValue: developmentCompilerVersion, moduleVersion: "v1.2.3-rc.1.0.20260811120000-0123456789ab", want: developmentCompilerVersion},
		{name: "dirty pseudo version", linkerValue: developmentCompilerVersion, moduleVersion: "v0.0.0-20260811123456-fedcba987654+dirty", want: developmentCompilerVersion},
		{name: "dirty release checkout", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0+dirty", want: developmentCompilerVersion},
		{name: "build metadata", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0+build.1", want: developmentCompilerVersion},
		{name: "leading zero release", linkerValue: developmentCompilerVersion, moduleVersion: "v01.0.0", want: developmentCompilerVersion},
		{name: "leading zero numeric prerelease", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0-rc.01", want: developmentCompilerVersion},
		{name: "leading zero numeric beta identifier", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0-beta.01", want: developmentCompilerVersion},
		{name: "empty beta identifier", linkerValue: developmentCompilerVersion, moduleVersion: "v1.0.0-beta..1", want: developmentCompilerVersion},
		{name: "beta linker override wins", linkerValue: "v1.0.0-beta.1", moduleVersion: "(devel)", want: "v1.0.0-beta.1"},
		{name: "beta two linker override wins", linkerValue: "v1.0.0-beta.2", moduleVersion: "(devel)", want: "v1.0.0-beta.2"},
		{name: "linker override wins", linkerValue: "v1.0.0-rc.1", moduleVersion: "v1.0.0", want: "v1.0.0-rc.1"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := selectCompilerVersion(test.linkerValue, test.moduleVersion); got != test.want {
				t.Fatalf("selectCompilerVersion(%q, %q) = %q, want %q", test.linkerValue, test.moduleVersion, got, test.want)
			}
		})
	}
}
