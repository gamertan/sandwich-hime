// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const nativeReceiptMaximumAge = 30 * 24 * time.Hour

var runnerVersionPattern = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$`)

type nativeLane struct {
	directory string
	goos      string
	goarch    string
	goVersion string
	runner    string
}

var requiredNativeLanes = []nativeLane{
	{directory: "darwin-arm64-go1.26.7", goos: "darwin", goarch: "arm64", goVersion: "go1.26.7", runner: "macbook-air-himesan-darwin-arm64"},
	{directory: "darwin-arm64-go1.27.0", goos: "darwin", goarch: "arm64", goVersion: "go1.27.0", runner: "macbook-air-himesan-darwin-arm64"},
	{directory: "linux-amd64-go1.26.7", goos: "linux", goarch: "amd64", goVersion: "go1.26.7", runner: "cliff-himesan-linux-amd64"},
	{directory: "linux-amd64-go1.27.0", goos: "linux", goarch: "amd64", goVersion: "go1.27.0", runner: "cliff-himesan-linux-amd64"},
}

var requiredNativeGates = []string{
	"build", "consumer", "contracts", "fuzz", "generation", "package",
	"public-snapshot", "race", "test", "vet", "vulnerability",
}

// NativeReceiptExpectation identifies the exact source verified by all native
// runner lanes.
type NativeReceiptExpectation struct {
	Repository string
	Commit     string
	Tree       string
}

// NativeReceiptSummary is safe to include in human release evidence.
type NativeReceiptSummary struct {
	SchemaVersion   string              `json:"schema_version"`
	Repository      string              `json:"repository"`
	Commit          string              `json:"commit"`
	Tree            string              `json:"tree"`
	GeneratedDigest string              `json:"generated_output_sha256"`
	Lanes           []NativeLaneSummary `json:"lanes"`
}

// NativeLaneSummary records one verified native receipt without local paths.
type NativeLaneSummary struct {
	Platform       string `json:"platform"`
	GoVersion      string `json:"go_version"`
	RunnerVersion  string `json:"runner_version"`
	ArtifactSHA256 string `json:"unsigned_artifact_sha256"`
	ReceiptSHA256  string `json:"receipt_sha256"`
	CompletedAt    string `json:"completed_at"`
}

// VerifyNativeReceiptSet requires all four maintained target/toolchain lanes,
// validates their checksum sidecars, and proves identical generated output.
func VerifyNativeReceiptSet(directory string, expected NativeReceiptExpectation) (NativeReceiptSummary, error) {
	if strings.TrimSpace(expected.Repository) == "" || strings.ContainsAny(expected.Repository, "\r\n\x00") {
		return NativeReceiptSummary{}, errors.New("native receipt repository is required and must be one line")
	}
	if !gitObjectPattern.MatchString(expected.Commit) || !gitObjectPattern.MatchString(expected.Tree) {
		return NativeReceiptSummary{}, errors.New("native receipt commit and tree must be full lowercase Git object identities")
	}
	if err := validateNativeDirectory(directory); err != nil {
		return NativeReceiptSummary{}, err
	}

	summary := NativeReceiptSummary{
		SchemaVersion: "himesan.native-receipt-set.v1",
		Repository:    expected.Repository, Commit: expected.Commit, Tree: expected.Tree,
	}
	for _, lane := range requiredNativeLanes {
		receipt, receiptDigest, err := readNativeLane(filepath.Join(directory, lane.directory))
		if err != nil {
			return NativeReceiptSummary{}, fmt.Errorf("verify native lane %s: %w", lane.directory, err)
		}
		if receipt.SchemaVersion != receiptSchema || receipt.Repository != expected.Repository ||
			receipt.Commit != expected.Commit || receipt.Tree != expected.Tree ||
			receipt.GOOS != lane.goos || receipt.GOARCH != lane.goarch ||
			receipt.GoVersion != lane.goVersion || receipt.RunnerName != lane.runner {
			return NativeReceiptSummary{}, fmt.Errorf("native lane %s does not match its source, platform, toolchain, or runner", lane.directory)
		}
		if !runnerVersionPattern.MatchString(receipt.RunnerVersion) || !digestPattern.MatchString(receipt.UnsignedArtifactSHA) {
			return NativeReceiptSummary{}, fmt.Errorf("native lane %s has an invalid runner or artifact identity", lane.directory)
		}
		completedAt, err := time.Parse(time.RFC3339, receipt.CompletedAt)
		if err != nil || completedAt.After(time.Now().UTC().Add(5*time.Minute)) || time.Since(completedAt) > nativeReceiptMaximumAge {
			return NativeReceiptSummary{}, fmt.Errorf("native lane %s is not fresh, completed, RFC3339 evidence", lane.directory)
		}
		if err := requireNativeGates(receipt.SuccessfulGates); err != nil {
			return NativeReceiptSummary{}, fmt.Errorf("native lane %s: %w", lane.directory, err)
		}
		if summary.GeneratedDigest == "" {
			summary.GeneratedDigest = receipt.GeneratedDigest
		} else if summary.GeneratedDigest != receipt.GeneratedDigest {
			return NativeReceiptSummary{}, errors.New("native lanes did not produce identical generated output")
		}
		summary.Lanes = append(summary.Lanes, NativeLaneSummary{
			Platform: lane.goos + "/" + lane.goarch, GoVersion: lane.goVersion,
			RunnerVersion: receipt.RunnerVersion, ArtifactSHA256: receipt.UnsignedArtifactSHA,
			ReceiptSHA256: receiptDigest, CompletedAt: receipt.CompletedAt,
		})
	}
	return summary, nil
}

func validateNativeDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil {
		return fmt.Errorf("inspect native evidence directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("native evidence must be a real directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read native evidence directory: %w", err)
	}
	expected := make([]string, 0, len(requiredNativeLanes))
	for _, lane := range requiredNativeLanes {
		expected = append(expected, lane.directory)
	}
	observed := make([]string, 0, len(entries))
	for _, entry := range entries {
		observed = append(observed, entry.Name())
	}
	sort.Strings(expected)
	sort.Strings(observed)
	if !equalStrings(expected, observed) {
		return fmt.Errorf("native evidence directories = %v, want exactly %v", observed, expected)
	}
	return nil
}

func readNativeLane(directory string) (Receipt, string, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return Receipt{}, "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return Receipt{}, "", errors.New("lane must be a real directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return Receipt{}, "", err
	}
	if len(entries) != 2 || entries[0].Name() != "TEND-CI-VERIFICATION.json" || entries[1].Name() != "TEND-CI-VERIFICATION.json.sha256" {
		return Receipt{}, "", errors.New("lane must contain only its receipt and checksum sidecar")
	}
	receiptPath := filepath.Join(directory, "TEND-CI-VERIFICATION.json")
	contents, err := readRegularFile(receiptPath, maximumEvidenceSize)
	if err != nil || len(contents) == 0 {
		return Receipt{}, "", errors.New("receipt must be a non-empty bounded regular file")
	}
	receiptDigest := digest(contents)
	sidecar, err := readRegularFile(receiptPath+".sha256", 512)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("read receipt checksum: %w", err)
	}
	expectedSidecar := receiptDigest + "  TEND-CI-VERIFICATION.json\n"
	if string(sidecar) != expectedSidecar {
		return Receipt{}, "", errors.New("receipt checksum sidecar does not match")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var receipt Receipt
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, "", fmt.Errorf("decode receipt: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Receipt{}, "", fmt.Errorf("decode receipt: %w", err)
	}
	if err := validateReceipt(receipt); err != nil {
		return Receipt{}, "", err
	}
	return receipt, receiptDigest, nil
}

func requireNativeGates(gates []string) error {
	observed := make(map[string]bool, len(gates))
	for _, gate := range gates {
		if observed[gate] {
			return fmt.Errorf("successful gate %q is duplicated", gate)
		}
		observed[gate] = true
	}
	for _, gate := range requiredNativeGates {
		if !observed[gate] {
			return fmt.Errorf("required successful gate %q is missing", gate)
		}
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
