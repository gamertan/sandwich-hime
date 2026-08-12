<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Sandwich Hime

Sandwich Hime (Hime-san) is an HTML-first component compiler for Go. It keeps the direct, mixed-markup feeling of classic PHP while producing typed, deterministic Go that an ordinary `go build` can audit and deploy.

```sando
<?sando go
package views

func Profile(page ProfileView)
?>
<section class="profile">
  <h1><?= page.Name ?></h1>
  <? if page.IsAdmin { ?>
    <?~ AdminBadge() ?>
  <? } ?>
</section>
```

The generated API is ordinary Go:

```go
func Profile(page ProfileView) sando.Component
```

Hime-san is a development tool, not an application framework. A consuming project keeps its `.sando` sources, commits the adjacent `.sando.go` output, and imports only the small Apache-2.0 `sando` runtime. Hime-san never owns the router, middleware, layout policy, request object, or production server.

## Status

This repository is an unsupported public pre-1.0 source preview, not a supported v1 release. EQL Wiki remains the proof-of-production proving ground, and v1 is gated on security testing, cross-platform determinism, and a 14-day production soak with no renderer, security, or accessibility regression.

For repository development:

```sh
go install ./cmd/himesan
himesan generate ./examples/eql-shaped
himesan check ./examples/eql-shaped
go test ./...
(cd sando && go test ./...)
(cd examples/eql-shaped && himesan dev --config himesan.json)
```

The example's development server remains the application's own `net/http`
program. Hime-san builds it in the user cache, health-checks a random loopback
upstream, and serves the last healthy candidate through
`http://127.0.0.1:7331` with local-only reload diagnostics.

The eventual versioned installs are:

```sh
go install gamertan.com/sandwich-hime/cmd/himesan@v1.0.0
go get gamertan.com/sandwich-hime/sando@v1.0.0
```

Those vanity paths must not be advertised as working until the corresponding signed releases and `gamertan.com` metadata exist.

## The contract

- One typed component per `.sando` file.
- Go statements are trusted source code; rendered values are untrusted.
- `<?= ... ?>` escapes for the statically known HTML context.
- `<?~ ... ?>` composes another component and propagates errors.
- Ambiguous or unsupported HTML contexts fail compilation.
- Generation is deterministic, formatted, atomic, and never edits handwritten Go or `go.mod`.
- `check` is read-only and detects invalid or stale generated output.
- Production builds need no compiler binary.

The language is specified in [SPEC.md](SPEC.md). The security boundary is in [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md), the development loop is in [docs/DEVELOPMENT_SERVER.md](docs/DEVELOPMENT_SERVER.md), and the multi-license boundary is in [LICENSES.md](LICENSES.md).

User-authored templates and generated application files remain under terms chosen by their authors to the extent they hold the necessary rights. The compiler is AGPL-3.0-only, the runtime is Apache-2.0, and [OUTPUT_EXCEPTION.md](OUTPUT_EXCEPTION.md) grants an additional permission for Cole Speelman-owned generator scaffolding copied into output.

## Names matter

- Project: **Sandwich Hime / Hime-san**
- CLI: `himesan`
- Template: `page.sando`
- Generated Go: `page.sando.go`
- Runtime: `sando`
- `.san`: reserved exclusively for the separate San language

The project is never marketed as bare “Hime”; that name is already used by an unrelated Go web framework.

## Why

This is a love letter to hand-built web development: the immediacy of a 2004 personal site, with typed interfaces, reproducible builds, modern contextual safety, and boring production operations. Performance claims will follow published measurements, never precede them.
