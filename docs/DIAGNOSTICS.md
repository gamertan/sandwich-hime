<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Diagnostics

Human diagnostics use stable `path:line:column` locations and `HIM####` codes. `--json` emits one object containing the operation result and structured diagnostics. Error diagnostics make generation/check exit nonzero; audit warnings do not.

Code families are intentionally coarse compatibility surfaces:

| Range | Area |
| --- | --- |
| `HIM10xx` | Source encoding |
| `HIM11xx` | Component header and Go declaration |
| `HIM12xx` | Template tags and expressions |
| `HIM13xx` | HTML parser context and structure |
| `HIM14xx` | Generated Go/backend validation |
| `HIM15xx` | Component graph and package collisions |
| `HIM19xx` | Trusted-value audit warnings |
| `HIM20xx` | Discovery and cancellation |
| `HIM21xx` | Owned atomic generation |
| `HIM22xx` | Read-only freshness checking |
| `HIM29xx` | Boundary warnings |

Scripts should consume the JSON `code`, `severity`, and location fields, not parse English messages. Message wording may improve within a compatible release.

The exact v1 code inventory is machine-checked against
[`contracts/diagnostic-codes-v1.txt`](../contracts/diagnostic-codes-v1.txt).
Adding, removing, or renumbering a code requires an explicit compatibility
review and snapshot update.

## CLI and structured-output contract

Command exit codes use three classes: `0` for success (including help and
warning-only results), `1` for a completed operation that failed validation or
runtime service, and `2` for invalid command usage or failure to encode the
requested CLI result. `gen` normalizes to `generate`; `bless` remains a named
read-only alias of `check` in structured output.

The v1 JSON shapes are published as closed schemas:

- [`himesan-operation-output-v1.schema.json`](../contracts/himesan-operation-output-v1.schema.json)
  for `generate`, `check`, and `bless`;
- [`himesan-version-output-v1.schema.json`](../contracts/himesan-version-output-v1.schema.json)
  for `version --json`.

Unknown output fields are not introduced in a compatible v1 patch without an
explicit schema/version decision. Consumers should still ignore English
message wording.
