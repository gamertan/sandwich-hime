#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/himesan-consumer.XXXXXXXX")
temporary=$(CDPATH= cd -- "$temporary" && pwd -P)
cleanup() { rm -rf -- "$temporary"; }
trap cleanup EXIT HUP INT TERM

mkdir -p "$temporary/golden"
cp "$repo_root/internal/compiler/testdata/golden/basic.sando" "$temporary/golden/page.sando"
GOTOOLCHAIN=local go build -trimpath -o "$temporary/himesan" "$repo_root/cmd/himesan"
"$temporary/himesan" generate "$temporary/golden/page.sando"
(
	cd "$temporary"
	GOTOOLCHAIN=local go mod init example.test/himesan-consumer
	GOTOOLCHAIN=local go mod edit -go=1.25
	GOTOOLCHAIN=local go mod edit -replace=gamertan.com/sandwich-hime/sando="$repo_root/sando"
	GOTOOLCHAIN=local go mod tidy
	GOTOOLCHAIN=local go test ./...
)

printf 'Temporary Go 1.25 consumer compiled generated output successfully.\n'
