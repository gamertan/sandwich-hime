<!-- SPDX-License-Identifier: 0BSD -->

# Synthetic EQL-shaped fixture

This copyable 0BSD example exercises a shared document layout, typed page data,
component composition, loops, text escaping, attribute escaping, and URL
policy without containing EQL Wiki code, data, routes, or its database.

From the repository root:

```sh
go run ./cmd/himesan generate ./examples/eql-shaped
go run ./cmd/himesan check ./examples/eql-shaped
(cd examples/eql-shaped && go test ./... && go run ./cmd/example)
```
