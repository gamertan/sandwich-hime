<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Repository verification tools

These scripts are intentionally understandable shell rather than a release
framework with hidden defaults. The maintained native verification paths are
Linux/amd64 and macOS/arm64.

- `verify.sh` runs root and nested-module tests and vet, builds `himesan`, checks the compiler-owned golden output, and proves two generation passes leave the same bytes and unchanged modification times. Set `HIMESAN_RACE=1` for race tests.
- `check-licenses.sh` enforces the AGPL compiler / Apache runtime boundary and prevents generated application Go from inheriting an AGPL identifier.
- `test-public-snapshot.sh` proves the exact allowlist, secret/path scanner, legal boundary, deterministic manifest, and no-overwrite export behavior on both maintained native hosts.
- Contract tests bind the exported runtime API, CLI help, JSON/configuration schemas, diagnostic-code inventory, generic component signature, and generated provenance to the reviewed files under `contracts/` and `sando/testdata/`.
- `release-check.sh --version vX.Y.Z` is a clean-checkout technical preflight, including exact candidate-version and generated-provenance checks. Beta publication follows the narrower prerelease gates in `RELEASE.md`; release candidates and final v1 additionally use `--public` with a human-reviewed `HIMESAN_RELEASE_EVIDENCE_DIR` and the four-lane `HIMESAN_NATIVE_EVIDENCE_DIR`. Seal the review directory with `go run ./cmd/himesan-release evidence-manifest`; `verify-native` independently checks every native receipt sidecar, source identity, gate, freshness bound, and generated-output digest. The script never tags, pushes, publishes, or deploys.
- `verify-public-install.sh --version vX.Y.Z` is a post-tag/publication check. It verifies exact `go-get=1` package routes, adds the nested runtime before installing the parent compiler, and exercises fresh direct-fetch and public-proxy caches without interactive Git credentials.
- `package-native.sh` performs two native builds and uses the repository-owned Go packager for a deterministic archive, manifest, SBOM, and checksums. `package-macos.sh` is the explicit Apple Silicon entrypoint used by the release operator.
- `sign-notarize-macos.sh` is a deliberately manual boundary. It requires the explicitly approved unsigned archive digest, uses Cole's Developer ID and Keychain-held notary profile, regenerates provenance and checksums for the changed signed Mach-O bytes, and produces a signed, notarized, and stapled DMG. The native runner receives neither credential.
- `verify-real-browser.sh` is opt-in release evidence. It runs the development client in an actual reviewed Chrome/Chromium binary, exercising CSP-restricted execution, SSE diagnostics, reload, and fragment/API exclusions, then reruns the process cleanup integration cases. Chrome is not a normal build or consumer dependency.

The canonical Linux and macOS CI gates run the contract and public-snapshot
checks plus bounded fuzz sessions for the parser/context compiler, Go-aware
delimiter scanner, URL policy, and LSP boundaries. The compiler target also
asserts deterministic diagnostics and generated Go, source-digest binding, and
safe source-map directives. Seed-corpus execution remains part of ordinary
`go test`; the bounded sessions are extra evidence, not a substitute for the
long exact-candidate campaign before v1.

The release preflight invokes `govulncheck` from the official Go vulnerability project at the exact module version `golang.org/x/vuln@v1.6.0`. Updating that pin requires reviewing the upstream tag and rerunning the supported Go lines.

## Preview automation status

Forge workflows are intentionally excluded from the sanitized pre-1.0 public
snapshot. The private development repository uses pinned Linux/amd64 and
repository-scoped native macOS/arm64 runners; the public source remains
independently verifiable with the repository scripts and both native release
preflights.

The private `public-candidate-verification` workflow is a release controller,
not public-source evidence by association. It accepts only the exact lowercase
commit currently at canonical public `main`, clones only that fixed Gitea origin without credentials,
and produces receipts naming `gamertan/sandwich-hime`. Development-repository
receipts cannot satisfy the public release preflight.

If Gitea automation is later added to the public repository, pin every external action to a reviewed immutable commit, document its provenance, grant minimum permissions, and keep a local verification path. A secondary forge may host a sanitized, read-only discovery snapshot, but hosted workflows stay disabled there and it does not become a release or contribution authority.
