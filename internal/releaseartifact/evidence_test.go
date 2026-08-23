// SPDX-License-Identifier: AGPL-3.0-only

package releaseartifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceManifestBindsReviewedContentAndIdentity(t *testing.T) {
	directory := evidenceFixture(t)
	identity := evidenceIdentity()
	checksum, err := WriteEvidenceManifest(directory, identity)
	if err != nil {
		t.Fatal(err)
	}
	if !digestPattern.MatchString(checksum) {
		t.Fatalf("manifest checksum = %q", checksum)
	}
	if err := VerifyEvidenceManifest(directory, identity); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, requiredEvidenceFiles[0])
	if err := os.WriteFile(path, []byte("# Changed after review\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEvidenceManifest(directory, identity); err == nil {
		t.Fatal("changed evidence bytes were accepted")
	}
}

func TestEvidenceManifestRejectsCandidateMismatchAndUnknownFields(t *testing.T) {
	t.Run("candidate identity", func(t *testing.T) {
		directory := evidenceFixture(t)
		identity := evidenceIdentity()
		if _, err := WriteEvidenceManifest(directory, identity); err != nil {
			t.Fatal(err)
		}
		identity.Commit = strings.Repeat("d", 40)
		if err := VerifyEvidenceManifest(directory, identity); err == nil {
			t.Fatal("mismatched candidate was accepted")
		}
	})

	t.Run("unknown manifest field", func(t *testing.T) {
		directory := evidenceFixture(t)
		identity := evidenceIdentity()
		if _, err := WriteEvidenceManifest(directory, identity); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, evidenceManifestName)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		contents = []byte(strings.Replace(string(contents), `"files":`, `"unexpected": true, "files":`, 1))
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := VerifyEvidenceManifest(directory, identity); err == nil {
			t.Fatal("unknown manifest field was accepted")
		}
	})
}

func TestEvidenceManifestRejectsSymlinkAndPlaceholderDocuments(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		directory := evidenceFixture(t)
		name := requiredEvidenceFiles[0]
		path := filepath.Join(directory, name)
		target := filepath.Join(directory, "target.md")
		if err := os.WriteFile(target, []byte("# Target\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := WriteEvidenceManifest(directory, evidenceIdentity()); err == nil {
			t.Fatal("symlinked evidence was accepted")
		}
	})

	t.Run("placeholder", func(t *testing.T) {
		directory := evidenceFixture(t)
		if err := os.WriteFile(filepath.Join(directory, requiredEvidenceFiles[0]), []byte("not reviewed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := WriteEvidenceManifest(directory, evidenceIdentity()); err == nil {
			t.Fatal("heading-free placeholder was accepted")
		}
	})
}

func evidenceFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range requiredEvidenceFiles {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("# Reviewed "+name+"\n\nExact bounded evidence.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func evidenceIdentity() EvidenceIdentity {
	return EvidenceIdentity{
		Repository: "gamertan/sandwich-hime",
		Version:    "v1.0.0-rc.1",
		Commit:     strings.Repeat("a", 40),
		Tree:       strings.Repeat("b", 40),
		ReviewedBy: "release operator",
		ReviewedAt: "2026-08-23T18:00:00Z",
	}
}
