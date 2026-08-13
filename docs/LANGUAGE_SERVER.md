<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Hime-san language server

Hime-san `v1.0.0-beta.2` adds a reusable, read-only Language Server Protocol
surface:

```sh
himesan lsp --stdio
```

The server accepts one local workspace root per process. Editors with multiple
workspace folders start one process for each folder. Standard output contains
only framed JSON-RPC; bounded operational messages go to standard error and do
not include template source, environment values, or secrets.

## Beta 2 capabilities

- full-document synchronization and unsaved in-memory overlays;
- live compiler diagnostics, trust warnings, duplicate components, and
  statically knowable component cycles;
- UTF-16 protocol positions without changing compiler CLI byte coordinates;
- hover help for tags, inferred output contexts, component signatures, and
  trusted-output boundaries;
- document symbols for the declared component and template regions;
- delimiter/tag completion plus same-package and already-imported component
  completion; and
- component go-to-definition.

The index honors the compiler's symlink, nested-module, VCS, vendor, and
filesystem boundaries. Open/save is analyzed immediately; ordinary edits are
debounced for 200 ms and superseded analyses are canceled. Appearance,
deletion, rename, and save notifications rebuild the bounded source index.

## Deliberate exclusions

The language server does not generate files, report generated-file freshness,
invoke Go or `gopls`, execute project code, fetch dependencies, access the
network, or start `himesan dev`. It provides no general Go or HTML completion,
formatting, rename, references, automatic imports, or live browser preview.

Use explicit `himesan check --json` for committed-output freshness and normal
Go tests/builds for type checking. Editors remain responsible for workspace
trust, process startup, and user-visible command policy.

## Resource limits

Protocol frames and individual documents are limited to 16 MiB, the indexed
workspace source set to 64 MiB and 10,000 `.sando` files. These are denial-of-
service guardrails for trusted local workspaces, not a sandbox for hostile
template authors.
