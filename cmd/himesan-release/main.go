// SPDX-License-Identifier: AGPL-3.0-only

// Command himesan-release creates deterministic unsigned artifacts and native
// verification receipts. Signing and notarization intentionally remain outside
// this command and outside unattended runner authority.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gamertan.com/sandwich-hime/internal/releaseartifact"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "himesan-release: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: himesan-release <package|receipt|evidence-manifest|verify-evidence|verify-native|extract-macos|finalize-macos> [options]")
	}
	switch arguments[0] {
	case "package":
		return runPackage(arguments[1:])
	case "receipt":
		return runReceipt(arguments[1:])
	case "evidence-manifest":
		return runEvidenceManifest(arguments[1:])
	case "verify-evidence":
		return runVerifyEvidence(arguments[1:])
	case "verify-native":
		return runVerifyNative(arguments[1:])
	case "extract-macos":
		return runExtractMacOS(arguments[1:])
	case "finalize-macos":
		return runFinalizeMacOS(arguments[1:])
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func runVerifyNative(arguments []string) error {
	flags := flag.NewFlagSet("verify-native", flag.ContinueOnError)
	var directory string
	var expected releaseartifact.NativeReceiptExpectation
	flags.StringVar(&directory, "directory", "", "four-lane native receipt directory")
	flags.StringVar(&expected.Repository, "repository", "", "repository identity")
	flags.StringVar(&expected.Commit, "commit", "", "source commit")
	flags.StringVar(&expected.Tree, "tree", "", "source tree")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	summary, err := releaseartifact.VerifyNativeReceiptSet(directory, expected)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(summary)
}

func runExtractMacOS(arguments []string) error {
	flags := flag.NewFlagSet("extract-macos", flag.ContinueOnError)
	var archive, checksum, output string
	flags.StringVar(&archive, "archive", "", "unsigned Darwin/arm64 archive")
	flags.StringVar(&checksum, "sha256", "", "approved archive SHA-256")
	flags.StringVar(&output, "output", "", "empty extraction parent directory")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	root, err := releaseartifact.ExtractVerifiedMacOSPackage(archive, checksum, output)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"root": root, "unsigned_archive_sha256": checksum})
}

func runFinalizeMacOS(arguments []string) error {
	flags := flag.NewFlagSet("finalize-macos", flag.ContinueOnError)
	var options releaseartifact.MacOSSigningOptions
	flags.StringVar(&options.Directory, "directory", "", "extracted signed distribution directory")
	flags.StringVar(&options.UnsignedArchiveSHA256, "unsigned-archive-sha256", "", "approved unsigned archive SHA-256")
	flags.StringVar(&options.Identity, "identity", "", "Developer ID identity")
	flags.StringVar(&options.Identifier, "identifier", "", "signed binary identifier")
	flags.StringVar(&options.FinalizedAt, "finalized-at", "", "RFC3339 finalization time")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if err := releaseartifact.FinalizeSignedMacOSDistribution(options); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"valid": true, "directory": options.Directory})
}

func runEvidenceManifest(arguments []string) error {
	flags := flag.NewFlagSet("evidence-manifest", flag.ContinueOnError)
	var directory string
	var identity releaseartifact.EvidenceIdentity
	flags.StringVar(&directory, "directory", "", "reviewed evidence directory")
	bindEvidenceIdentityFlags(flags, &identity, true)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	checksum, err := releaseartifact.WriteEvidenceManifest(directory, identity)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"manifest": filepath.Join(directory, "RELEASE-EVIDENCE.json"), "sha256": checksum})
}

func runVerifyEvidence(arguments []string) error {
	flags := flag.NewFlagSet("verify-evidence", flag.ContinueOnError)
	var directory string
	var identity releaseartifact.EvidenceIdentity
	flags.StringVar(&directory, "directory", "", "sealed evidence directory")
	bindEvidenceIdentityFlags(flags, &identity, false)
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if err := releaseartifact.VerifyEvidenceManifest(directory, identity); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"valid": true, "files": releaseartifact.RequiredEvidenceFiles()})
}

func bindEvidenceIdentityFlags(flags *flag.FlagSet, identity *releaseartifact.EvidenceIdentity, review bool) {
	flags.StringVar(&identity.Repository, "repository", "", "canonical repository identity")
	flags.StringVar(&identity.Version, "version", "", "candidate semantic version")
	flags.StringVar(&identity.Commit, "commit", "", "source commit")
	flags.StringVar(&identity.Tree, "tree", "", "source tree")
	if review {
		flags.StringVar(&identity.ReviewedBy, "reviewed-by", "", "human reviewer identity")
		flags.StringVar(&identity.ReviewedAt, "reviewed-at", "", "RFC3339 review time")
	}
}

func runPackage(arguments []string) error {
	flags := flag.NewFlagSet("package", flag.ContinueOnError)
	var options releaseartifact.PackageOptions
	flags.StringVar(&options.Version, "version", "", "candidate semantic version")
	flags.StringVar(&options.Commit, "commit", "", "source commit")
	flags.StringVar(&options.Tree, "tree", "", "source tree")
	flags.StringVar(&options.GoVersion, "go-version", "", "Go toolchain identity")
	flags.StringVar(&options.GOOS, "goos", "", "target operating system")
	flags.StringVar(&options.GOARCH, "goarch", "", "target architecture")
	flags.StringVar(&options.BinaryPath, "binary", "", "unsigned native binary")
	flags.StringVar(&options.LicensePath, "license", "LICENSE", "license text")
	flags.StringVar(&options.ReleaseNotes, "release-notes", "RELEASE.md", "release notes")
	flags.StringVar(&options.OutputDirectory, "output", "", "output directory")
	flags.Int64Var(&options.SourceDateEpoch, "source-date-epoch", 0, "fixed Unix timestamp")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	result, err := releaseartifact.Package(options)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func runReceipt(arguments []string) error {
	flags := flag.NewFlagSet("receipt", flag.ContinueOnError)
	var receipt releaseartifact.Receipt
	var output, gates, generatedFiles string
	flags.StringVar(&output, "output", "", "receipt output path")
	flags.StringVar(&receipt.Repository, "repository", "", "repository identity")
	flags.StringVar(&receipt.Commit, "commit", "", "source commit")
	flags.StringVar(&receipt.Tree, "tree", "", "source tree")
	flags.StringVar(&receipt.GOOS, "goos", "", "native operating system")
	flags.StringVar(&receipt.GOARCH, "goarch", "", "native architecture")
	flags.StringVar(&receipt.GoVersion, "go-version", "", "Go toolchain identity")
	flags.StringVar(&receipt.RunnerVersion, "runner-version", "", "Gitea Runner version")
	flags.StringVar(&receipt.RunnerName, "runner-name", "", "runner identity")
	flags.StringVar(&receipt.UnsignedArtifactSHA, "artifact-sha256", "", "optional unsigned artifact digest")
	flags.StringVar(&receipt.CompletedAt, "completed-at", "", "RFC3339 completion time")
	flags.StringVar(&gates, "gates", "", "comma-separated successful gates")
	flags.StringVar(&generatedFiles, "generated-files", "", "comma-separated generated output paths")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if output == "" {
		return errors.New("output is required")
	}
	receipt.SuccessfulGates = splitList(gates)
	digest, err := releaseartifact.DigestFiles(splitList(generatedFiles))
	if err != nil {
		return err
	}
	receipt.GeneratedDigest = digest
	checksum, err := releaseartifact.WriteReceipt(output, receipt)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]string{"receipt": output, "sha256": checksum})
}

func splitList(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			values = append(values, item)
		}
	}
	return values
}
