<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Local development supervisor

`himesan dev [package] [-- app-args...]` is an explicitly local convenience. It generates templates, builds the selected Go package into the user cache, starts a candidate on a random `127.0.0.1` address, health-checks it, and only then switches a stable loopback reverse proxy. Generation, build, startup, and health failures leave the previous healthy child serving.

The application must read its listen address from the configured environment variable and expose the configured health path. It remains an ordinary application server; no development proxy code appears in generated files or production binaries.

## `himesan.json` schema version 1

```json
{
  "version": 1,
  "sourceRoots": ["views"],
  "goPackage": "./cmd/site",
  "appArgs": ["--development"],
  "listenAddressEnv": "HIMESAN_LISTEN_ADDR",
  "healthPath": "/healthz",
  "proxyAddress": "127.0.0.1:7331",
  "additionalWatchRoots": ["assets"]
}
```

Unknown fields, non-loopback proxy addresses, invalid environment names, malformed health paths, NULs, and unsupported schema versions are rejected. Arguments are passed directly without a shell. The configuration contains no commands, secrets, credentials, or production bind address.

Simple projects can override the package and repeat `--source`/`--watch`, plus `--proxy`, `--listen-env`, and `--health`. When a config path is supplied, relative paths resolve from its directory.

## Browser behavior

The stable proxy reserves `/__himesan/events` for SSE. It injects a fixed reload/diagnostic client only into successful full HTML documents with positive document evidence. Fragments, APIs, encoded bodies, range responses, downloads, HEAD, and non-success responses are never injected. Development responses are non-cacheable.

When an existing CSP is present, the proxy adds the fixed script's SHA-256 source and same-origin SSE connection permission; it does not add `unsafe-inline` or `unsafe-eval`. The proxy and every candidate upstream are literal loopback addresses. Replaced process groups are terminated and waited for on Unix and Windows.

If the active application exits, the proxy immediately forgets that exact upstream and closes its idle connections. Requests receive the waiting page until another candidate passes its health check; a different process that later acquires the old loopback port is not selected implicitly.

Loopback is host-local, not user-local. Host, Origin, and Fetch Metadata checks defend against browser cross-site and DNS-rebinding requests, but they are not authentication against another process or account on the same workstation. Run `himesan dev` only on a trusted, single-user development machine and do not place secrets in its diagnostics. It invokes the configured Go toolchain, may fetch dependencies according to the user's Go environment, executes the project binary with the user's inherited environment, and forwards the application's requests and responses. Eligible HTML responses may be buffered up to 16 MiB for reload injection; application request, response, and SSE concurrency limits remain the application's and operating system's responsibility.

This is not a production proxy, TLS terminator, public preview server, process orchestrator, or deployment system. V1 refuses non-loopback binding.
