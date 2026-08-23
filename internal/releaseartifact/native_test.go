// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifyNativeReceiptSetRequiresEveryLaneAndMatchingGeneratedOutput(t *testing.T) {
	expected := NativeReceiptExpectation{
		Repository: "gamertan/sandwich-hime",
		Commit:     strings.Repeat("a", 40), Tree: strings.Repeat("b", 40),
	}
	directory := nativeReceiptFixture(t, expected, strings.Repeat("c", 64))
	summary, err := VerifyNativeReceiptSet(directory, expected)
	if err != nil {
		t.Fatal(err)
	}
	if summary.GeneratedDigest != strings.Repeat("c", 64) || len(summary.Lanes) != 4 {
		t.Fatalf("native summary is incomplete: %#v", summary)
	}

	t.Run("source mismatch", func(t *testing.T) {
		mismatch := expected
		mismatch.Commit = strings.Repeat("d", 40)
		if _, err := VerifyNativeReceiptSet(directory, mismatch); err == nil {
			t.Fatal("mismatched source commit was accepted")
		}
	})

	t.Run("generated mismatch", func(t *testing.T) {
		mismatched := nativeReceiptFixture(t, expected, strings.Repeat("c", 64))
		lane := requiredNativeLanes[0]
		laneDirectory := filepath.Join(mismatched, lane.directory)
		if err := os.RemoveAll(laneDirectory); err != nil {
			t.Fatal(err)
		}
		writeNativeLaneFixture(t, laneDirectory, lane, expected, strings.Repeat("e", 64))
		if _, err := VerifyNativeReceiptSet(mismatched, expected); err == nil {
			t.Fatal("different generated output was accepted")
		}
	})
}

func TestVerifyNativeReceiptSetRejectsTamperingAndUnexpectedFiles(t *testing.T) {
	expected := NativeReceiptExpectation{
		Repository: "gamertan/sandwich-hime",
		Commit:     strings.Repeat("a", 40), Tree: strings.Repeat("b", 40),
	}

	t.Run("checksum", func(t *testing.T) {
		directory := nativeReceiptFixture(t, expected, strings.Repeat("c", 64))
		sidecar := filepath.Join(directory, requiredNativeLanes[0].directory, "TEND-CI-VERIFICATION.json.sha256")
		if err := os.Chmod(sidecar, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecar, []byte(strings.Repeat("0", 64)+"  TEND-CI-VERIFICATION.json\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyNativeReceiptSet(directory, expected); err == nil {
			t.Fatal("tampered checksum was accepted")
		}
	})

	t.Run("unexpected entry", func(t *testing.T) {
		directory := nativeReceiptFixture(t, expected, strings.Repeat("c", 64))
		if err := os.WriteFile(filepath.Join(directory, "notes.txt"), []byte("not a receipt"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := VerifyNativeReceiptSet(directory, expected); err == nil {
			t.Fatal("unexpected native evidence entry was accepted")
		}
	})
}

func nativeReceiptFixture(t *testing.T, expected NativeReceiptExpectation, generatedDigest string) string {
	t.Helper()
	directory := t.TempDir()
	for _, lane := range requiredNativeLanes {
		writeNativeLaneFixture(t, filepath.Join(directory, lane.directory), lane, expected, generatedDigest)
	}
	return directory
}

func writeNativeLaneFixture(t *testing.T, directory string, lane nativeLane, expected NativeReceiptExpectation, generatedDigest string) {
	t.Helper()
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	runnerVersion := "v3.1.0"
	if lane.goos == "darwin" {
		runnerVersion = "v3.3.0"
	}
	receipt := Receipt{
		Repository: expected.Repository, Commit: expected.Commit, Tree: expected.Tree,
		GOOS: lane.goos, GOARCH: lane.goarch, GoVersion: lane.goVersion,
		RunnerVersion: runnerVersion, RunnerName: lane.runner,
		GeneratedDigest: generatedDigest, UnsignedArtifactSHA: strings.Repeat("f", 64),
		SuccessfulGates: append([]string(nil), requiredNativeGates...),
		CompletedAt:     time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
	}
	if _, err := WriteReceipt(filepath.Join(directory, "TEND-CI-VERIFICATION.json"), receipt); err != nil {
		t.Fatal(err)
	}
}
