// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractAndFinalizeSignedMacOSDistribution(t *testing.T) {
	archive, archiveDigest := macOSPackageFixture(t)
	extraction := t.TempDir()
	root, err := ExtractVerifiedMacOSPackage(archive, archiveDigest, extraction)
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(root, "himesan")
	unsignedBinary, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(binaryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	binary, err := os.OpenFile(binaryPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := binary.WriteString("developer-id-signature"); err != nil {
		t.Fatal(err)
	}
	if err := binary.Close(); err != nil {
		t.Fatal(err)
	}

	options := MacOSSigningOptions{
		Directory: root, UnsignedArchiveSHA256: archiveDigest,
		Identity:    "Developer ID Application: Example (TEAMID)",
		Identifier:  "com.gamertan.sandwich-hime.himesan",
		FinalizedAt: "2026-08-23T18:00:00Z",
	}
	if err := FinalizeSignedMacOSDistribution(options); err != nil {
		t.Fatal(err)
	}

	var build buildInfo
	buildContents, err := os.ReadFile(filepath.Join(root, "BUILDINFO.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictJSON(buildContents, &build); err != nil {
		t.Fatal(err)
	}
	signedBinary, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if build.SchemaVersion != 2 || build.BinarySHA256 != digest(signedBinary) ||
		build.UnsignedBinarySHA256 != digest(unsignedBinary) || build.UnsignedArchiveSHA256 != archiveDigest ||
		build.SigningIdentifier != options.Identifier {
		t.Fatalf("signed build information is incomplete: %#v", build)
	}

	var signing signingInfo
	signingContents, err := os.ReadFile(filepath.Join(root, "SIGNING.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictJSON(signingContents, &signing); err != nil {
		t.Fatal(err)
	}
	if signing.SignedBinarySHA256 != build.BinarySHA256 || signing.UnsignedBinarySHA256 != build.UnsignedBinarySHA256 {
		t.Fatalf("signing record does not match build information: %#v", signing)
	}

	var sbom spdxDocument
	sbomContents, err := os.ReadFile(filepath.Join(root, "SBOM.spdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeStrictJSON(sbomContents, &sbom); err != nil {
		t.Fatal(err)
	}
	if got := sbom.Packages[0].Checksums[0].ChecksumValue; got != build.BinarySHA256 {
		t.Fatalf("signed SBOM checksum = %s, want %s", got, build.BinarySHA256)
	}
	assertChecksumManifest(t, root, []string{"BUILDINFO.json", "LICENSE", "RELEASE.md", "SBOM.spdx.json", "SIGNING.json", "himesan"})
	if err := FinalizeSignedMacOSDistribution(options); err == nil {
		t.Fatal("signed distribution was finalized twice")
	}
}

func TestExtractVerifiedMacOSPackageRejectsSubstitutionAndSymlink(t *testing.T) {
	archive, archiveDigest := macOSPackageFixture(t)
	if _, err := ExtractVerifiedMacOSPackage(archive, strings.Repeat("0", 64), t.TempDir()); err == nil {
		t.Fatal("archive substitution was accepted")
	}
	symlink := filepath.Join(t.TempDir(), "candidate.tar.gz")
	if err := os.Symlink(archive, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractVerifiedMacOSPackage(symlink, archiveDigest, t.TempDir()); err == nil {
		t.Fatal("symlinked archive was accepted")
	}

	unsafeArchive := filepath.Join(t.TempDir(), "unsafe.tar.gz")
	destination := t.TempDir()
	escapeName := filepath.Base(destination) + "-outside"
	unsafeContents := unsafeTarGzip(t, "../"+escapeName, []byte("not a package"))
	if err := os.WriteFile(unsafeArchive, unsafeContents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractVerifiedMacOSPackage(unsafeArchive, digest(unsafeContents), destination); err == nil {
		t.Fatal("archive path traversal was accepted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(destination), escapeName)); !os.IsNotExist(err) {
		t.Fatal("unsafe archive wrote outside the extraction directory")
	}
}

func macOSPackageFixture(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	options := PackageOptions{
		Version: "v1.0.0-rc.1", Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40),
		GoVersion: "go1.27.0", GOOS: "darwin", GOARCH: "arm64",
		BinaryPath:      writeFixture(t, directory, "himesan", "unsigned Mach-O fixture"),
		LicensePath:     writeFixture(t, directory, "LICENSE", "licence"),
		ReleaseNotes:    writeFixture(t, directory, "RELEASE.md", "release"),
		OutputDirectory: filepath.Join(directory, "package"), SourceDateEpoch: 1_700_000_000,
	}
	result, err := Package(options)
	if err != nil {
		t.Fatal(err)
	}
	return result.ArchivePath, result.SHA256
}

func assertChecksumManifest(t *testing.T, directory string, names []string) {
	t.Helper()
	var lines []string
	for _, name := range names {
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, digest(contents)+"  "+name)
	}
	checksums, err := os.ReadFile(filepath.Join(directory, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(checksums), strings.Join(lines, "\n")+"\n"; got != want {
		t.Fatalf("checksum manifest mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func unsafeTarGzip(t *testing.T, name string, contents []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o444, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
