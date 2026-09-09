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

Version 1.0.0 was published on September 9, 2026 from public commit
`d978994b274db223370ad15c8291230e1b60d9ca`. The signed runtime tag preceded the
signed compiler tag; both are immutable. The
[canonical release](https://gitea.speelman.ca/gamertan/sandwich-hime/releases/tag/v1.0.0)
provides the notarized Apple Silicon DMG and Linux/amd64 archive with checksums.
All four maintained native/toolchain lanes and final preflight passed. Fresh
runtime-first direct and public-proxy/checksum-database installs passed on
macOS/arm64 and Linux/amd64. Evidence remains bound to that source, not to later
documentation commits. Private development and archival history stays private.

Use [RELEASE.md](RELEASE.md) for subsequent immutable releases.
Passing CI is evidence for that review, not an automatic publication decision.

- [x] Finalize the maintainer-approved individual [contribution agreement](CLA.md)
  version 1.0 and explicit prospective acceptance process. Preserve the AGPL
  compiler, Apache runtime, chosen application license and existing DCO/output
  grants. Publication is not contributor acceptance or retroactive assent.
  The maintainer chose to proceed without an outside legal-review prerequisite;
  see [the rationale](docs/LICENSING_INTENT.md).
- [x] Record the maintainer's approval to proceed with ownership notices, output
  permission, reciprocal contribution terms and pre-registration naming scope.
  This is not a statement of external legal review or guaranteed enforceability.
- [x] Record the maintainer's decision to proceed with Sandwich Hime as the
  primary project identity and Hime-san as its tool name. This is acceptance of
  the documented name-review limitations, not formal trademark clearance,
  completed similarity analysis or a registration requirement.
- [x] Complete final security/signing readiness review. Distinguish successful
  signing from the explicitly deferred recovery drills below.
- [x] Verify `gamertan.com` vanity metadata and documented RC installs using clean
  direct-fetch and public-proxy caches. Final-version installs also passed after
  publication on both maintained platforms; an independent Mac installation
  remains an accepted follow-up below.
- [x] Confirm the sanitized public Gitea source contains no private paths,
  identifiers, history, or unsupported claims.
- [x] Publish signed RC.1 artifacts on Linux/amd64 and Darwin/arm64.
- [x] Record the maintainer's v1 acceptance of existing live dogfooding despite
  missing timed checkpoint notes. This is an explicit assurance-gap acceptance,
  not reconstructed reviews or a claim of measured error-free operation. Missing
  notes alone do not restart the observation period or block release.
- [x] Publish `sando/v1.0.0`, then `v1.0.0`, without moving either tag.

## Accepted assurance follow-ups

- [ ] Exercise the published Mac download/install and CLI on independent Apple
  Silicon hardware when available. The maintainer accepted this gap for v1 and
  prefers an independent machine over another profile on the development Mac.
  Record the result in documentation; fix any reproduced defect in an appropriate
  patch release. This is not a current launch blocker or completed install test.
- [ ] Exercise offline signing-key restoration and independent second-person
  credential recovery/verification. The maintainer explicitly deferred these
  for v1.0.0; they are not launch blockers or completed recovery evidence. Loss
  of the current credentials or operator access remains an incident-response
  risk until these drills and independent access are proven.
