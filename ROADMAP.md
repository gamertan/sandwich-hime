<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Proof-in-the-pudding roadmap

Unchecked items are release blockers for the milestone that contains them, not
necessarily blockers for an earlier prerelease. The ordered initiative,
repository topology, release-candidate sequence, and definition of confidence
are maintained in [docs/V1_RELEASE_PLAN.md](docs/V1_RELEASE_PLAN.md).

## Beta 1: public learning and evaluation (historical)

Beta 1 deliberately ships before the final-v1 compatibility and artifact gates.
Its scope is classroom use, learning, prototypes, and compatibility feedback;
it is not a production-stability promise.

- [x] Define beta versus RC/final support and compatibility policy.
- [x] Establish a one-time public pre-beta Linux/Windows evidence matrix on Go
  1.25 and Go 1.26.
- [x] Record the untested macOS boundary without presenting it as evidence.
- [x] Rerun the historical Windows/Linux campaign and deterministic generation
  on the exact Beta 1 candidate.
- [x] Publish immutable `sando/v1.0.0-beta.1`, then
  `v1.0.0-beta.1`, from the reviewed public commit.
- [x] Verify clean runtime-first direct and public-proxy installs after
  publication.

## Compiler and runtime for RC/final

- [x] Freeze and machine-check the compiler, CLI, diagnostic, schema, generated,
  and runtime compatibility contracts.
- [x] Repeat compiler-owned deterministic golden output across the supported
  Linux and macOS Go lanes on the exact candidate.
- [x] Compile temporary consumer modules using committed Go and only the Apache
  runtime.
- [x] Run the parser, delimiter, context, path, and source-map release fuzz
  campaign.
- [x] Evidence adversarial escaping and filesystem cases.
- [x] Pass test, race, vet, vulnerability, and license gates on the latest two
  supported Go lines.
- [x] Reproduce compiler/runtime release artifacts, checksums, and SBOMs; sign
  and notarize the macOS distribution outside runner authority.

## Development supervisor for RC/final

- [x] Generation/build/start/health failures keep the previous healthy server
  live.
- [x] SSE reconnect/reload and mapped overlay diagnostics pass browser-level
  tests.
- [x] CSP hash injection, fragment/API/download exclusion, and cache disabling
  pass.
- [x] Replaced and interrupted child processes leave no descendants on
  supported systems.

## Repository-owned release evidence

- [x] Differentially test contextual escaping against Go's documented
  `html/template` safety baseline.
- [x] Reproduce repository-owned synthetic benchmark cases and methodology from
  a clean checkout.
- [x] Review generated output for stable provenance, source mappings, and
  absence of compiler-license headers.
- [x] Document the production boundary: committed generated Go plus the Apache
  runtime, with no compiler or development supervisor in the deployed binary.
- [x] Keep unsupported or unmeasured performance and production claims out of
  release materials.

## Final public launch

- [ ] Complete final human review of ownership notices, output permission, DCO
  contribution process, and pre-registration trademark terms.
- [ ] Complete name clearance, security-mailbox recovery, release signing, and
  two-person credential recovery.
- [ ] Verify `gamertan.com` vanity metadata and documented RC installs from clean
  machines.
- [ ] Confirm the sanitized public Gitea source contains no private paths,
  identifiers, history, or unsupported claims.
- [ ] Publish and observe signed RC artifacts on Linux/amd64 and Darwin/arm64.
- [ ] Publish `sando/v1.0.0`, then `v1.0.0`, without moving either tag.
