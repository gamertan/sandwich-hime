<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Security policy

Sandwich Hime is an unsupported public pre-1.0 source preview. No version is yet supported for production use, and the project makes no vulnerability-response SLA or bug-bounty promise.

Do not put undisclosed vulnerability details, credentials, personal data, or a working exploit in a public issue. Until a dedicated confidential address is published, use the repository owner's published Gitea contact method to ask for a private channel without disclosing the issue. If no private contact method is available, retain the details rather than publishing them. A tested confidential contact and documented response targets remain blockers for a supported release.

The compiler treats templates and embedded Go as trusted source and rendered values as untrusted data. It does not sandbox template authors. The security boundary and known non-goals are specified in [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md).

For a future supported release, the intended process includes a private reproducer, regression test, coordinated disclosure when appropriate, checksums, and an advisory. Release artifacts and tags must be signed. Dependencies are minimized and scanned; generation/checking never fetch dependencies or execute project code.

This policy describes the project's current process and limitations; it is not legal advice and does not promise that every report can be accepted, embargoed, or fixed on a particular schedule.
