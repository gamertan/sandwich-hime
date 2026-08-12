<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Changelog

Sandwich Hime follows semantic versioning after final v1. Compiler and nested
runtime releases are versioned independently and listed together when they form
one coordinated release.

## v1.0.0-beta.1 — 2026-08-12

This is the first installable public beta: `sando/v1.0.0-beta.1` for the
Apache-2.0 runtime and `v1.0.0-beta.1` for the compiler and CLI. The beta is
for learning, classroom projects, evaluation, and compatibility feedback. It is
not a production-stability promise.

### Added

- Typed one-component `.sando` language and explicit `<?sando go` target.
- Context-annotated renderer IR with deterministic, atomic Go generation.
- Read-only stale-output checking and structured diagnostics.
- Independent Apache-2.0 `sando` component/runtime ABI.
- Loopback-only last-good development supervisor with SSE reload and diagnostic
  overlay.
- Compiler-owned deterministic golden fixture and standalone release gates.
- Multi-license, security, governance, trademark, AI contribution, and release
  policies.
- Public beta support policy for evaluation and classroom use, including a
  provisional macOS lane and a community compatibility-reporting path.

### Pre-beta verification baseline

Maintainer-run Linux and native Windows matrices passed on public commit
`113c95c21e57227b4675c9fda015ada59cc9e9a6` (tree
`a2aeb4dac22853cb3894e3e487b94bbeff5051e5`) with Go 1.25.12 and Go
1.26.5. The tested golden output had the same SHA-256 on each tested host:
`63fa75a3049a3a8a12d769d7f9b6b510dfe763baacf706775b75cef2c57a984f`.

That commit is a pre-beta baseline, not evidence for the later Beta 1 commit.
The required matrix must be rerun from the exact candidate before its tags are
published. Native macOS execution remains pending and is explicitly provisional
for this beta.

### Known limitations

- Source syntax, generated format, CLI details, and runtime API may change
  before final v1.
- Native macOS behavior has not yet been maintainer-validated.
- Prebuilt binary artifacts, checksums, SBOMs, reproducible archives,
  key-recovery rehearsal, systematic browser differential, long fuzz,
  benchmark, and final compatibility gates remain work toward the release
  candidate and final v1. Beta 1 itself is a signed source/module release.
- The project has no independent security audit, certification, or formal
  verification.

### Removed since the private prototype

- Unpublished `.go.hime` syntax and the 2025 generated API.
- Injected helper directories, nested demo modules, Go plugins, and manually
  repaired generated output.
- Repository-bundled application examples and deployment-specific evidence.
- Placeholder novelty commands that did not perform project work.

Private prototype history is intentionally outside the sanitized public
repository. The public changelog begins with Beta 1.
