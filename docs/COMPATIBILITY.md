<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Compatibility policy

## Current release-candidate contract

Compiler `v1.0.0-rc.1` and runtime `sando/v1.0.0-rc.1` are the current
semantic-version prereleases. The intended v1 source syntax, generated API,
runtime API, CLI behavior, diagnostics, and configuration schemas are frozen
except for release-blocking corrections. Every correction receives a new
immutable RC, documentation, and deterministic generation evidence.

The RC is not yet the final-v1 support commitment. Maintainers accept and
triage security reports within the boundary described in
[SECURITY.md](../SECURITY.md). A security correction may intentionally fail
closed when retaining behavior would contradict a published safety guarantee.

## Final-v1 contract

At final v1, semantic versions apply independently to the compiler and
`sando` runtime. Generated files record the exact compiler version and
required runtime ABI. Patch releases do not intentionally change accepted
source semantics or generated public signatures. Minor releases may add
fail-closed syntax or API capabilities while continuing to render previously
valid components. Major releases may remove or reinterpret behavior.

Generated files are source artifacts, not a stable interchange format across
compiler versions; `himesan check` defines whether they are current. The
project makes no compatibility promise for internal packages, development SSE
payloads before final v1, or hand-edited generated files.

The v1 compatibility snapshots cover the exported `sando` API and values, CLI
help and exit-code classes, structured operation/version output, diagnostic
codes, `himesan.json`, and generated provenance. English diagnostic wording,
internal packages, temporary paths, and compiler implementation details are not
stable API.

An API deprecated after final v1 remains available for the rest of the v1
major line and may be removed in v2. A security correction may fail closed in
a patch release when retaining old behavior would contradict a published safety
guarantee; that exception receives an advisory and migration note rather than a
silent compatibility claim. Until a broader maintenance policy is announced,
only the latest stable v1 patch and the current prerelease receive fixes.

## Go and platform support

The modules retain a `go 1.25` language directive for consumer compatibility.
The maintained v1 build and verification targets are Linux/amd64 and Apple
Silicon macOS/arm64 using the pinned patched Go 1.26.7 and Go 1.27.0 toolchains.
Both native targets are release blockers. A sleeping or unavailable Mac delays
the release gate rather than silently converting it into Linux or
cross-compilation evidence. Native Windows, Intel macOS, Linux/arm64, and other
targets may work but are not v1 compatibility promises. A Go or platform
support change is announced in release notes before it takes effect.

### Historical Beta 1 observations

The following table is retained because the tests genuinely ran. It records a
point-in-time Beta 1 campaign and does not define the current support matrix.

The current public evidence is the exact Beta 1 source at commit
`b7a84054d755e42285e50298e41e47f06a8325a5` (tree
`be9e118e38dfebed19f60403ededdadabe07d2aa`):

| Platform | Go lanes | Maintainer-run result |
| --- | --- | --- |
| Windows 11/amd64 on NTFS | 1.25.12, 1.26.5 | Native tests, race, vet, builds, generation, process cleanup, watcher boundaries, and temporary consumer compilation passed; privileged symlink and POSIX-only permission cases were not exercised |
| Linux/amd64 on WSL2 with an ext4 checkout | 1.25.12, 1.26.5 | Tests, race, vet, builds, generation, focused filesystem/development cases, and license checks passed |
| Linux/amd64 in isolated containers on a Linux server | 1.25.12, 1.26.5 | The earlier pre-beta baseline passed tests, race, vet, builds, deterministic generation, and license checks; this was not rerun on the exact Beta 1 commit |
| macOS | — | Not executed during the Beta 1 campaign |

The golden generated file had SHA-256
`63fa75a3049a3a8a12d769d7f9b6b510dfe763baacf706775b75cef2c57a984f`
on every tested Windows and Linux lane in that historical campaign.

The signed Beta tags and fresh direct/public-proxy installation were verified
after publication. For Beta 1, add the nested runtime to an application module
before installing the parent compiler at the same version; this avoids a Go
module-cache path-selection ambiguity observed in the reverse order.

## Portability feedback

Developers may try the beta on an unsupported target and report useful gaps. A
good report includes the operating system and architecture, `go version`, the
exact command, and a minimal reproduction or diagnostic output. Ordinary
portability reports belong on the canonical Gitea project. Suspected
vulnerabilities must use the private route in [SECURITY.md](../SECURITY.md).

Community reports can reveal gaps and help prioritize future work. They do not
constitute an independent audit, create a support promise, or shift
responsibility for security review, triage, fixes, and release decisions.
