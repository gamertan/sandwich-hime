<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# v1.0.0 launch initiative

Sandwich Hime v1 is a compatibility and evidence milestone, not a reason to
accumulate features. The intended product is already visible: an HTML-first,
ahead-of-time template engine for Go, typed generated components, a small
HTTP-independent runtime, and an optional opinionated local development loop.

Development records may remain private, but their repository names, branch
names, paths, commit mappings, and history are not release provenance. Public
releases originate only from the reviewed canonical public tree.

## Repository and publication topology

| Surface | Purpose | History and tags |
| --- | --- | --- |
| Private development storage | Working branches, private review records, and historical context | identities and history are not exported; no public release tags |
| Public Gitea `sandwich-hime` | Canonical sanitized source, contribution venue, module origin, and releases | fresh reviewed history; authoritative immutable `sando/vX.Y.Z` and `vX.Y.Z` tags |
| GitHub `gamertan/sandwich-hime` | Discoverability and a convenient sanitized source snapshot | no private refs, force-mirrors, workflows, contribution authority, release artifacts, or semver tags |

Each public update is exported through the exact committed allowlist, inspected,
committed as a fresh public snapshot, and compared byte-for-byte with the
reviewed export. GitHub receives that public tree only. It never receives
private history or an indiscriminate Git mirror.

## Current readiness

Beta 1 is deliberately earlier than a release candidate. It creates a real,
repeatable install for learners and evaluators without claiming that the final
v1 compatibility, native-platform, artifact, signing, or soak gates are
complete.

### Demonstrated in the pre-beta public baseline

Public commit `113c95c21e57227b4675c9fda015ada59cc9e9a6` (tree
`a2aeb4dac22853cb3894e3e487b94bbeff5051e5`) passed maintainer-run Go
1.25.12 and Go 1.26.5 matrices on native Windows/amd64, Linux/amd64 under WSL2,
and isolated Linux/amd64 server containers. The same generated golden SHA-256
was observed across those lanes.

That result is a pre-beta baseline only. The exact Beta 1 candidate must rerun
the required Windows/Linux matrix after all version, documentation, and source
changes and before tags are created.

Other demonstrated controls include:

- zero third-party Go module requirements in the compiler and nested `sando`
  runtime;
- a version-specific compile-time runtime ABI marker;
- owned-output, orphan, stale, symlink, nested-module, permission, last-good,
  writer-error, and enumerated contextual-output tests;
- loopback-only, browser-origin-hardened development proxy behavior; and
- a public threat model, security policy, and dated evidence ledger.

### Not demonstrated yet

- native maintainer-run macOS execution; macOS is provisional for Beta 1;
- stable final-v1 API, CLI, schema, diagnostic, and generated snapshots;
- systematic browser-parser and `html/template` differential testing;
- a long semantic fuzz campaign beyond bounded no-panic smoke;
- committed comparative benchmarks and predefined regression thresholds;
- complete real-browser development-supervisor evidence;
- deterministic prebuilt archives, checksums, SBOMs, signed binaries, and
  tested signing/recovery procedures; or
- clean direct and public-proxy installation of the not-yet-published Beta 1
  tags.

## Beta 1 publication lane

Beta 1 is supported for learning, classroom projects, evaluation, prototypes,
and compatibility feedback. It is not recommended as a production-stable
dependency, and its interfaces may change.

- [x] Define beta support, security, compatibility, and macOS-provisional
  language.
- [x] Establish the named public pre-beta Linux/Windows baseline.
- [ ] Rerun the supported Go matrix and deterministic generation on the exact
  Beta 1 candidate.
- [ ] Run the candidate-version freshness, bounded fuzz, vulnerability, and
  license gates.
- [ ] Publish immutable `sando/v1.0.0-beta.1`, then
  `v1.0.0-beta.1`, from the same reviewed public commit.
- [ ] Verify clean direct and public-proxy installs and record the result.
- [ ] Add native macOS maintainer evidence before RC; community reports inform
  that work but do not replace maintainer responsibility.

## Milestone 1: contract freeze

Required before security/platform release-candidate work is declared complete:

- [ ] Decide and specify whether generic component function signatures are v1.
- [ ] Inventory and freeze every exported `sando` symbol, trusted type,
  sentinel error, concrete error field, helper, and ABI marker.
- [ ] Freeze CLI commands, exit-code meanings, diagnostic codes, JSON schemas,
  `himesan.json` schema, and generated provenance fields.
- [ ] Specify nil/stringification behavior, supported HTML-context matrix,
  component trust boundary, URL semantics, and explicit unsupported cases.
- [ ] Add machine-checked public API, CLI, diagnostic, schema, and generated
  output compatibility snapshots.
- [ ] Define the v1 deprecation and security-support policy.

## Milestone 2: security and native-platform evidence

- [ ] Run the minimum supported Go line and the latest two stable Go lines on
  native Linux, macOS, and Windows hosts.
- [ ] Prove identical generated bytes across those hosts and exercise native
  path, replacement, permission, race, process-tree, and watcher behavior.
- [ ] Build a systematic differential corpus against Go's documented
  `html/template` safety baseline for overlapping supported contexts.
- [ ] Parse representative outputs in real browsers and test structure/code
  invariants rather than only byte equality.
- [ ] Extend semantic fuzzing across delimiters, HTML transitions, imports,
  paths, source maps, URL normalization, and filesystem operations.
- [ ] Resolve or explicitly accept every open item in
  `SECURITY_EVIDENCE.md`; no accepted item may contradict a public guarantee.
- [ ] Test delivery and reply through `security@sandwichhime.com`.
- [ ] Define severity, advisory, retraction, and CVE-request handling.

## Milestone 3: measured performance and development UX

- [ ] Commit a synthetic, repository-owned benchmark corpus comparing
  equivalent typed views and output with `html/template`.
- [ ] Define “no material regression” before measuring the release candidate;
  publish hardware, OS, Go version, commands, samples, allocations, and output
  equivalence with every result.
- [ ] Test SSE reconnect, reload, diagnostic overlays, CSP changes, fragment/API
  exclusions, caching, and child cleanup in a real browser on supported hosts.
- [ ] Remove any v1 development-supervisor guarantee that cannot be evidenced
  reliably instead of substituting prose for a test.

## Milestone 4: release rehearsal

- [ ] Make version validation identical in the CLI, generated headers, scripts,
  and release artifacts; reject ambiguous build metadata.
- [ ] Build the candidate compiler at its candidate version and prove its
  committed outputs are current under that exact binary.
- [ ] Produce deterministic archives/binaries, checksums, SBOMs, signatures,
  and source/build provenance from a clean sanitized canonical checkout.
- [ ] Test release-key backup and two-person recovery for Gitea, domains,
  signing material, and publication instructions.
- [ ] Make evidence gates validate content and commit identity rather than only
  the presence of non-empty files.
- [ ] Rehearse runtime-first publication and rollback without creating public
  semver tags.

## Milestone 5: release candidates and final launch

1. Export and review the sanitized canonical release tree.
2. Publish signed `sando/v1.0.0-rc.1`, then signed `v1.0.0-rc.1` from the same
   reviewed public Gitea commit.
3. Verify documented installs through fresh `GOPROXY=direct` and
   `proxy.golang.org` caches on supported Go versions and native platforms.
4. Run the complete evidence suite again from the exact public commit.
5. Operate the official Sandwich Hime website on the RC runtime for a 14-day
   observation period with no unresolved Hime render, security, accessibility,
   or rollback regression. This is product dogfooding, not a dependency on
   another application's private repository.
6. Fix findings in a new RC; restart the observation period when the affected
   boundary warrants it.
7. Finalize the changelog, supported-version table, migration notes, release
   notes, legal/trademark review, checksums, SBOMs, and signatures.
8. Publish `sando/v1.0.0` first and `v1.0.0` second. Never move a tag.
9. Refresh the untagged GitHub discovery snapshot and point it to canonical
   Gitea releases and contribution channels.

## Explicitly deferrable after v1

Unless testing finds a release-blocking consequence, v1 need not include every
possible context, hostile-local filesystem hardening, typed trust-flow analysis,
complete dynamic cycle detection, an encrypted reporting key, or an external
audit. Those limits must remain visible and must not be contradicted by
marketing. New features do not outrank a small stable contract.

## Definition of confidence

“Ready for v1” means a reviewer can trace each promise to a stable public
contract, executable evidence from supported native environments, and a signed
artifact built from the exact canonical source. It does not mean perfect,
invulnerable, or finished forever.
