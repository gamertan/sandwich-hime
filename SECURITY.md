<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Security policy

Sandwich Hime is a public pre-1.0 source preview. Security reports are welcome
now, even though no version is currently designated as supported for production
use. The project would rather receive a careful early report than project
confidence it has not earned.

## Supported versions

| Version | Security status |
| --- | --- |
| Public `main` source preview | Best-effort assessment and fixes; interfaces may change |
| Versioned releases | None published yet |

This table will name supported release lines once immutable compiler and runtime
versions are published. A pre-1.0 release is not a promise of API stability or
fitness for a particular application.

## Report a vulnerability privately

Email **security@sandwichhime.com**. Please do not put an undisclosed
vulnerability, working exploit, credential, secret, or personal data in a
public issue.

Helpful reports include:

- the affected compiler/runtime version or exact commit;
- the relevant `.sando` source, generated Go, or development configuration;
- a minimal reproducer and the observed security impact;
- operating system, architecture, Go version, and browser when relevant;
- whether the issue is already public or has a disclosure deadline; and
- a safe way to credit the reporter, or a request to remain anonymous.

Minimize sensitive data. The mailbox is the private reporting route, but
ordinary email is not end-to-end encrypted. Do not send production secrets or
unnecessary personal data. An encryption key will be published only after its
ownership, backup, and recovery procedure have been tested.

## What to expect

These are best-effort targets for a founder-maintained project, not an SLA:

- acknowledge a report within 7 calendar days;
- provide an initial severity/scope assessment within 14 calendar days when a
  reproducible issue is available; and
- provide an update at least every 30 calendar days while an accepted report
  remains unresolved.

Health, disability, family responsibility, incomplete evidence, or incident
complexity may make those targets impossible. If that happens, the maintainer
will communicate the delay when safely able rather than inventing certainty.

For a reproducible accepted vulnerability, the project aims to retain a private
reproducer where safe, add a regression test where practical, document affected
versions, and agree on a coordinated disclosure plan when appropriate. A fix
may be delivered through a new immutable version, a retraction, an advisory, or
documentation that narrows an incorrect guarantee. Published tags will not be
moved or silently replaced.

## Scope and trust boundary

The most useful reports concern:

- contextual escaping or browser-parser disagreements;
- unsafe URL acceptance or trusted-value boundary confusion;
- parser, generator, path, symlink, ownership, or atomic-write failures;
- generated-code/runtime ABI mismatches;
- deterministic-output or source-provenance failures;
- development proxy exposure, request-origin controls, process cleanup, or
  unintended execution; and
- dependency, release, signing, checksum, or artifact-integrity problems.

Templates and embedded Go are trusted application source. Sandwich Hime is not
a sandbox for an untrusted template author. Handwritten Go implementations of
`sando.Component` and explicit `sando.Trust*` calls are trusted output
capabilities. Application routing, authorization, HTTP headers, database
security, deployment, and production process isolation remain application
responsibilities unless a defect originates in Sandwich Hime itself.

The complete boundary and known non-goals are maintained in
[the threat model](docs/THREAT_MODEL.md). Reproducible assessment results and
open gaps are recorded separately in
[the security evidence ledger](docs/SECURITY_EVIDENCE.md).

## Good-faith research

Good-faith research means making a reasonable effort to:

- test only systems, repositories, and data you own or are authorized to test;
- prefer local reproductions and the smallest proof necessary;
- stop if testing risks availability, privacy, data integrity, or another
  person's account;
- avoid persistence, destructive changes, social engineering, spam, denial of
  service, credential collection, and unnecessary data access;
- retain and transmit the minimum sensitive information required; and
- allow reasonable time for investigation before public disclosure.

This policy permits research on local copies of the source. It does not
authorize active testing of project-operated websites, Gitea infrastructure,
or third-party deployments without separate written permission. Passively
observed issues are welcome. It does not create a bug bounty, safe-harbor
contract, embargo obligation, or promise that every report can be accepted.
The project will not pursue action against research that the maintainer
reasonably believes followed this policy in good faith, but cannot bind third
parties or override applicable law. When uncertain, contact the security
mailbox before testing.

## Current assurance level

The code has maintainer-led threat modeling, adversarial unit and integration
tests, race testing, bounded fuzz smoke tests, static analysis, dependency
inventory, and known-vulnerability scanning. The evidence ledger records those
maintainer-run checks against named commits and dates; its results are
point-in-time evidence, not continuous assurance. The project has not received
an independent security audit, certification, or formal verification. Coverage
percentages, passing scanners, and a clean vulnerability database result are
evidence of specific checks—not proof that no vulnerability exists.

Release artifacts and tags are intended to carry signatures, checksums, an
SBOM, and exact source/build provenance. Those controls are publication gates
until the first versioned release is actually available.

This policy is practical project guidance, not legal advice.
