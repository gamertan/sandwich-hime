<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Architecture

Sandwich Hime is separated into three trust and deployment zones:

```text
trusted .sando source
        |
        v
AGPL himesan compiler: parse -> context-annotated renderer IR -> Go backend
        |
        v
committed .sando.go + Apache sando runtime
        |
        v
ordinary application/router/server chosen by the user
```

The compiler is globally installed for development. `generate` and `check` read source and emit or compare ordinary Go; they do not load plugins, fetch modules, run application code, or mutate module metadata. Generated output is the only bridge from compiler internals into an application.

The renderer IR records source spans and output context independently of Go syntax. The only v1 backend is `go`, selected explicitly in the source header. A future San backend may consume the same IR, but v1 contains no San parser, `.san` handling, or compatibility promise.

The nested `sando` module is a small ABI with no HTTP opinions. It owns component invocation, contextual writer helpers, and opaque trusted values. The application owns buffering, status codes, headers, routing, authentication, caching, CSP, and deployment.

`himesan dev` is a separate local-only supervisor. Its loopback proxy and browser client never enter generated output or a production binary. Candidate application processes become live only after generation, build, startup, and health checks pass; otherwise the last healthy process remains upstream.
