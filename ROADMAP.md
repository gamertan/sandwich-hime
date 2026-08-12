<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Proof-in-the-pudding roadmap

Unchecked items are release blockers, not aspirational marketing.

## Compiler and runtime

- [ ] Deterministic golden output repeated across Linux, macOS, and Windows.
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

## EQL Wiki proof

- [ ] Separate `codex/himesan-pilot` worktree created after compiler gates.
- [ ] Shared layout/home, browse fragment, and item page reach differential parity.
- [ ] Accessibility, CSP, links/forms, malicious values, status, caching, and fragments pass.
- [ ] Blue/green renderer flag and slot-switch rollback verified.
- [ ] Fourteen continuous production days complete with zero Hime renderer failure or security/accessibility regression.
- [ ] Honest before/after case study and reproducible benchmark report approved.

## Public launch

- [ ] Ownership notices, output permission, DCO contribution process, and pre-registration trademark terms receive final human review.
- [ ] Name clearance, security mailbox, two-person credential recovery, and signing keys complete.
- [ ] Gamertan vanity metadata and documentation verified from a clean machine.
- [ ] Sanitized fresh-history public Gitea snapshot contains no private paths, identifiers, history, or unsupported release claims.
- [ ] Canonical public Gitea source preview and Gamertan documentation launch together with no secondary forge mirror.
