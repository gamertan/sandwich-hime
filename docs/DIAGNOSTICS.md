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
