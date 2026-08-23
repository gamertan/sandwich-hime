// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	evidenceSchema       = "himesan.release-evidence.v1"
	evidenceManifestName = "RELEASE-EVIDENCE.json"
	maximumEvidenceSize  = 1 << 20
)

var requiredEvidenceFiles = []string{
	"benchmark-methodology.md",
	"development-supervisor.md",
	"legal-review.md",
	"native-platforms.md",
	"security.md",
	"signing-and-recovery.md",
	"vanity-imports.md",
}

// EvidenceIdentity binds human-reviewed release evidence to one source state.
type EvidenceIdentity struct {
	Repository string
	Version    string
	Commit     string
	Tree       string
	ReviewedBy string
	ReviewedAt string
}

type evidenceManifest struct {
	SchemaVersion string         `json:"schema_version"`
	Repository    string         `json:"repository"`
	Version       string         `json:"version"`
	Commit        string         `json:"commit"`
	Tree          string         `json:"tree"`
	ReviewStatus  string         `json:"review_status"`
	ReviewedBy    string         `json:"reviewed_by"`
	ReviewedAt    string         `json:"reviewed_at"`
	Files         []evidenceFile `json:"files"`
}

type evidenceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// RequiredEvidenceFiles returns the fixed v1 human-review document set.
func RequiredEvidenceFiles() []string {
	return append([]string(nil), requiredEvidenceFiles...)
}

// WriteEvidenceManifest seals the reviewed evidence documents without
// modifying them. The manifest is immutable and fails if it already exists.
func WriteEvidenceManifest(directory string, identity EvidenceIdentity) (string, error) {
	if err := validateEvidenceIdentity(identity, true); err != nil {
		return "", err
	}
	files, err := inspectEvidenceFiles(directory)
	if err != nil {
		return "", err
	}
	manifest := evidenceManifest{
		SchemaVersion: evidenceSchema,
		Repository:    identity.Repository,
		Version:       identity.Version,
		Commit:        identity.Commit,
		Tree:          identity.Tree,
		ReviewStatus:  "reviewed",
		ReviewedBy:    identity.ReviewedBy,
		ReviewedAt:    identity.ReviewedAt,
		Files:         files,
	}
	contents, err := indentedJSON(manifest)
	if err != nil {
		return "", err
	}
	path := filepath.Join(directory, evidenceManifestName)
	if err := writeExclusive(path, contents, 0o444); err != nil {
		return "", fmt.Errorf("write release evidence manifest: %w", err)
	}
	return digest(contents), nil
}

// VerifyEvidenceManifest proves that the sealed evidence is for the expected
// source state and that every reviewed byte remains unchanged.
func VerifyEvidenceManifest(directory string, expected EvidenceIdentity) error {
	if err := validateEvidenceIdentity(expected, false); err != nil {
		return err
	}
	manifestPath := filepath.Join(directory, evidenceManifestName)
	contents, err := readEvidenceFile(manifestPath, true)
	if err != nil {
		return fmt.Errorf("read release evidence manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var manifest evidenceManifest
	if err := decoder.Decode(&manifest); err != nil {
		return fmt.Errorf("decode release evidence manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode release evidence manifest: %w", err)
	}
	if manifest.SchemaVersion != evidenceSchema {
		return fmt.Errorf("release evidence schema = %q, want %q", manifest.SchemaVersion, evidenceSchema)
	}
	if manifest.ReviewStatus != "reviewed" {
		return errors.New("release evidence review status must be reviewed")
	}
	actual := EvidenceIdentity{
		Repository: manifest.Repository,
		Version:    manifest.Version,
		Commit:     manifest.Commit,
		Tree:       manifest.Tree,
		ReviewedBy: manifest.ReviewedBy,
		ReviewedAt: manifest.ReviewedAt,
	}
	if err := validateEvidenceIdentity(actual, true); err != nil {
		return err
	}
	if actual.Repository != expected.Repository || actual.Version != expected.Version ||
		actual.Commit != expected.Commit || actual.Tree != expected.Tree {
		return errors.New("release evidence repository, version, commit, or tree does not match the candidate")
	}

	observed, err := inspectEvidenceFiles(directory)
	if err != nil {
		return err
	}
	if len(manifest.Files) != len(observed) {
		return fmt.Errorf("release evidence manifest contains %d files, want %d", len(manifest.Files), len(observed))
	}
	for index := range observed {
		if manifest.Files[index] != observed[index] {
			return fmt.Errorf("release evidence file %q is missing, reordered, or has a changed digest", observed[index].Path)
		}
	}
	return nil
}

func inspectEvidenceFiles(directory string) ([]evidenceFile, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, fmt.Errorf("inspect evidence directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("evidence directory must be a real directory, not a symlink")
	}
	files := make([]evidenceFile, 0, len(requiredEvidenceFiles))
	for _, name := range requiredEvidenceFiles {
		contents, err := readEvidenceFile(filepath.Join(directory, name), false)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", name, err)
		}
		if !utf8.Valid(contents) || bytes.IndexByte(contents, 0) >= 0 {
			return nil, fmt.Errorf("%s must be NUL-free UTF-8 text", name)
		}
		if !strings.Contains(string(contents), "# ") {
			return nil, fmt.Errorf("%s must contain a Markdown heading", name)
		}
		files = append(files, evidenceFile{Path: name, SHA256: digest(contents)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func readEvidenceFile(path string, manifest bool) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("must be a regular file, not a symlink")
	}
	if info.Size() == 0 || info.Size() > maximumEvidenceSize {
		return nil, fmt.Errorf("size must be between 1 and %d bytes", maximumEvidenceSize)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if manifest && len(contents) > maximumEvidenceSize {
		return nil, errors.New("manifest exceeds the size limit")
	}
	return contents, nil
}

func validateEvidenceIdentity(identity EvidenceIdentity, requireReview bool) error {
	if strings.TrimSpace(identity.Repository) == "" || strings.ContainsAny(identity.Repository, "\r\n\x00") {
		return errors.New("evidence repository is required and must be one line")
	}
	if !releaseVersionPattern.MatchString(identity.Version) {
		return errors.New("evidence version must be a canonical v-prefixed semantic version")
	}
	if !gitObjectPattern.MatchString(identity.Commit) || !gitObjectPattern.MatchString(identity.Tree) {
		return errors.New("evidence commit and tree must be full lowercase Git object identities")
	}
	if !requireReview {
		return nil
	}
	if strings.TrimSpace(identity.ReviewedBy) == "" || strings.ContainsAny(identity.ReviewedBy, "\r\n\x00") {
		return errors.New("evidence reviewer is required and must be one line")
	}
	reviewedAt, err := time.Parse(time.RFC3339, identity.ReviewedAt)
	if err != nil {
		return fmt.Errorf("evidence review time must be RFC3339: %w", err)
	}
	if reviewedAt.After(time.Now().UTC().Add(5 * time.Minute)) {
		return errors.New("evidence review time cannot be in the future")
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}
