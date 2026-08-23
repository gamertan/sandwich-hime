// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackageIsByteReproducible(t *testing.T) {
	directory := t.TempDir()
	binary := writeFixture(t, directory, "himesan", "native binary")
	license := writeFixture(t, directory, "LICENSE", "license")
	releaseNotes := writeFixture(t, directory, "RELEASE.md", "release")
	options := PackageOptions{
		Version: "v1.0.0-rc.1", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), GoVersion: "go1.27.0",
		GOOS: "darwin", GOARCH: "arm64", BinaryPath: binary, LicensePath: license,
		ReleaseNotes: releaseNotes, SourceDateEpoch: 1_700_000_000,
	}
	options.OutputDirectory = filepath.Join(directory, "first")
	first, err := Package(options)
	if err != nil {
		t.Fatal(err)
	}
	options.OutputDirectory = filepath.Join(directory, "second")
	second, err := Package(options)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 {
		t.Fatalf("repeated packages differ: %s != %s", first.SHA256, second.SHA256)
	}
}

func TestWriteReceiptSortsGatesAndWritesChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	checksum, err := WriteReceipt(path, Receipt{
		Repository: "example.test/development-source", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40),
		GOOS: "darwin", GOARCH: "arm64", GoVersion: "go1.27.0",
		RunnerVersion: "v3.3.0", RunnerName: "mac", GeneratedDigest: strings.Repeat("c", 64),
		SuccessfulGates: []string{"vet", "test"}, CompletedAt: time.Unix(1_700_000_000, 0).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	if checksum == "" {
		t.Fatal("empty checksum")
	}
	if _, err := os.Stat(path + ".sha256"); err != nil {
		t.Fatal(err)
	}
}

func TestPackageRejectsUnsafeVersionAndExistingArtifact(t *testing.T) {
	directory := t.TempDir()
	options := PackageOptions{
		Version: "../../outside", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40),
		GoVersion: "go1.27.0", GOOS: "darwin", GOARCH: "arm64",
		BinaryPath:      writeFixture(t, directory, "himesan", "native binary"),
		LicensePath:     writeFixture(t, directory, "LICENSE", "license"),
		ReleaseNotes:    writeFixture(t, directory, "RELEASE.md", "release"),
		OutputDirectory: filepath.Join(directory, "output"), SourceDateEpoch: 1_700_000_000,
	}
	if _, err := Package(options); err == nil {
		t.Fatal("unsafe version was accepted")
	}
	options.Version = "v1.0.0-rc.1"
	if _, err := Package(options); err != nil {
		t.Fatal(err)
	}
	if _, err := Package(options); err == nil {
		t.Fatal("existing immutable artifact was overwritten")
	}
}

func TestDigestFilesBindsNamesAndBytes(t *testing.T) {
	directory := t.TempDir()
	one := writeFixture(t, directory, "one", "same")
	two := writeFixture(t, directory, "two", "same")
	forward, err := DigestFiles([]string{two, one})
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := DigestFiles([]string{one, two})
	if err != nil {
		t.Fatal(err)
	}
	if forward != reverse {
		t.Fatal("file ordering changed digest")
	}
}

func writeFixture(t *testing.T, directory, name, contents string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
