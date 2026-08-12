<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Contributing

The canonical public project and only contribution venue is the
[founder-controlled Gitea repository](https://gitea.speelman.ca/gamertan/sandwich-hime).
Repository maintainers may temporarily disable issue or patch intake during the
pre-1.0 preview; do not route around a closed intake channel by sending
unsolicited private patches.

Public pre-1.0 contributions use Developer Certificate of Origin 1.1 sign-off. The proposed `CLA.md` is an inactive draft, is not a condition of contribution, and creates no contributor or project obligations. If a contribution agreement is ever activated after legal review, the project will announce its prospective terms rather than silently applying the draft.

For local work:

```sh
go test ./...
go test -race ./...
go vet ./...
(cd sando && go test -race ./... && go vet ./...)
./scripts/check-licenses.sh
```

Changes require focused tests, stable diagnostics, formatted generated goldens when applicable, documentation for public behavior, and a signed-off commit (`git commit -s`). The sign-off certifies the [DCO](DCO.txt); it is not a copyright assignment or acceptance of the inactive CLA. Do not commit production data, private application fixtures, secrets, build candidates, or developer cache files.

The project requires no copyright assignment. Ownership remains determined by applicable law and any employer or other agreement. Contributors submit each file under the license identified for that repository area, and the DCO records their certification that they have the right to do so. Material AI assistance must follow [AI_CONTRIBUTIONS.md](AI_CONTRIBUTIONS.md). Review considers provenance, safety, maintenance cost, compatibility, and fit—not just whether code passes tests.

If a compiler contribution adds or changes contributor-owned scaffolding that Hime-san is intended to copy into generated output, its signed-off commit must also contain this trailer:

```text
Himesan-Output-Permission: v1.0
```

That trailer records the contributor's grant of the additional permission in [OUTPUT_EXCEPTION.md](OUTPUT_EXCEPTION.md) for the affected contribution. DCO sign-off does not supply that separate grant. Maintainers must preserve the signed grant in the private contribution record even when a sanitized public snapshot uses fresh history. A patch without it must not cause contributor-owned text to be emitted; maintainers must reject or redesign such a patch rather than assume permission.

Potential vulnerabilities follow [SECURITY.md](SECURITY.md), not the ordinary contribution channel. Do not place confidential vulnerability details in an issue or patch description.
