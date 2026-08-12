<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Threat model

This document separates demonstrated behavior from intended release work. It
defines the boundary Sandwich Hime can reasonably defend; it is not a claim
that the project or an application using it is universally secure.

## Security objective

For supported HTML contexts, data supplied to a compiler-generated component
should remain data. It must not change HTML structure, create executable code,
escape a quoted attribute, or introduce a disallowed URL scheme unless trusted
application source makes an explicit security-sensitive decision.

That objective follows the same high-level model documented by Go's
`html/template`: template authors are trusted while rendered data is not. The
implementations and accepted languages differ. A systematic differential test
campaign against `html/template` remains open work; current tests cover fixed
adversarial cases and do not establish equivalence.

## Trusted capabilities

- `.sando` source, including its static markup and embedded Go statements;
- handwritten application Go and values whose formatting methods execute Go;
- handwritten implementations of `sando.Component`;
- explicit `sando.TrustHTML`, `TrustURL`, `TrustJS`, and `TrustCSS` calls;
- the selected compiler binary, Go toolchain, runtime module, and generated Go;
- local project code built and executed by `himesan dev`; and
- the user account, filesystem, environment, and other processes on the
  development workstation.

Template semantics become trusted source when built into an application;
templates are not an untrusted-content sandbox. Someone allowed to edit one can
execute ordinary Go through the application build and must receive the same
trust as any other code contributor. Arbitrary or malformed template bytes
remain adversarial input to compiler robustness while they are being inspected.

A handwritten `sando.Component` is a trusted output capability. It may write
arbitrary bytes, change HTML parser context, recurse, block, panic, or perform
side effects. Hime-generated components are independently checked for balanced
HTML and may be inserted with `<?~` only at an HTML content boundary. The open
Go interface does not extend that proof to handwritten implementations.

The `Trust*` constructors do not sanitize. They record that trusted application
code accepts responsibility for the supplied bytes. `himesan check` emits
best-effort lexical audit hints for direct constructor/type use visible inside
`.sando` source; it is not Go type analysis, taint analysis, or a complete
inventory of trust created transitively in handwritten Go.

## Untrusted inputs

- ordinary values supplied to generated components;
- filenames and directory entries encountered during discovery;
- missing, stale, or manually modified generated output;
- browser requests reaching the development proxy;
- child-process output, exit behavior, and health failures; and
- malformed template bytes from a repository being inspected, provided the
  template is not subsequently built and executed as trusted Go.

## Demonstrated controls

These are point-in-time implementation and test observations indexed to named
commits in the evidence ledger, not continuous assurance.

- Untrusted dynamic output is accepted only in supported HTML text, quoted
  attribute, URL, and RCDATA contexts. Script and style interpolation requires
  an explicit `TrustedJS` or `TrustedCSS` capability.
- Dynamic tag names, attribute names, unquoted values, event-handler values,
  style attributes, foreign SVG/MathML content, URL lists, `srcdoc`, meta
  refresh, malformed HTML, and ambiguous parser states are rejected.
- Ordinary text and quoted attributes are escaped; `title` and `textarea` use
  an RCDATA writer that escapes every trusted wrapper as ordinary text.
- Ordinary whole URL values are normalized and checked before any bytes are
  written. Schemes outside `http`, `https`, `mailto`, and `tel` fail unless a
  `TrustedURL` deliberately bypasses that check.
- Writer errors and contract-violating short writes propagate to the caller.
- All sources in one operation are parsed/context-checked/formatted in memory
  before the first output change. Each changed output is replaced atomically,
  and existing destinations lacking the generated-file ownership marker, plus
  symlink or non-regular destinations, are rejected. The marker prevents
  accidents; it is not authentication against a hostile local actor.
- Recursive discovery skips VCS, vendor, symlink, nested-module, and detected
  filesystem boundaries. Explicit files are still subject to no-symlink and
  regular-file checks.
- `generate` and `check` do not invoke the Go toolchain, fetch dependencies,
  execute project code, or edit `go.mod`. `himesan dev` is a separate command
  that intentionally does all of generate, build, and execute trusted project
  code.
- The development proxy binds to a literal loopback address, validates Host
  authority, and checks Origin and Sec-Fetch-Site when those headers are
  present. These checks are hardening, not a user-authentication boundary. Its
  injected client is fixed and authorized with a hash rather than
  `unsafe-inline`.

Executable tests and their latest maintainer-observed results are indexed in
[SECURITY_EVIDENCE.md](SECURITY_EVIDENCE.md).

## Important limits

### URLs

URL checking prevents disallowed or ambiguous schemes; it does not decide
whether a destination is authorized or trustworthy. `https:` and relative URLs
can still leave an origin, submit data, change a document base, or load active
content depending on the element and attribute. Applications must validate
destinations and apply tighter policy for sensitive sinks.

### Trusted raw values

`TrustedHTML`, `TrustedJS`, and `TrustedCSS` are intentional escape hatches.
Their authors must preserve the surrounding HTML parser state, including
container-closing and legacy parser-transition sequences. Prefer ordinary data,
keep trust conversion beside its validator, and review each use manually.

### Rendering resources and failures

The runtime does not impose output-size, recursion, CPU, allocation, or time
limits; recover panics; or make an arbitrary `io.Writer` transactional.
Applications should render into a buffer when an all-or-error HTTP body matters
and should apply their own request deadlines, bounded writers, input limits, and
panic policy. A context is passed through components, but generated output does
not automatically stop between writes when it is canceled.

The compiler likewise has no hard source-size, memory, or compile-time budget.
Run it only against repositories whose resource use the caller is willing to
accept.

### Filesystem concurrency

Discovery and generation defend against symlinks and non-regular files observed
at their checks. They are not currently a security boundary against a hostile
local actor racing path components between inspection and use. Run the compiler
inside a trusted workspace and user account. Atomic replacement describes each
file's visibility; a multi-file generation is not a filesystem transaction if
a later replacement fails.

The watcher is a development convenience, not a filesystem-integrity monitor.
It may miss adversarial changes engineered to preserve the metadata it samples.

### Development supervisor

Loopback is host-local, not user-local. The development supervisor has no user
authentication boundary against another process/account on the workstation. It
builds and executes project code with the user's inherited environment and may
fetch dependencies according to the user's Go configuration. It is not a
production proxy, public preview host, TLS terminator, deployment system, or
safe runner for untrusted repositories.

The watcher scans configured trees and eligible HTML responses may be buffered
up to 16 MiB for reload injection. The application and operating system retain
responsibility for request, response, SSE, file-count, and process resource
limits.

Development CSP rewriting is reload convenience, not production CSP
validation. Process cleanup is best-effort; a deliberately detached descendant
may outlive the process tree the supervisor can identify. Human-readable
diagnostics may also contain filenames or child-tool output supplied by a local
repository, so terminals and log consumers remain part of the trusted
development environment.

## Non-goals

Sandwich Hime does not:

- sandbox template authors or embedded Go;
- sanitize arbitrary trusted HTML, JavaScript, CSS, or URLs;
- provide application authentication, authorization, CSRF policy, CSP, routing,
  database security, TLS, caching, or production process isolation;
- type-check all embedded Go during `himesan check` (the normal Go build/test
  remains required);
- detect every dynamic or handwritten component cycle;
- guarantee safety under hostile concurrent mutation of the workspace; or
- replace independent review, browser testing, vulnerability response, or the
  consuming application's threat model.

## Open release work

- broaden semantic and browser-parser differential testing;
- execute the native Windows/macOS security and process-lifecycle matrix;
- complete signed release provenance, checksums, and SBOM evidence;
- test the confidential reporting and signing-key recovery procedures; and
- close or explicitly accept every finding listed in the evidence ledger before
  assigning a supported release line.
