<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Changelog

Sandwich Hime follows semantic versioning after final v1. Compiler and nested
runtime releases are versioned independently and listed together when they form
one coordinated release.

## v1.0.0-beta.2 — 2026-08-12

Compiler-only release; the unchanged Apache runtime remains
`sando/v1.0.0-beta.1` with ABI `sando.v1`.

### Added

- Standard-library-only `himesan lsp --stdio` with one workspace per process,
  full-document overlays, 200 ms edit debounce, cancellation, and bounded
  indexing through the compiler's existing filesystem boundaries.
- Live diagnostics, trust warnings, duplicate/cycle reporting, UTF-16 LSP
  positions, tag/context/component hover, document symbols, typed component
  completion, and component go-to-definition.
- Additive `features: ["lsp-stdio"]` in `himesan version --json`.
- Protocol framing and malformed-input fuzzing; overlay, Unicode, CRLF, NUL,
  deletion, nested-module, symlink, completion, definition, cancellation,
  shutdown, no-write, and resource-limit tests.

### Boundaries

The server does not generate, run Go or project code, fetch dependencies,
access the network, start the dev supervisor, format, rename, add imports, or
delegate general Go completion to `gopls`. Generated-file freshness remains an
explicit `himesan check --json` workflow.

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

### Release verification

The exact public Beta 1 commit
`b7a84054d755e42285e50298e41e47f06a8325a5` (tree
`be9e118e38dfebed19f60403ededdadabe07d2aa`) passed maintainer-run
executed Linux and native Windows matrices with Go 1.25.12 and Go 1.26.5. The
tested golden output had the same SHA-256 on each tested host:
`63fa75a3049a3a8a12d769d7f9b6b510dfe763baacf706775b75cef2c57a984f`.

The complete release preflight passed, including race tests, bounded parser
fuzz smoke, known-vulnerability analysis of both zero-third-party-dependency
modules, candidate-version provenance, and six cross-builds. The signed runtime
and compiler tags were published in that order. Fresh direct and public-proxy
runtime-first installs passed after normal proxy propagation. Native macOS
execution remains pending and is explicitly provisional for this beta.

### Known limitations

- Source syntax, generated format, CLI details, and runtime API may change
  before final v1.
- Native macOS behavior has not yet been maintainer-validated.
- In a shared fresh Go module cache, add the nested `sando` runtime before
  installing the parent compiler module at the same Beta 1 version.
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
