<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Compatibility policy

## Beta contract

Compiler `v1.0.0-beta.2` and runtime `sando/v1.0.0-beta.1` are semantic-version
prereleases. They are supported for learning, classroom projects, evaluation,
and compatibility testing. Before final v1, source syntax, generated output,
the runtime API, CLI behavior, diagnostics, and configuration may change
without compatibility shims. Every public change must still be documented and
generation must remain deterministic.

The beta is not a production-stability commitment. Maintainers accept and
triage security reports within the boundary described in
[SECURITY.md](../SECURITY.md), but cannot promise that a beta fix preserves its
public API.

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

## Go and platform support

The current beta targets Go 1.25 and Go 1.26. Support is based on point-in-time,
maintainer-run release matrices, not an implication of continuous CI coverage.
A Go support change is announced in release notes before it takes effect.

The current public evidence is the exact Beta 1 source at commit
`b7a84054d755e42285e50298e41e47f06a8325a5` (tree
`be9e118e38dfebed19f60403ededdadabe07d2aa`):

| Platform | Go lanes | Maintainer-run result |
| --- | --- | --- |
| Windows 11/amd64 on NTFS | 1.25.12, 1.26.5 | Native tests, race, vet, builds, generation, process cleanup, watcher boundaries, and temporary consumer compilation passed; privileged symlink and POSIX-only permission cases were not exercised |
| Linux/amd64 on WSL2 with an ext4 checkout | 1.25.12, 1.26.5 | Tests, race, vet, builds, generation, focused filesystem/development cases, and license checks passed |
| Linux/amd64 in isolated containers on a Linux server | 1.25.12, 1.26.5 | The earlier pre-beta baseline passed tests, race, vet, builds, deterministic generation, and license checks; this was not rerun on the exact Beta 1 commit |
| macOS | — | Native maintainer validation pending; provisional for Beta 1 |

The golden generated file had SHA-256
`63fa75a3049a3a8a12d769d7f9b6b510dfe763baacf706775b75cef2c57a984f`
on every tested Windows and Linux lane.

The signed Beta tags and fresh direct/public-proxy installation were verified
after publication. For Beta 1, add the nested runtime to an application module
before installing the parent compiler at the same version; this avoids a Go
module-cache path-selection ambiguity observed in the reverse order.

## macOS feedback

Mac learners, teachers, and Go developers are warmly invited to try the beta.
A useful compatibility report includes the macOS version, Intel or Apple
Silicon architecture, `go version`, the exact command, and a minimal
reproduction or diagnostic output. Ordinary compatibility reports belong on
the canonical Gitea project. Suspected vulnerabilities must use the private
route in [SECURITY.md](../SECURITY.md).

Community reports can reveal gaps and help prioritize maintainer testing. They
do not constitute an independent audit or shift responsibility for security
review, triage, fixes, and release decisions to the community.
