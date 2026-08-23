// SPDX-License-Identifier: AGPL-3.0-only

// Package releaseartifact creates deterministic Hime-san release archives and
// checksummed native-platform verification receipts.
package releaseartifact

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const receiptSchema = "himesan.native-verification.v1"

var (
	releaseVersionPattern = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	gitObjectPattern      = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	digestPattern         = regexp.MustCompile(`^[0-9a-f]{64}$`)
	goVersionPattern      = regexp.MustCompile(`^go(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$`)
	platformPattern       = regexp.MustCompile(`^[a-z0-9]+$`)
)

// PackageOptions contains the reviewed inputs to one release archive.
type PackageOptions struct {
	Version         string
	Commit          string
	Tree            string
	GoVersion       string
	GOOS            string
	GOARCH          string
	BinaryPath      string
	LicensePath     string
	ReleaseNotes    string
	OutputDirectory string
	SourceDateEpoch int64
}

// PackageResult identifies the immutable unsigned release archive.
type PackageResult struct {
	ArchivePath string `json:"archive_path"`
	SHA256      string `json:"sha256"`
}

type buildInfo struct {
	SchemaVersion         int    `json:"schema_version"`
	Version               string `json:"version"`
	Commit                string `json:"commit"`
	Tree                  string `json:"tree"`
	GoVersion             string `json:"go_version"`
	GOOS                  string `json:"goos"`
	GOARCH                string `json:"goarch"`
	BinarySHA256          string `json:"binary_sha256"`
	UnsignedBinarySHA256  string `json:"unsigned_binary_sha256,omitempty"`
	UnsignedArchiveSHA256 string `json:"unsigned_archive_sha256,omitempty"`
	SigningIdentifier     string `json:"signing_identifier,omitempty"`
}

type spdxDocument struct {
	SPDXVersion       string        `json:"spdxVersion"`
	DataLicense       string        `json:"dataLicense"`
	SPDXID            string        `json:"SPDXID"`
	Name              string        `json:"name"`
	DocumentNamespace string        `json:"documentNamespace"`
	CreationInfo      creationInfo  `json:"creationInfo"`
	Packages          []spdxPackage `json:"packages"`
}

type creationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type spdxPackage struct {
	Name             string         `json:"name"`
	SPDXID           string         `json:"SPDXID"`
	VersionInfo      string         `json:"versionInfo"`
	DownloadLocation string         `json:"downloadLocation"`
	FilesAnalyzed    bool           `json:"filesAnalyzed"`
	LicenseConcluded string         `json:"licenseConcluded"`
	LicenseDeclared  string         `json:"licenseDeclared"`
	Checksums        []spdxChecksum `json:"checksums"`
}

type spdxChecksum struct {
	Algorithm     string `json:"algorithm"`
	ChecksumValue string `json:"checksumValue"`
}

// Receipt describes one completed native verification run.
type Receipt struct {
	SchemaVersion       string   `json:"schema_version"`
	Repository          string   `json:"repository"`
	Commit              string   `json:"commit"`
	Tree                string   `json:"tree"`
	GOOS                string   `json:"goos"`
	GOARCH              string   `json:"goarch"`
	GoVersion           string   `json:"go_version"`
	RunnerVersion       string   `json:"runner_version"`
	RunnerName          string   `json:"runner_name"`
	GeneratedDigest     string   `json:"generated_output_sha256"`
	UnsignedArtifactSHA string   `json:"unsigned_artifact_sha256,omitempty"`
	SuccessfulGates     []string `json:"successful_gates"`
	CompletedAt         string   `json:"completed_at"`
}

type archiveMember struct {
	name string
	mode int64
	data []byte
}

// Package creates a byte-reproducible unsigned tar.gz and its SHA-256 sidecar.
func Package(options PackageOptions) (PackageResult, error) {
	if err := validatePackageOptions(options); err != nil {
		return PackageResult{}, err
	}
	binary, err := os.ReadFile(options.BinaryPath)
	if err != nil {
		return PackageResult{}, fmt.Errorf("read binary: %w", err)
	}
	license, err := os.ReadFile(options.LicensePath)
	if err != nil {
		return PackageResult{}, fmt.Errorf("read license: %w", err)
	}
	releaseNotes, err := os.ReadFile(options.ReleaseNotes)
	if err != nil {
		return PackageResult{}, fmt.Errorf("read release notes: %w", err)
	}

	binaryDigest := digest(binary)
	infoBytes, err := indentedJSON(buildInfo{
		SchemaVersion: 1,
		Version:       options.Version,
		Commit:        options.Commit,
		Tree:          options.Tree,
		GoVersion:     options.GoVersion,
		GOOS:          options.GOOS,
		GOARCH:        options.GOARCH,
		BinarySHA256:  binaryDigest,
	})
	if err != nil {
		return PackageResult{}, err
	}

	created := time.Unix(options.SourceDateEpoch, 0).UTC().Format(time.RFC3339)
	sbomBytes, err := indentedJSON(spdxDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              "himesan-" + options.Version + "-" + options.GOOS + "-" + options.GOARCH,
		DocumentNamespace: "https://sandwichhime.com/spdx/himesan/" + options.Version + "/" + options.Commit + "/" + options.GOOS + "-" + options.GOARCH,
		CreationInfo: creationInfo{
			Created:  created,
			Creators: []string{"Organization: Gamertan"},
		},
		Packages: []spdxPackage{{
			Name:             "himesan",
			SPDXID:           "SPDXRef-Package-himesan",
			VersionInfo:      options.Version,
			DownloadLocation: "NOASSERTION",
			FilesAnalyzed:    false,
			LicenseConcluded: "AGPL-3.0-only",
			LicenseDeclared:  "AGPL-3.0-only",
			Checksums: []spdxChecksum{{
				Algorithm:     "SHA256",
				ChecksumValue: binaryDigest,
			}},
		}},
	})
	if err != nil {
		return PackageResult{}, err
	}

	members := []archiveMember{
		{name: "BUILDINFO.json", mode: 0o444, data: infoBytes},
		{name: "LICENSE", mode: 0o444, data: license},
		{name: "RELEASE.md", mode: 0o444, data: releaseNotes},
		{name: "SBOM.spdx.json", mode: 0o444, data: sbomBytes},
		{name: "himesan", mode: 0o555, data: binary},
	}
	checksumLines := make([]string, 0, len(members))
	for _, member := range members {
		checksumLines = append(checksumLines, digest(member.data)+"  "+member.name)
	}
	members = append(members, archiveMember{
		name: "SHA256SUMS", mode: 0o444,
		data: []byte(strings.Join(checksumLines, "\n") + "\n"),
	})
	sort.Slice(members, func(i, j int) bool { return members[i].name < members[j].name })

	if err := os.MkdirAll(options.OutputDirectory, 0o755); err != nil {
		return PackageResult{}, fmt.Errorf("create output directory: %w", err)
	}
	base := "himesan-" + strings.TrimPrefix(options.Version, "v") + "-" + options.GOOS + "-" + options.GOARCH
	archivePath := filepath.Join(options.OutputDirectory, base+".tar.gz")
	if err := writeArchive(archivePath, base, members, time.Unix(options.SourceDateEpoch, 0).UTC()); err != nil {
		return PackageResult{}, err
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		return PackageResult{}, fmt.Errorf("read completed archive: %w", err)
	}
	archiveDigest := digest(archive)
	if err := writeExclusive(archivePath+".sha256", []byte(archiveDigest+"  "+filepath.Base(archivePath)+"\n"), 0o444); err != nil {
		return PackageResult{}, fmt.Errorf("write archive checksum: %w", err)
	}
	return PackageResult{ArchivePath: archivePath, SHA256: archiveDigest}, nil
}

// WriteReceipt validates, canonicalizes, and writes a receipt plus SHA sidecar.
func WriteReceipt(path string, receipt Receipt) (string, error) {
	receipt.SchemaVersion = receiptSchema
	if err := validateReceipt(receipt); err != nil {
		return "", err
	}
	sort.Strings(receipt.SuccessfulGates)
	contents, err := indentedJSON(receipt)
	if err != nil {
		return "", err
	}
	if err := writeExclusive(path, contents, 0o444); err != nil {
		return "", fmt.Errorf("write receipt: %w", err)
	}
	checksum := digest(contents)
	if err := writeExclusive(path+".sha256", []byte(checksum+"  "+filepath.Base(path)+"\n"), 0o444); err != nil {
		return "", fmt.Errorf("write receipt checksum: %w", err)
	}
	return checksum, nil
}

// DigestFiles returns a stable digest over sorted names and file contents.
func DigestFiles(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", errors.New("at least one generated file is required")
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	hash := sha256.New()
	for _, path := range sorted {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", path, err)
		}
		_, _ = io.WriteString(hash, filepath.ToSlash(path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(contents)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validatePackageOptions(options PackageOptions) error {
	for name, value := range map[string]string{
		"version": options.Version, "commit": options.Commit, "tree": options.Tree,
		"go version": options.GoVersion, "GOOS": options.GOOS, "GOARCH": options.GOARCH,
		"binary": options.BinaryPath, "license": options.LicensePath,
		"release notes": options.ReleaseNotes, "output directory": options.OutputDirectory,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if options.SourceDateEpoch <= 0 {
		return errors.New("source date epoch must be positive")
	}
	if !releaseVersionPattern.MatchString(options.Version) {
		return errors.New("version must be a canonical v-prefixed semantic version without build metadata")
	}
	if !gitObjectPattern.MatchString(options.Commit) || !gitObjectPattern.MatchString(options.Tree) {
		return errors.New("commit and tree must be full lowercase Git object identities")
	}
	if !goVersionPattern.MatchString(options.GoVersion) {
		return errors.New("Go version must be a complete goX.Y.Z toolchain identity")
	}
	if !platformPattern.MatchString(options.GOOS) || !platformPattern.MatchString(options.GOARCH) {
		return errors.New("GOOS and GOARCH must contain only lowercase letters and digits")
	}
	return nil
}

func validateReceipt(receipt Receipt) error {
	for name, value := range map[string]string{
		"repository": receipt.Repository, "commit": receipt.Commit, "tree": receipt.Tree,
		"GOOS": receipt.GOOS, "GOARCH": receipt.GOARCH, "Go version": receipt.GoVersion,
		"runner version": receipt.RunnerVersion, "runner name": receipt.RunnerName,
		"generated digest": receipt.GeneratedDigest, "completed at": receipt.CompletedAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if len(receipt.SuccessfulGates) == 0 {
		return errors.New("at least one successful gate is required")
	}
	if !gitObjectPattern.MatchString(receipt.Commit) || !gitObjectPattern.MatchString(receipt.Tree) {
		return errors.New("commit and tree must be full lowercase Git object identities")
	}
	if !goVersionPattern.MatchString(receipt.GoVersion) {
		return errors.New("Go version must be a complete goX.Y.Z toolchain identity")
	}
	if !platformPattern.MatchString(receipt.GOOS) || !platformPattern.MatchString(receipt.GOARCH) {
		return errors.New("GOOS and GOARCH must contain only lowercase letters and digits")
	}
	if !digestPattern.MatchString(receipt.GeneratedDigest) {
		return errors.New("generated-output digest must be a lowercase SHA-256")
	}
	if receipt.UnsignedArtifactSHA != "" && !digestPattern.MatchString(receipt.UnsignedArtifactSHA) {
		return errors.New("unsigned-artifact digest must be a lowercase SHA-256")
	}
	if _, err := time.Parse(time.RFC3339, receipt.CompletedAt); err != nil {
		return fmt.Errorf("completed at must be RFC3339: %w", err)
	}
	return nil
}

func writeArchive(path, root string, members []archiveMember, modified time.Time) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()

	gzipWriter := gzip.NewWriter(file)
	gzipWriter.Header.ModTime = modified
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	for _, member := range members {
		header := &tar.Header{
			Name:       root + "/" + member.name,
			Mode:       member.mode,
			Size:       int64(len(member.data)),
			ModTime:    modified,
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Uid:        0,
			Gid:        0,
			Uname:      "",
			Gname:      "",
			Format:     tar.FormatUSTAR,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write archive header: %w", err)
		}
		if _, err := tarWriter.Write(member.data); err != nil {
			return fmt.Errorf("write archive member: %w", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("close tar stream: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("close gzip stream: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync archive: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close archive: %w", err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		return fmt.Errorf("set archive permissions: %w", err)
	}
	success = true
	return nil
}

func writeExclusive(path string, contents []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(contents); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	success = true
	return nil
}

func indentedJSON(value any) ([]byte, error) {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode JSON: %w", err)
	}
	return append(contents, '\n'), nil
}

func digest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
