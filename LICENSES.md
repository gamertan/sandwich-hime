<!-- SPDX-FileCopyrightText: 2025-2026 Cole Speelman -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# License map and generated-output policy

Sandwich Hime deliberately separates the development tool from application runtime code.

| Path or material | License |
| --- | --- |
| Project-authored files in the repository root, `cmd/**`, `internal/**`, `docs/**`, and `scripts/**`, except the legal texts listed below | AGPL-3.0-only |
| Nested `sando/**` runtime module, except its verbatim license text | Apache-2.0 |
| `LICENSE`, `sando/LICENSE`, and `DCO.txt` | Their own stated copying terms and notices |
| User-authored `.sando` templates | Chosen by their author, subject to rights in their inputs |
| Generated application `.sando.go` files | Chosen by the template/application author, subject to rights in their inputs and dependencies |

Sandwich Hime claims no copyright in a user's template merely because the compiler processes it. Generated files contain application input, ordinary Go syntax, calls to the Apache-licensed runtime, and a small amount of compiler-authored scaffolding. [OUTPUT_EXCEPTION.md](OUTPUT_EXCEPTION.md) grants an additional permission for Cole Speelman-owned scaffolding copied into generated output. Under the terms granted by this project, generation alone does not require the generated file or surrounding application to use the AGPL. This permission does not grant rights in user inputs, third-party material, or additions owned by other compiler contributors.

The AGPL compiler is a separately installed development process. The Apache runtime must never import an AGPL package. Importing the Apache runtime or using generated output does not, by itself, incorporate the compiler into an application. Redistribution of the runtime remains subject to Apache-2.0 and any other applicable third-party obligations.

The `.sando` source and committed `.sando.go` output under
`internal/compiler/testdata/golden/**` are non-copyable compiler test fixtures
covered by the AGPL `internal/**` map above. The generated fixture intentionally
has no inline SPDX header because that absence is one of the generator's tested
application-output properties.

`COPYRIGHT` identifies Cole Speelman's original project work without claiming contributor-owned work. The nested runtime carries its own `sando/COPYRIGHT`. The project requires no copyright assignment; ownership of contributions remains determined by applicable law and existing agreements.

A compiler contribution that adds text intended to be copied into generated output must record the `Himesan-Output-Permission: v1.0` grant required by [CONTRIBUTING.md](CONTRIBUTING.md). Without that grant, the contribution must be designed so its contributor-owned text is not emitted. DCO sign-off alone does not grant the additional output permission.

Official flags, mascots, and badges are not covered merely because they use a project mark. Each published asset must identify its copyright holder and reuse license.

SPDX identifiers state the applicable license for comment-capable source and documentation. Directory-level maps cover generated files and formats such as JSON that cannot safely carry comments. Full license texts are at `LICENSE` and `sando/LICENSE`. Copyable examples are maintained in separate repositories and must declare their own licenses; official copyable examples are intended to use 0BSD. License texts and the verbatim `DCO.txt` retain their own notices and are not relicensed as project documentation.

The snapshot exporter's `PUBLIC-SNAPSHOT.json` and `PUBLIC-SNAPSHOT.sha256` are generated factual provenance records and intentionally carry no inline SPDX comment. They do not change the license of any listed file.

These are practical project licensing terms, not legal advice or a prediction of how every jurisdiction will classify a particular work. The inactive CLA draft and pre-registration trademark policy say so explicitly. Qualified legal review remains prudent before changing these terms, activating a CLA, registering marks, or making a fact-specific licensing decision; it is not represented as a prerequisite to publishing the current unsupported source preview.
