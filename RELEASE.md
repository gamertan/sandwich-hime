<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Release process

Sandwich Hime uses separate root and runtime version lines. Compiler tags are `vX.Y.Z`; runtime tags are `sando/vX.Y.Z`. Generated headers record both compiler version and runtime ABI.

The public pre-1.0 source snapshot is not a supported release and does not imply that the v1 gates below have passed.

No v1.0.0 release occurs until every gate in this repository is evidenced,
including cross-platform deterministic generation, temporary-module
compilation, fuzz/adversarial suites, race/vet/vulnerability/license checks on
the latest two supported Go lines, development-supervisor failure tests, and
reproducible repository-owned benchmark and security results. A deployment,
example, or case study in another repository is neither imported nor required
as release evidence.

Release candidates require a clean canonical checkout, reviewed changelog, compatible vanity-import metadata, reproducible binaries, signed annotated tags, checksums, SBOMs, vulnerability results, and verification on Linux, macOS, and Windows. The runtime is tagged and published independently before the compiler that references its ABI.

Gitea is the only canonical public forge. Public source is exported into a
separate, sanitized Gitea repository with fresh history; private development
history and the private-to-public commit mapping are not published. A
sanitized GitHub discovery snapshot may copy reviewed public source, but it is
not an issue, contribution, release, or module origin and must never receive
private development refs or an indiscriminate Git mirror. Release binaries and
provenance are built from the reviewed canonical Gitea commit. Compiler
documentation, binaries, checksums, SBOMs, and the independently versioned
runtime tag form the coordinated v1 release. Example applications and product
sites keep their own history, deployment, and evidence.

The hosting configuration must answer exact package discovery requests, not
only module-root pages. In particular,
`/sandwich-hime/cmd/himesan?go-get=1` returns compiler metadata and the
`/sandwich-hime/sando` subtree returns runtime metadata. After signed tags and
public metadata exist, run `scripts/verify-public-install.sh --version
vX.Y.Z`; it exercises the documented `go install` and `go get` commands from
fresh direct-fetch and public-proxy caches. This post-publication check is
separate from the pre-tag, read-only `scripts/release-check.sh`.

Release notes report hardware, commit, datasets, commands, `ns/op`, allocations, response latency, and methodology for performance claims. “Fastest” or equivalent language is prohibited without durable, reproducible evidence.

Production applications compile and deploy their committed `.sando.go` files
with the Apache-2.0 `sando` runtime. They do not need the AGPL compiler or the
local development supervisor. Release checks verify that boundary without
executing or inspecting an unrelated application repository.
