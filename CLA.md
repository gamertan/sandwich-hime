<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Individual contribution and stewardship agreement

**Version 1.0 — prospective contribution terms.** This version takes effect for
new contribution intake when first published in the canonical public repository.
An individual agreement is formed only through the explicit contributor and
Steward acceptance in section 9. Existing contributions remain under their
recorded licenses, DCO sign-offs and output permissions; publication does not
create retroactive acceptance. Nobody must sign this agreement to learn, install,
use or build applications with Sandwich Hime.

Purpose: keep the shared project available for lawful use, study, improvement
and commercial work, without collecting contributors' ownership or giving
official stewards a private route around the project's copyleft. The practical
scope and practical limits are in
[Licensing intent](docs/LICENSING_INTENT.md).

## 1. Parties and contribution scope

The parties are the individual identified in the acceptance record ("you") and
Cole Speelman, acting in his individual capacity as maintainer of Sandwich
Hime ("the Steward"). Sandwich Hime is the project name, not a separate legal
entity. An organization does not become a party merely because you work for it.

A "Contribution" is material you intentionally submit for inclusion in this
repository after accepting this agreement, identified by a patch, commit or
pull request in the contribution record. Ordinary discussion, support requests,
confidential security reports, and material marked "not a contribution" are
not submissions under this agreement. Earlier work requires a separate,
explicit identification and acceptance; nothing applies retroactively by default.

The "Recorded License" is the file license and any explicit additional
permission identified for that Contribution at submission, with the applicable
repository revision retained. The current map is [LICENSES.md](LICENSES.md):
AGPL-3.0-only for compiler/project material and Apache-2.0 for the nested
runtime. Existing third-party material keeps its own terms. A later edit to
the map does not change a Contribution's Recorded License.

## 2. Your ownership and the public grant

You retain ownership of rights you hold. You license your Contribution to the
Steward and recipients under its Recorded License, including that license's
copyright and patent provisions, conditions, duration, termination and cure
rules. No copyright assignment, exclusive license or agency is created.

There is no additional general grant to sublicense or relicense your work
under arbitrary terms. AGPL recipients obtain their rights under the AGPL;
this agreement is not an alternate proprietary license. Permissions already
granted by Apache-2.0 or another Recorded License are not narrowed here.

You remain free to use or license rights you actually own elsewhere. You may
stop making future contributions at any time. That does not withdraw rights
already granted to recipients who comply with the applicable license. No
contribution fee, royalty, revenue share or exclusivity is required.

## 3. Generated-output permission is specific

Contributor-owned scaffolding intended to be emitted into generated output
requires your explicit `Himesan-Output-Permission: v1.0` trailer on the signed-off
submission, as described in [OUTPUT_EXCEPTION.md](OUTPUT_EXCEPTION.md). That
record identifies the affected material and retains the exact permission text
and version. This is an additional copyright permission, not an additional
patent grant.

General CLA acceptance and DCO sign-off do not supply this separate grant.
Without it, the Steward must reject or redesign the contribution so that your
unpermitted material is not emitted. The grant does not license the compiler
as a whole permissively or give rights in material you do not own.

## 4. Authority, provenance and patents

You represent that you own, or have sufficient authorization to submit and
license, the Contribution under its Recorded License. Where an employer or
another rights holder has an interest, obtain the necessary permission before
submission. If that authority is uncertain, disclose the issue and withhold
the affected material; your signature cannot grant someone else's rights.

Identify incorporated third-party material, its source and known restrictions.
Disclose material AI assistance under the revision of
[AI_CONTRIBUTIONS.md](AI_CONTRIBUTIONS.md) identified in your acceptance record;
a responsible human must review the submission and make the representations.
Later policy edits do not amend these contractual representations without
express agreement under section 8.
Do not send confidential prompts, personal data or secrets as provenance.
Notify the Steward if you later learn a material representation was inaccurate.

The patent grant is the one in the Recorded License, including AGPL section 11
or Apache-2.0 section 3 as applicable. This agreement adds no patent assignment,
new retaliation trigger, or guarantee that no third party holds a patent. It
does not purport to license claims outside your authority.

## 5. Steward commitments: preserve the common project

In accepting Contributions under this agreement, the Steward undertakes to:

- Publish accepted material included in an official public release under its
  Recorded License, with source available as that license requires. The
  agreement does not promise immediate publication of every submitted patch.
- Preserve contributor copyright notices, required attribution, license and
  permission records. Public squashes must not misrepresent authorship; keep
  the original signed contribution record without publishing private intake data.
- Not use this agreement to remove copyleft from contributor-owned AGPL code,
  issue an exclusive or proprietary license for that code, or sell a paid
  exception to its source-sharing duties. The expressly recorded output
  permission remains the narrow exception already described above.
- Not seek a private side agreement to evade that official stewardship
  commitment. This is a promise about the Steward's official use of accepted
  material, not a restriction on a contributor's independent use of their work.
- Give no sponsor, purchaser or voting majority authority through this
  agreement to acquire contributor ownership or override existing grants.
- Before voluntarily transferring official stewardship, obtain the successor's
  written assumption of these stewardship obligations. Delegating maintenance
  does not itself release the Steward from contractual obligations or transfer
  contributor copyrights.

These are commitments by the contracting Steward, not new downstream license
conditions. They do not bind unrelated forks or non-signing third parties,
restrict permissive-license freedoms, or confer authority over rights that
another person owns. Retained public licenses remain available independently
of changes in maintainers, sponsorship or repository ownership.

## 6. Contributor and maintainer safeguards

Contributions are voluntary. Neither party promises that a patch will be
accepted, merged unchanged, maintained indefinitely or kept in every future
version. Removing a feature does not revoke licenses already granted.

No employment, partnership, governance seat, support obligation, indemnity or
obligation to fund litigation is created. Warranty disclaimers and liability
limits remain those in the Recorded License, subject to mandatory law. You
are not asked to certify worldwide freedom from infringement or waive all
claims against the Steward.

No blanket moral-rights waiver is requested. To the extent law permits, you
consent to ordinary licensed editing, combination, compilation and distribution
of the Contribution, while retaining protection against false attribution or
endorsement. Neither party may use the other's name to imply an endorsement
that was not given. This is not a power to forbid lawful criticism or forks.

The Steward may coordinate compliance reports but receives no assignment,
power of attorney or assumed standing to litigate another owner's copyright.
Separate authority may be necessary for particular enforcement action.

## 7. Commercial freedom and license limits

Users may charge for lawful services, support, distribution and applications
subject to the relevant software licenses. A complying commercial competitor
or independently branded fork does not breach this agreement merely by being
successful, and upstream contribution is not mandatory.

No customer must sign this CLA to use the software. A blanket SaaS prohibition,
competition veto, moral-use test or mandatory payment to the project is not
added to the AGPL. Network-source duties come from the applicable license;
they do not automatically reach applications built with the compiler.

## 8. Changes, disputes and succession

Changing this document or the governance policy does not amend an accepted
agreement. Any amendment requires the affected parties' express agreement to
identified new terms; continued use of the software is not acceptance.
Contract disputes do not retract lawful downstream grants or add license
termination grounds beyond the relevant software license.

The proposed governing law is Ontario law and applicable federal Canadian law,
without excluding non-waivable rights or remedies available under applicable
law. The parties should first try good-faith written resolution where practical;
no mandatory arbitration, class-action waiver or bar to urgent relief is added.
This agreement makes no guarantee about a particular court's interpretation.

## 9. Acceptance and minimal records

Identify version 1.0 using an immutable canonical commit containing this text
and the incorporated AI policy, not a moving branch link. The contributor sends
the statement below with their name, date and identified contribution through
their canonical Gitea account. A public pull-request comment is sufficient if
the contributor chooses to make it public. For private acceptance, request a
private contact channel from the maintainer before sending personal details.
Do not post employer documents, private contact details or identity documents.

> I agree to the Sandwich Hime Individual contribution and stewardship agreement
> version 1.0 and its incorporated AI policy at canonical commit [full commit],
> for contribution [pull request or commit]. I intend this statement, sent from
> my account and signed with my name and date, as my electronic acceptance.

The Steward must reply with explicit matching acceptance of the identified
agreement and contribution before merging it. Retain both statements, their
dates/account identifiers, the exact agreement and policy texts with SHA-256
digests, and the contribution's Recorded License and source identifiers.
Neither a DCO sign-off, a merge, nor silence substitutes for this exchange.
Later contributions need their own acceptance unless both parties expressly
include future intentional submissions under that exact agreement version.
Every commit still requires DCO sign-off; affected output scaffolding still
requires its separate explicit permission.

Keep private acceptance and any necessary authority records in restricted
maintainer storage, not in the public source snapshot. Do not request a home
address, government identification or unrelated employer/customer records.
The public attribution and sign-offs the contributor intentionally submits are
distinct from private intake records. The Steward uses retained records only
to administer contributions, document rights and handle related disputes, and
does not sell them or repurpose contacts for marketing.

A contributor may request access, correction or deletion through the same
private channel. Retain only records reasonably necessary to substantiate grants
still relied upon or meet applicable legal obligations; delete unnecessary
copies and explain any retention needed when responding to a request.
Withdrawing from future participation does not revoke valid existing grants.
If an individual's authority does not cover employer-owned material, do not
accept it until the rights holder's authorization is recorded. A separate
entity agreement may be needed; this document does not bind an employer by
assumption.
