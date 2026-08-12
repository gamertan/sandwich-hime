<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Threat model

## Trusted

- `.sando` files and embedded Go statements;
- handwritten application Go;
- explicit calls to `sando.TrustHTML`, `TrustURL`, `TrustJS`, and `TrustCSS`;
- the selected compiler binary and runtime module version.

## Untrusted

- values supplied to components unless deliberately wrapped in a trusted type;
- filenames and directory entries encountered during discovery;
- stale or manually modified generated output;
- browser requests reaching the development proxy;
- child process output and health failures.

## Guarantees sought by v1

- Context-sensitive escaping for supported HTML text, quoted attributes, URL attributes, and explicitly trusted script/style values.
- Compilation failure for unsupported or ambiguous output contexts.
- Dangerous normalized URL schemes fail rendering unless explicitly trusted.
- Component calls cannot change the surrounding HTML parser context.
- Dynamic `title` and `textarea` content uses a distinct RCDATA writer that escapes even `TrustedHTML`; trusted HTML cannot close those elements.
- Writer failures propagate and partial output is visible to the caller as an error; applications can buffer when atomic responses matter.
- Generation plans all outputs before atomic replacement, targets only owned files, preserves last-good output on failure, and follows neither symlinks nor nested-module traversal.
- `generate` and `check` do not execute project code, invoke Go tooling, fetch dependencies, or alter `go.mod`.

## Non-goals

Templates are not a sandbox. A malicious template author can write malicious Go in a statement tag. Sandwich Hime does not validate business authorization, prevent unsafe application logic, make an arbitrary `io.Writer` transactional, or secure an application router/server. Trusted constructors are intentionally sharp tools and must remain conspicuous in review and `himesan check` reporting. A `TrustedHTML` fragment must be balanced and context-neutral; `TrustedJS` and `TrustedCSS` authors are responsible for excluding container-closing HTML sequences.

The v1 HTML state machine is deliberately smaller than a browser parser. Any construct it cannot prove safe is rejected rather than guessed. Differential testing against Go `html/template` is a baseline, not a claim of byte-identical output or universal parser equivalence.

## Principal attack classes

Tests cover delimiter confusion, malformed HTML, quote/entity injection, event attributes, dangerous and obfuscated URLs, script/style termination, Unicode and NUL handling, component context breaks, import/source-map injection, CRLF and path behavior, symlinks, nested modules, stale outputs, interrupted/read-only writes, writer failures, component cycles, development-proxy exposure, CSP weakening, compression/content-length mistakes, and orphaned child processes.
