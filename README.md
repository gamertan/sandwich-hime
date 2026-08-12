<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Sandwich Hime

> **Canonical project:** development, contribution and security instructions,
> releases, and stewardship live on the
> [founder-controlled Gamertan Gitea](https://gitea.speelman.ca/gamertan/sandwich-hime).
> A GitHub copy, when present, is a read-only discovery snapshot rather than a
> contribution or release authority.

Sandwich Hime is an HTML-first, ahead-of-time template engine for Go. Hime-san
keeps the direct, mixed-markup feeling of classic PHP while compiling trusted
`.sando` templates into typed, deterministic Go components that an ordinary
`go build` can audit and deploy.

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

Templates are what authors write. Components are the typed output and
composition unit. Hime-san is an intentionally opinionated development tool,
not an application framework: a consuming project keeps its `.sando` sources,
commits the adjacent `.sando.go` output, and imports only the small Apache-2.0
`sando` runtime. Hime-san never owns the router, middleware, layout policy,
request object, or production server.

## Status

This repository is an unsupported public pre-1.0 source preview, not a
supported v1 release. V1 is gated only by repository-owned compiler, runtime,
security, compatibility, and release checks. Application-specific deployments,
examples, and case studies live in their own repositories and are not imported
as release evidence here.

For repository development:

```sh
go install ./cmd/himesan
./scripts/verify.sh
./scripts/check-licenses.sh
```

The portable path is `generate`, `check`, and the project's normal Go tools.
For people who want the paved road, `himesan dev` deliberately does more: it
builds the application's own `net/http` program in the user cache,
health-checks a random loopback candidate, preserves the last healthy process,
and serves it through `http://127.0.0.1:7331` with local-only reload
diagnostics. That is a Cole-shaped convenience, not a production server or a
requirement. Take the paved path—or don't.

The eventual versioned installs are:

```sh
go install gamertan.com/sandwich-hime/cmd/himesan@v1.0.0
go get gamertan.com/sandwich-hime/sando@v1.0.0
```

Those vanity paths must not be advertised as working until the corresponding signed releases and `gamertan.com` metadata exist.

## The contract

- One typed template, compiled into one component constructor, per `.sando` file.
- Go statements are trusted source code; rendered values are untrusted.
- `<?= ... ?>` escapes for the statically known HTML context.
- `<?~ ... ?>` composes another component and propagates errors.
- Ambiguous or unsupported HTML contexts fail compilation.
- Generation is deterministic and formatted; each owned output is replaced atomically, and handwritten Go and `go.mod` are never edited.
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

This is a love letter to hand-built web development: the immediacy of Cole's
2004 Geocities page for a PSO Gameclub, with typed interfaces, reproducible
builds, modern contextual safety, and boring production operations. The first
prototype was a wonderfully cursed princess that could `bless` or `rebuke`
templates, announced when Hime-san was resting, and concluded that creating a
compiler was not hubris but destiny. The jokes stayed because developer joy is
part of the point. The security boundary grew up because care is part of the
point too.

Cole builds with ADHD, neurodivergence, a disabled body, finite energy, and a
family he wants to leave understandable work for. Clear files, calm defaults,
accessible documentation, last-good development builds, and honest limitations
are therefore operating requirements—not decorative empathy. The project aims
to leave room for individuals, tiny teams, learners, disabled people, and
anyone too small to win a complexity contest.

Open source permits corporate use. Independence comes instead from the
AGPL-3.0-only compiler, the Apache-2.0 runtime, application-owned generated
output, contributor-held copyright, founder-led governance, release-key
control, and careful trademark stewardship. Companies may use and contribute;
participation does not confer ownership of the identity or project.

The fuller origin, Japanese craft inspirations, family dedication, human-art
commitment, and stewardship boundary live on the
[project site](https://sandwichhime.com/docs/project/). The official
[step-by-step lesson](https://sandwichhime.com/docs/tutorial/) lives with that
documentation, and its
[0BSD runnable companion](https://gitea.speelman.ca/gamertan/sandwich-hime-tutorial)
has a separate repository and history. Tutorials and copyable applications are
maintained separately from this compiler repository.
Performance claims will follow repository-owned measurements, never precede
them.
