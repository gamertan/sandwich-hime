<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Sandwich Hime language specification

Status: v1 development draft. Implemented behavior and this document must change together.

## Source unit

A `.sando` file defines exactly one component. After optional UTF-8 BOM and whitespace, it starts with a target header and ends at EOF:

```sando
<?sando go
package views

import "example.com/site/model"

func Card(card model.Card)
?>
<article><h2><?= card.Title ?></h2></article>
```

The v1 target is `go`. Other target names are rejected. The explicit target is an architectural seam for a possible future San backend; it is not a promise that such a backend exists.

The header permits one package clause, ordinary Go imports, and one bodyless, receiverless function declaration. The component name is the function name and its parameters form the generated typed API. Multiple components, methods, global declarations, and executable initialization in the header are errors.

## Template tags

| Form | Meaning |
| --- | --- |
| `<? statements ?>` | Trusted Go statements inside the component renderer |
| `<?= expression ?>` | Render a value escaped for the statically determined output context |
| `<?~ expression ?>` | Render a `sando.Component` and propagate its error |
| `<?# comment ?>` | Template-only comment; emits no output |

EOF closes the component. There is no inheritance DSL, implicit request, reflection registry, dynamic component lookup, or raw-output intrinsic.

## Generated API

For `func Card(card model.Card)`, generation emits:

```go
func Card(card model.Card) sando.Component
```

The component captures its typed parameters and renders later with a context and writer. All static writes, escaping operations, nested component renders, and application-provided writers propagate errors.

Generated files are adjacent to their source (`card.sando.go`), formatted with `go/format`, and contain the compiler version, runtime ABI, source digest, and source mappings. Hime-san does not inject the compiler's AGPL license identifier or copyright claim. An application rightsholder remains free to select AGPL intentionally through the application's own license policy.

## Trust and output contexts

Template files and embedded Go are trusted application source. Values rendered by the application are untrusted.

V1 recognizes:

- HTML text;
- quoted attribute values;
- quoted values of recognized URL attributes;
- script and style data only when the Go expression has the conspicuous trusted runtime type;
- component boundaries in ordinary HTML content.

`<?~ ... ?>` is illegal inside an attribute, script, style, tag, or comment. Dynamic tag names, attribute names, event-handler attributes, and unquoted dynamic attributes are rejected. Unsupported, malformed, or ambiguous contexts fail compilation. Static markup must return to the same neutral parser context at EOF and must be structurally balanced under the v1 HTML model.

Ordinary URL values are attribute-escaped and rejected at render time when their normalized scheme is dangerous. Only `sando.TrustedURL`, made by an explicit `sando.TrustURL` call in trusted Go code, may bypass that scheme policy. The analogous trusted HTML, JavaScript, and CSS types are opaque and have conspicuous constructors.

## Compatibility

V1 is a clean break from the 2025 prototype. `.go.hime`, injected `himesan` helper directories, `SandoName(io.Writer)` functions, Go plugins, and nested demonstration modules are not accepted or generated. `.san` is not and will never be a Sandwich Hime extension.

The source language and generated runtime ABI are versioned independently. See [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).
