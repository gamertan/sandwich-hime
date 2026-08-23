// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maximumNativeArchiveSize = 256 << 20

var unsignedPackageMembers = map[string]os.FileMode{
	"BUILDINFO.json": 0o444,
	"LICENSE":        0o444,
	"RELEASE.md":     0o444,
	"SBOM.spdx.json": 0o444,
	"SHA256SUMS":     0o444,
	"himesan":        0o555,
}

// MacOSSigningOptions are the reviewed identities added after Developer ID
// signing changes the native binary bytes.
type MacOSSigningOptions struct {
	Directory             string
	UnsignedArchiveSHA256 string
	Identity              string
	Identifier            string
	FinalizedAt           string
}

type signingInfo struct {
	SchemaVersion         int    `json:"schema_version"`
	UnsignedArchiveSHA256 string `json:"unsigned_archive_sha256"`
	UnsignedBinarySHA256  string `json:"unsigned_binary_sha256"`
	SignedBinarySHA256    string `json:"signed_binary_sha256"`
	Identity              string `json:"identity"`
	Identifier            string `json:"identifier"`
	FinalizedAt           string `json:"finalized_at"`
}

// ExtractVerifiedMacOSPackage verifies the approved unsigned archive and
// extracts its fixed file set without delegating path handling to system tar.
func ExtractVerifiedMacOSPackage(archivePath, expectedSHA256, outputDirectory string) (string, error) {
	if !digestPattern.MatchString(expectedSHA256) {
		return "", errors.New("approved archive digest must be a lowercase SHA-256")
	}
	info, err := os.Lstat(archivePath)
	if err != nil {
		return "", fmt.Errorf("inspect unsigned archive: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("unsigned archive must be a regular file, not a symlink")
	}
	if info.Size() <= 0 || info.Size() > maximumNativeArchiveSize {
		return "", errors.New("unsigned archive size is outside the permitted range")
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		return "", fmt.Errorf("read unsigned archive: %w", err)
	}
	if digest(archive) != expectedSHA256 {
		return "", errors.New("unsigned archive does not match the approved digest")
	}
	members, root, err := readUnsignedPackage(archive)
	if err != nil {
		return "", err
	}
	if err := validateUnsignedPackage(members, root); err != nil {
		return "", err
	}

	outputInfo, err := os.Lstat(outputDirectory)
	if err != nil {
		return "", fmt.Errorf("inspect extraction directory: %w", err)
	}
	if !outputInfo.IsDir() || outputInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("extraction destination must be a real directory")
	}
	rootPath := filepath.Join(outputDirectory, root)
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		return "", fmt.Errorf("create extracted package root: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(rootPath)
		}
	}()
	for name, mode := range unsignedPackageMembers {
		if err := writeExclusive(filepath.Join(rootPath, name), members[name], mode); err != nil {
			return "", fmt.Errorf("extract %s: %w", name, err)
		}
	}
	success = true
	return rootPath, nil
}

// FinalizeSignedMacOSDistribution replaces unsigned binary provenance with a
// complete signed-distribution record and regenerates every internal checksum.
func FinalizeSignedMacOSDistribution(options MacOSSigningOptions) error {
	if !digestPattern.MatchString(options.UnsignedArchiveSHA256) {
		return errors.New("unsigned archive digest must be a lowercase SHA-256")
	}
	if strings.TrimSpace(options.Identity) == "" || strings.ContainsAny(options.Identity, "\r\n\x00") {
		return errors.New("signing identity is required and must be one line")
	}
	if strings.TrimSpace(options.Identifier) == "" || strings.ContainsAny(options.Identifier, "\r\n\x00") {
		return errors.New("signing identifier is required and must be one line")
	}
	if _, err := time.Parse(time.RFC3339, options.FinalizedAt); err != nil {
		return fmt.Errorf("finalization time must be RFC3339: %w", err)
	}

	for name := range unsignedPackageMembers {
		if _, err := readRegularFile(filepath.Join(options.Directory, name), maximumNativeArchiveSize); err != nil {
			return fmt.Errorf("inspect signed distribution %s: %w", name, err)
		}
	}
	buildPath := filepath.Join(options.Directory, "BUILDINFO.json")
	buildContents, err := os.ReadFile(buildPath)
	if err != nil {
		return fmt.Errorf("read unsigned build information: %w", err)
	}
	var build buildInfo
	if err := decodeStrictJSON(buildContents, &build); err != nil {
		return fmt.Errorf("decode unsigned build information: %w", err)
	}
	if build.SchemaVersion != 1 || !digestPattern.MatchString(build.BinarySHA256) || build.GOOS != "darwin" || build.GOARCH != "arm64" {
		return errors.New("unsigned build information is not a supported Darwin/arm64 package")
	}
	unsignedBinaryDigest := build.BinarySHA256
	signedBinary, err := os.ReadFile(filepath.Join(options.Directory, "himesan"))
	if err != nil {
		return fmt.Errorf("read signed binary: %w", err)
	}
	signedBinaryDigest := digest(signedBinary)
	if signedBinaryDigest == unsignedBinaryDigest {
		return errors.New("Developer ID signing did not change the native binary bytes")
	}

	build.SchemaVersion = 2
	build.BinarySHA256 = signedBinaryDigest
	build.UnsignedBinarySHA256 = unsignedBinaryDigest
	build.UnsignedArchiveSHA256 = options.UnsignedArchiveSHA256
	build.SigningIdentifier = options.Identifier
	newBuild, err := indentedJSON(build)
	if err != nil {
		return err
	}

	sbomPath := filepath.Join(options.Directory, "SBOM.spdx.json")
	sbomContents, err := os.ReadFile(sbomPath)
	if err != nil {
		return fmt.Errorf("read unsigned SBOM: %w", err)
	}
	var sbom spdxDocument
	if err := decodeStrictJSON(sbomContents, &sbom); err != nil {
		return fmt.Errorf("decode unsigned SBOM: %w", err)
	}
	if len(sbom.Packages) != 1 || len(sbom.Packages[0].Checksums) != 1 ||
		sbom.Packages[0].Checksums[0].Algorithm != "SHA256" ||
		sbom.Packages[0].Checksums[0].ChecksumValue != unsignedBinaryDigest {
		return errors.New("unsigned SBOM does not match the unsigned binary")
	}
	sbom.Packages[0].Checksums[0].ChecksumValue = signedBinaryDigest
	newSBOM, err := indentedJSON(sbom)
	if err != nil {
		return err
	}
	signingContents, err := indentedJSON(signingInfo{
		SchemaVersion:         1,
		UnsignedArchiveSHA256: options.UnsignedArchiveSHA256,
		UnsignedBinarySHA256:  unsignedBinaryDigest,
		SignedBinarySHA256:    signedBinaryDigest,
		Identity:              options.Identity,
		Identifier:            options.Identifier,
		FinalizedAt:           options.FinalizedAt,
	})
	if err != nil {
		return err
	}

	if err := replaceRegularFile(buildPath, newBuild, 0o444); err != nil {
		return fmt.Errorf("replace signed build information: %w", err)
	}
	if err := replaceRegularFile(sbomPath, newSBOM, 0o444); err != nil {
		return fmt.Errorf("replace signed SBOM: %w", err)
	}
	if err := writeExclusive(filepath.Join(options.Directory, "SIGNING.json"), signingContents, 0o444); err != nil {
		return fmt.Errorf("write signing information: %w", err)
	}

	checksumNames := []string{"BUILDINFO.json", "LICENSE", "RELEASE.md", "SBOM.spdx.json", "SIGNING.json", "himesan"}
	var checksumLines []string
	for _, name := range checksumNames {
		contents, err := os.ReadFile(filepath.Join(options.Directory, name))
		if err != nil {
			return fmt.Errorf("read signed distribution member %s: %w", name, err)
		}
		checksumLines = append(checksumLines, digest(contents)+"  "+name)
	}
	checksums := []byte(strings.Join(checksumLines, "\n") + "\n")
	if err := replaceRegularFile(filepath.Join(options.Directory, "SHA256SUMS"), checksums, 0o444); err != nil {
		return fmt.Errorf("replace signed distribution checksums: %w", err)
	}
	return nil
}

func readUnsignedPackage(archive []byte) (map[string][]byte, string, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, "", fmt.Errorf("open unsigned gzip archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(io.LimitReader(gzipReader, maximumNativeArchiveSize+1))
	members := make(map[string][]byte, len(unsignedPackageMembers))
	root := ""
	var totalSize int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("read unsigned tar archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maximumNativeArchiveSize {
			return nil, "", errors.New("unsigned archive contains a non-regular or oversized member")
		}
		totalSize += header.Size
		if totalSize > maximumNativeArchiveSize {
			return nil, "", errors.New("unsigned archive expands beyond the permitted size")
		}
		clean := filepath.ToSlash(filepath.Clean(header.Name))
		parts := strings.Split(clean, "/")
		if len(parts) != 2 || parts[0] == "" || parts[0] == "." || parts[0] == ".." {
			return nil, "", fmt.Errorf("unsigned archive member has an unsafe path: %q", header.Name)
		}
		if root == "" {
			root = parts[0]
		} else if root != parts[0] {
			return nil, "", errors.New("unsigned archive contains more than one package root")
		}
		name := parts[1]
		expectedMode, ok := unsignedPackageMembers[name]
		if !ok || os.FileMode(header.Mode).Perm() != expectedMode || header.Linkname != "" {
			return nil, "", fmt.Errorf("unsigned archive member %q has an unexpected name, mode, or link", name)
		}
		if _, exists := members[name]; exists {
			return nil, "", fmt.Errorf("unsigned archive repeats member %q", name)
		}
		contents, err := io.ReadAll(io.LimitReader(tarReader, header.Size+1))
		if err != nil || int64(len(contents)) != header.Size {
			return nil, "", fmt.Errorf("read unsigned archive member %q", name)
		}
		members[name] = contents
	}
	if err := gzipReader.Close(); err != nil {
		return nil, "", fmt.Errorf("finish unsigned gzip archive: %w", err)
	}
	return members, root, nil
}

func validateUnsignedPackage(members map[string][]byte, root string) error {
	if len(members) != len(unsignedPackageMembers) || root == "" {
		return errors.New("unsigned archive does not contain the exact release file set")
	}
	var build buildInfo
	if err := decodeStrictJSON(members["BUILDINFO.json"], &build); err != nil {
		return fmt.Errorf("decode unsigned build information: %w", err)
	}
	if build.SchemaVersion != 1 || build.GOOS != "darwin" || build.GOARCH != "arm64" ||
		!releaseVersionPattern.MatchString(build.Version) || root != "himesan-"+strings.TrimPrefix(build.Version, "v")+"-darwin-arm64" ||
		!gitObjectPattern.MatchString(build.Commit) || !gitObjectPattern.MatchString(build.Tree) ||
		!goVersionPattern.MatchString(build.GoVersion) || build.BinarySHA256 != digest(members["himesan"]) {
		return errors.New("unsigned build information does not match the archive")
	}
	var sbom spdxDocument
	if err := decodeStrictJSON(members["SBOM.spdx.json"], &sbom); err != nil {
		return fmt.Errorf("decode unsigned SBOM: %w", err)
	}
	if len(sbom.Packages) != 1 || len(sbom.Packages[0].Checksums) != 1 ||
		sbom.Packages[0].Checksums[0].Algorithm != "SHA256" ||
		sbom.Packages[0].Checksums[0].ChecksumValue != build.BinarySHA256 {
		return errors.New("unsigned SBOM does not match the native binary")
	}
	expectedNames := []string{"BUILDINFO.json", "LICENSE", "RELEASE.md", "SBOM.spdx.json", "himesan"}
	var expectedLines []string
	for _, name := range expectedNames {
		expectedLines = append(expectedLines, digest(members[name])+"  "+name)
	}
	if string(members["SHA256SUMS"]) != strings.Join(expectedLines, "\n")+"\n" {
		return errors.New("unsigned package checksum manifest does not match its members")
	}
	return nil
}

func readRegularFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || info.Size() > maximum {
		return nil, errors.New("must be a bounded regular file, not a symlink")
	}
	return os.ReadFile(path)
}

func decodeStrictJSON(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func replaceRegularFile(path string, contents []byte, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("replacement target must be a regular file, not a symlink")
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".himesan-release-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	success := false
	defer func() {
		_ = temporary.Close()
		if !success {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(contents); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporaryPath, mode); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	success = true
	return nil
}
