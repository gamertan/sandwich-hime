<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Proof-in-the-pudding roadmap

Unchecked items are release blockers for the milestone that contains them, not
necessarily blockers for an earlier prerelease. The ordered initiative,
repository topology, release-candidate sequence, and definition of confidence
are maintained in [docs/V1_RELEASE_PLAN.md](docs/V1_RELEASE_PLAN.md).

## Beta 1: public learning and evaluation

Beta 1 deliberately ships before the final-v1 compatibility and artifact gates.
Its scope is classroom use, learning, prototypes, and compatibility feedback;
it is not a production-stability promise.

- [x] Define beta versus RC/final support and compatibility policy.
- [x] Establish a public pre-beta Linux/Windows matrix on Go 1.25 and Go 1.26.
- [x] Document macOS as provisional and invite useful community reports while
  retaining maintainer responsibility for security and releases.
- [ ] Rerun all required Windows/Linux checks and deterministic generation on
  the exact Beta 1 candidate.
- [ ] Publish immutable `sando/v1.0.0-beta.1`, then
  `v1.0.0-beta.1`, from the reviewed public commit.
- [ ] Verify clean direct and public-proxy installs after publication.
- [ ] Complete native macOS maintainer validation. This is an RC/final gate,
  not a Beta 1 gate.

## Compiler and runtime for RC/final

- [ ] Freeze and machine-check the compiler, CLI, diagnostic, schema, generated,
  and runtime compatibility contracts.
- [ ] Repeat compiler-owned deterministic golden output across Linux, macOS,
  and Windows on the exact candidate.
- [ ] Compile temporary consumer modules using committed Go and only the Apache
  runtime.
- [ ] Run the parser, delimiter, context, path, and source-map release fuzz
  campaign.
- [ ] Evidence adversarial escaping and filesystem cases.
- [ ] Pass test, race, vet, vulnerability, and license gates on the latest two
  supported Go lines.
- [ ] Reproduce signed compiler/runtime release artifacts, checksums, and SBOMs.

## Development supervisor for RC/final

- [ ] Generation/build/start/health failures keep the previous healthy server
  live.
- [ ] SSE reconnect/reload and mapped overlay diagnostics pass browser-level
  tests.
- [ ] CSP hash injection, fragment/API/download exclusion, and cache disabling
  pass.
- [ ] Replaced and interrupted child processes leave no descendants on
  supported systems.

## Repository-owned release evidence

- [ ] Differentially test contextual escaping against Go's documented
  `html/template` safety baseline.
- [ ] Reproduce repository-owned synthetic benchmark cases and methodology from
  a clean checkout.
- [ ] Review generated output for stable provenance, source mappings, and
  absence of compiler-license headers.
- [ ] Document the production boundary: committed generated Go plus the Apache
  runtime, with no compiler or development supervisor in the deployed binary.
- [ ] Keep unsupported or unmeasured performance and production claims out of
  release materials.

## Final public launch

- [ ] Complete final human review of ownership notices, output permission, DCO
  contribution process, and pre-registration trademark terms.
- [ ] Complete name clearance, security-mailbox recovery, release signing, and
  two-person credential recovery.
- [ ] Verify `gamertan.com` vanity metadata and documented installs from clean
  machines.
- [ ] Confirm the sanitized public Gitea source contains no private paths,
  identifiers, history, or unsupported claims.
- [ ] Publish and observe a signed RC on every supported native platform.
- [ ] Publish `sando/v1.0.0`, then `v1.0.0`, without moving either tag.
