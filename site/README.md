<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Gamertan site and vanity imports

This directory is a static, no-JavaScript site for the unsupported Sandwich Hime pre-1.0 public source preview. It is source material only: repository automation must not deploy it. The page is deliberately honest that no supported v1 version exists yet.

## Intended routes

The hosting layer must serve these files over HTTPS without an authentication challenge:

| Request path | File | Purpose |
| --- | --- | --- |
| `/sandwich-hime` and `/sandwich-hime/` | `index.html` | Project page and compiler-module `go-import` metadata |
| `/sandwich-hime/sando` and `/sandwich-hime/sando/` | `sando/index.html` | Nested-runtime metadata using the Go 1.25 `subdirectory` field |
| `/sandwich-hime/assets/site.css` | `assets/site.css` | Local-only presentation |

The Go command requests the exact import path; it does not retry at a parent
path after a 404. The hosting layer therefore needs query-scoped metadata
fallbacks:

- any `/sandwich-hime/...` request with `go-get=1`, except the nested runtime
  subtree, returns the compiler `index.html` with HTTP 200;
- `/sandwich-hime/sando` and every path below it with `go-get=1` return
  `sando/index.html` with HTTP 200;
- ordinary browser requests for nonexistent paths continue to return 404.

This includes `/sandwich-hime/cmd/himesan?go-get=1`, which is the path queried
by the documented `go install` command. The `go-import` tags occur early in
each document because the Go command uses a restricted HTML parser.

The nested metadata is intentionally:

```html
<meta name="go-import" content="gamertan.com/sandwich-hime/sando git https://gitea.speelman.ca/gamertan/sandwich-hime.git sando">
```

The fourth field maps the vanity path to the repository’s `sando` subdirectory. It is supported by the project’s minimum Go line, Go 1.25. Runtime versions must use tags such as `sando/v1.0.0`.

## Hosting setup

1. Keep the public DNS and TLS authority for `gamertan.com` under founder control. Use an `A`/`AAAA` record or a narrowly scoped `CNAME` appropriate to the chosen static host; do not delegate the whole zone to a project contributor.
2. Configure the exact browser routes plus the query-scoped `go-get=1`
   fallbacks above. Preserve ordinary 404 behavior and do not use a client-side
   redirect for metadata requests.
3. Return `Content-Type: text/html; charset=utf-8` for HTML and `text/css; charset=utf-8` for CSS.
4. Add server headers at least equivalent to `Content-Security-Policy: default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'`, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, and a conservative `Permissions-Policy`.
5. Keep deployment credentials outside this repository. A future deploy workflow needs a separately reviewed, least-privilege credential and protected environment approval.
6. Verify from an uncached public network before announcing installs:

   ```sh
   curl -fsS 'https://gamertan.com/sandwich-hime?go-get=1'
   curl -fsS 'https://gamertan.com/sandwich-hime/cmd/himesan?go-get=1'
   curl -fsS 'https://gamertan.com/sandwich-hime/sando?go-get=1'
   ./scripts/verify-public-install.sh --version v1.0.0
   ```

## Launch blockers

Do not deploy or remove the pre-release warning until all of these are evidenced:

- canonical Gitea is public, the security contact works, and protected release-key controls are active;
- root `v1.0.0` and nested `sando/v1.0.0` are signed and accompanied by checksums, SBOMs, release binaries, and reproducibility notes;
- Go 1.25 and Go 1.26 pass deterministic generation and tests on Linux, macOS, and Windows, including the canonical manual cross-platform workflow;
- vulnerability, race, fuzz, adversarial, license, accessibility, and CSP checks pass;
- EQL Wiki completes its differential pilot and 14-day production soak without Hime render, security, or accessibility regressions;
- the DCO contribution process, license map, governance, output permission, and trademark policy have final human review;
- `scripts/release-check.sh --version v1.0.0 --public` passes against a human-reviewed evidence bundle;
- the exact public `go install` and `go get` commands pass
  `scripts/verify-public-install.sh` from clean direct-fetch and public-proxy
  caches without repository credentials.

At launch, replace each page’s `himesan-release-status` value with the exact compiler release (for example, `v1.0.0`), replace the human-facing pre-release copy with verified install information, and run `scripts/check-site.sh --public v1.0.0`. The public release preflight enforces that transition so a green evidence bundle cannot accidentally publish a page that still says the runtime does not exist.

Gitea is the sole public forge. Cross-platform evidence must come from reviewed local or Gitea-runner execution; no secondary mirror or hosted workflow is part of the release plan.
