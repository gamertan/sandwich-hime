<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Compatibility policy

Before v1.0.0, source syntax and generated ABI may change without compatibility shims, but each public change must be documented and deterministic. Private prototype history is intentionally outside the sanitized public repository and carries no public compatibility promise.

At v1, semantic versions apply independently to the compiler and `sando` runtime. Generated files record the exact compiler version and required runtime ABI. Patch releases do not intentionally change accepted source semantics or generated public signatures. Minor releases may add fail-closed syntax or API capabilities while continuing to render previously valid components. Major releases may remove or reinterpret behavior.

The compiler supports the latest two Go release lines validated in CI. A support change is announced before release. Generated files are source artifacts, not a stable interchange format across compiler versions; `himesan check` defines whether they are current.

The project makes no compatibility promise for internal packages, development SSE payloads before v1, or hand-edited generated files.
