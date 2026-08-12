<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Proof-in-the-pudding roadmap

Unchecked items are release blockers, not aspirational marketing.

## Compiler and runtime

- [ ] Compiler-owned deterministic golden output repeated across Linux, macOS, and Windows.
- [ ] Temporary consumer modules compile using committed Go and only the Apache runtime.
- [ ] Parser, delimiter, context, path, and source-map fuzz targets survive the release campaign.
- [ ] Adversarial escaping and filesystem cases are evidenced.
- [ ] Latest two Go lines pass test, race, vet, vulnerability, and license gates.
- [ ] Signed compiler/runtime release artifacts, checksums, and SBOMs reproduce.

## Development supervisor

- [ ] Generation/build/start/health failures keep the previous healthy server live.
- [ ] SSE reconnect/reload and mapped overlay diagnostics pass browser-level tests.
- [ ] CSP hash injection, fragment/API/download exclusion, and cache disabling pass.
- [ ] Replaced and interrupted child processes leave no descendants on supported systems.

## Repository-owned release evidence

- [ ] Contextual escaping is differentially tested against Go's documented `html/template` safety baseline.
- [ ] Repository-owned synthetic benchmark cases and methodology are reproducible from a clean checkout.
- [ ] Generated output is reviewed for stable provenance, source mappings, and absence of compiler-license headers.
- [ ] Production application boundaries are documented: committed generated Go plus the Apache runtime, with no compiler or development supervisor in the deployed binary.
- [ ] Unsupported or unmeasured performance and production claims are absent from release materials.

## Public launch

- [ ] Ownership notices, output permission, DCO contribution process, and pre-registration trademark terms receive final human review.
- [ ] Name clearance, security mailbox, two-person credential recovery, and signing keys complete.
- [ ] `gamertan.com` vanity-import metadata and documented installs verified from a clean machine.
- [ ] Sanitized fresh-history public Gitea snapshot contains no private paths, identifiers, history, or unsupported release claims.
- [x] Canonical public Gitea source and project documentation launch, with any
  secondary forge explicitly limited to a sanitized discovery snapshot.
