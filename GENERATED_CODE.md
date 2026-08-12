<!-- SPDX-FileCopyrightText: 2025-2026 Cole Speelman -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Generated code policy

`.sando.go` files are compiler-managed outputs only when they contain the exact Sandwich Hime generated marker and name the adjacent `.sando` source. “Compiler-managed” describes replacement behavior, not copyright ownership. The compiler may atomically replace that exact output; it never edits other Go files.

Generated files are committed by default so production builds and source audits do not require the compiler. `himesan check` recreates output in memory and reports missing, invalid, or stale files without writing.

Output must be deterministic for identical source, compiler version, runtime ABI, and platform-independent inputs. Unchanged bytes retain their existing timestamp. If any affected source cannot be parsed, context-checked, or formatted, no output in that operation is replaced and last-good files remain available.

The template/application author chooses the generated file's license to the extent they hold the necessary rights. A project-wide license may cover generated files because inline headers would be overwritten. Sandwich Hime adds provenance metadata, not an AGPL license identifier or a compiler copyright claim.

[OUTPUT_EXCEPTION.md](OUTPUT_EXCEPTION.md) is an additional permission for Cole Speelman-owned generator scaffolding copied into output. It is intended to remove licensing ambiguity without claiming that every generated file is or is not a derivative work. It does not cover third-party inputs, code copied manually from the compiler, other contributors' additions unless they grant the same permission, or the Apache-licensed runtime.
