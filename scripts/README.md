<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Repository verification tools

These scripts are intentionally understandable shell and PowerShell rather than a release framework with hidden defaults.

- `verify.sh` runs root and nested-module tests and vet, builds `himesan`, checks the compiler-owned golden output, and proves two generation passes leave the same bytes and unchanged modification times. Set `HIMESAN_RACE=1` for race tests.
- `verify.ps1` provides the equivalent native Windows lane; pass `-Race` to include the race detector.
- `check-licenses.sh` enforces the AGPL compiler / Apache runtime boundary and prevents generated application Go from inheriting an AGPL identifier.
- `release-check.sh --version vX.Y.Z` is a clean-checkout technical preflight. Add `--public` and point `HIMESAN_RELEASE_EVIDENCE_DIR` at a human-reviewed evidence bundle for the public-launch gate. It never tags, pushes, publishes, or deploys.
- `verify-public-install.sh --version vX.Y.Z` is a post-tag/publication check. It verifies exact `go-get=1` package routes and runs the documented compiler install and runtime get from fresh direct-fetch and public-proxy caches without interactive Git credentials.

The canonical Linux CI and release preflight also run bounded fuzz sessions for the parser/context compiler and Go-aware delimiter scanner. Seed-corpus execution remains part of ordinary `go test`; the bounded sessions are extra evidence, not a substitute for longer scheduled fuzzing before v1.

The release preflight invokes `govulncheck` from the official Go vulnerability project at the exact module version `golang.org/x/vuln@v1.6.0`. Updating that pin requires reviewing the upstream tag and rerunning the supported Go lines.

## Preview automation status

Forge workflows are intentionally excluded from the sanitized pre-1.0 public snapshot until the project has confirmed its own Gitea runner availability and reviewed locally hosted or otherwise pinned dependencies. Local `verify.sh`, `verify.ps1`, license, and release-preflight results are the preview gates.

If Gitea automation is later added to the public repository, pin every external action to a reviewed immutable commit, document its provenance, grant minimum permissions, and keep a local verification path. A secondary forge may host a sanitized, read-only discovery snapshot, but hosted workflows stay disabled there and it does not become a release or contribution authority.
