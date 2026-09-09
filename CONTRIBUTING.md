<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Contributing

The canonical public project and only contribution venue is the
[founder-controlled Gitea repository](https://gitea.speelman.ca/gamertan/sandwich-hime).
Repository maintainers may temporarily disable issue or patch intake during the
pre-1.0 period; do not route around a closed intake channel by sending
unsolicited private patches.

New public contributions use DCO 1.1 sign-off and the prospective
[Individual contribution and stewardship agreement, version 1.0](CLA.md).
The agreement starts with this version's first canonical publication and binds
only people who expressly accept it; no earlier contribution is retroactively
covered. [Section 9](CLA.md#9-acceptance-and-minimal-records) gives the short
contributor statement and matching maintainer acceptance. A public Gitea
pull-request comment can record it, or contributors can request a private
channel first. Never post private identity or employer documents.

The agreement preserves ownership, the existing public licenses and reciprocal
official-steward duties. See [Licensing intent](docs/LICENSING_INTENT.md).
Learning, downloading, using Hime, and building private or commercial applications
require no contribution agreement. Ask ordinary usage questions without signing.

For local work:

```sh
go test ./...
go test -race ./...
go vet ./...
(cd sando && go test -race ./... && go vet ./...)
./scripts/check-licenses.sh
```

Changes require focused tests, stable diagnostics, formatted generated goldens when applicable, documentation for public behavior, and a signed-off commit (`git commit -s`). The sign-off certifies the [DCO](DCO.txt); it is not a copyright assignment or a substitute for the separate agreement acceptance. Do not commit production data, private application fixtures, secrets, build candidates, or developer cache files.

The project requires no copyright assignment. Ownership remains determined by applicable law and any employer or other agreement. Contributors submit each file under the license identified for that repository area, and the DCO records their certification that they have the right to do so. Material AI assistance must follow [AI_CONTRIBUTIONS.md](AI_CONTRIBUTIONS.md). Review considers provenance, safety, maintenance cost, compatibility, and fit—not just whether code passes tests.

If a compiler contribution adds or changes contributor-owned scaffolding that Hime-san is intended to copy into generated output, its signed-off commit must also contain this trailer:

```text
Himesan-Output-Permission: v1.0
```

That trailer records the contributor's grant of the additional permission in [OUTPUT_EXCEPTION.md](OUTPUT_EXCEPTION.md) for the affected contribution. DCO sign-off does not supply that separate grant. Maintainers must preserve the signed grant in the private contribution record even when development commits are squashed into a public publication commit. Public attribution must remain accurate; squashing does not transfer ownership. A patch without it must not cause contributor-owned text to be emitted; maintainers must reject or redesign such a patch rather than assume permission.

Potential vulnerabilities follow [SECURITY.md](SECURITY.md), not the ordinary contribution channel. Do not place confidential vulnerability details in an issue or patch description.
