<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Shared tools, commercial freedom

The design principle is **earn from useful work; preserve the shared freedoms
that made the work possible**. Commercial users are welcome. Sponsorship is
appreciated, not a license fee or a claim to project control. The intended users
are software developers and maintainers; v1 is a maintained compatibility
commitment, not a declaration that development has ended.

This document explains the [contribution agreement](../CLA.md) and official governance.
It is not a new software license, a downstream contract or a claim that every
future use or derivative of every component must be open source.

## Four different jobs

- The software license supplies recipients' permissions and obligations.
- The CLA records the rights for submitted contributions and reciprocal
  promises by the named official steward; it is not a contract with every user.
- Governance determines official decisions, repository access and succession.
- The trademark policy addresses origin and endorsement, not ownership of
  ideas or a veto on honest independent development.

Open-source commercial freedoms include competition. A subjective ban on
"greed," commercial hosting or businesses the maintainer dislikes would not
preserve those freedoms. We instead use concrete source-sharing, attribution,
no-hidden-relicensing and stewardship commitments. See the
[Open Source Definition](https://opensource.org/osd).

## The deliberate compiler/application boundary

The current [license map](../LICENSES.md) remains unchanged:

- Compiler/project material is AGPL-3.0-only. Distribution and covered modified
  network use have source obligations under that license.
- The `sando` runtime is Apache-2.0. That is permissive, not network copyleft;
  compliant closed-source derivatives are possible.
- User templates and generated applications may use their authors' chosen
  terms, subject to input/dependency rights and the explicit output permission.
  A proprietary paid website built with Hime is deliberately possible.

This protects the compiler's shared code without requiring its developers'
applications to become AGPL. Private production adoption is intentional: the
maintainer has confirmed this split for the release. The runtime's rendering,
escaping and URL-safety helpers are valuable implementation work; permissive
embedding makes them useful without imposing compiler licensing on applications.
It does not deliver "every part always open." Neither a later CLA nor a changed
README retracts existing recipients' licenses. No license conversion is proposed.

## Concrete cases

- A consultant sells a website built with Hime: allowed under the existing
  application boundary; no mandatory payment, public badge or CLA for the client.
- A business distributes a modified compiler: it must follow the AGPL's
  applicable source, licensing and notice requirements.
- A business operates a modified AGPL compiler with remote user interaction:
  section 13 requires an offer of that version's Corresponding Source to those
  interacting users. This is not necessarily every unrelated part of its service.
- A business merely hosts an unmodified program: hosting alone does not meet
  section 13's modification condition. Wrappers and combined works require
  fact-specific analysis, not an assumed universal SaaS prohibition.
- A company publishes a complying, independently branded fork: permitted,
  even if it competes successfully and submits no changes upstream.
- A sponsor wants official control or private permission to close contributor
  AGPL work: sponsorship grants neither; the steward covenant rejects
  that official relicensing route. A fork cannot claim official endorsement.

These examples summarize boundaries, not legal opinions on a specific service.
The [AGPL text](https://opensource.org/license/agpl-3.0), including sections 2,
7, 10 and 13, controls actual covered uses. The
[Apache license](https://www.apache.org/licenses/LICENSE-2.0) controls the runtime.
The AGPL permits removing additional permissions from a redistributed copy;
do not promise that every independent fork must keep our output exception.

## What the contribution agreement adds

An identified contributor keeps ownership and grants only the recorded public
license plus an expressly authorized output permission. No alternate broad
sublicensing grant, copyright assignment, forced upstream labor, contributor
indemnity or proprietary buyout permission is collected. Patent rights and
termination follow the relevant established license.

The named Steward promises accurate attribution, preservation of contribution
records, public licensing of accepted material in official releases, no official
side deal to remove contributor copyleft, and written assumption of those duties
before a voluntary transfer of official stewardship. Those contractual promises
are not appended to downstream AGPL licenses. They cannot guarantee that a
project will never be abandoned, that a competitor will never outperform it,
or that every later actor is bound without agreement. Copyright enforcement
for contributor-owned work may require separate cooperation or authority.

Contributors remain free to license their own work elsewhere. This is reciprocal
stewardship rather than acquiring all rights from contributors. Existing public
licenses, usable source and the ability to fork preserve continuity if official
stewardship fails; governance and operational recovery still require real people.

## Deliberate adoption, without a barrier to learning

The AGPL compiler, Apache runtime and chosen application-license boundary is
settled. Version 1.0 of the contribution agreement is prospective: explicit
contributor and Steward acceptance is necessary, and no earlier contribution
is silently covered. The small recordkeeping process is in its section 9.
Nobody signs it merely to learn, download or build a private paid application.

The maintainer chose to proceed with this reviewed project-specific wording
without making outside legal review a release prerequisite. It is not an
ASF-approved agreement, a claim that counsel reviewed it, or a guaranteed
takeover shield. The [ASF ICLA](https://www.apache.org/licenses/icla.pdf) is a
useful comparison for contribution scope, authority and explicit acceptance;
its broader licensing grant is not imported into this agreement. Specific
future disputes or changes in jurisdiction may warrant professional advice.

The principle may inform other Gamertan projects, but each license, dependency
and contributor history needs its own decision. This document does not change
other repositories or turn "Canadian license" into a new license family.
