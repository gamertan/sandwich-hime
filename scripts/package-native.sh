#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

usage() { printf 'Usage: scripts/package-native.sh --version vX.Y.Z --output DIR\n' >&2; }
version=''
output=''
while (( $# > 0 )); do
	case "$1" in
	--version) [[ $# -ge 2 ]] || { usage; exit 2; }; version=$2; shift 2 ;;
	--output) [[ $# -ge 2 ]] || { usage; exit 2; }; output=$2; shift 2 ;;
	*) usage; exit 2 ;;
	esac
done

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
	printf 'error: a canonical v-prefixed release version is required\n' >&2
	exit 2
fi
[[ -n "$output" ]] || { usage; exit 2; }
if ! git diff --quiet -- || ! git diff --cached --quiet --; then
	printf 'error: native release packaging requires a clean tracked worktree\n' >&2
	exit 1
fi
untracked_sources=$(git ls-files --others --exclude-standard -- \
	'*.go' '*.sando' 'go.mod' 'go.sum' 'vendor/**' || true)
if [[ -n "$untracked_sources" ]]; then
	printf 'error: untracked build inputs prevent trustworthy release provenance:\n%s\n' "$untracked_sources" >&2
	exit 1
fi
target="$(go env GOOS)/$(go env GOARCH)"
case "$target" in
darwin/arm64 | linux/amd64) ;;
*) printf 'error: unsupported maintained native target: %s\n' "$target" >&2; exit 1 ;;
esac
if [[ "$(go env GOVERSION)" != go1.26.7 && "$(go env GOVERSION)" != go1.27.0 ]]; then
	printf 'error: unsupported release toolchain: %s\n' "$(go env GOVERSION)" >&2
	exit 1
fi

temporary=$(mktemp -d "${TMPDIR:-/tmp}/himesan-native-package.XXXXXXXX")
temporary=$(CDPATH= cd -- "$temporary" && pwd -P)
cleanup() { rm -rf -- "$temporary"; }
trap cleanup EXIT HUP INT TERM

commit=$(git rev-parse HEAD)
tree=$(git rev-parse 'HEAD^{tree}')
source_date_epoch=$(git show -s --format=%ct HEAD)
go_version=$(go env GOVERSION)
target_os=${target%/*}
target_arch=${target#*/}
linker_flags="-buildid= -X gamertan.com/sandwich-hime/internal/version.Compiler=$version"
for pass in one two; do
	CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -trimpath \
		-ldflags "$linker_flags" -o "$temporary/himesan-$pass" ./cmd/himesan
done
if ! cmp -s "$temporary/himesan-one" "$temporary/himesan-two"; then
	printf 'error: repeated native builds were not byte-identical\n' >&2
	exit 1
fi
case "$target" in
darwin/arm64) expected='Mach-O 64-bit executable arm64' ;;
linux/amd64) expected='ELF 64-bit LSB executable, x86-64' ;;
esac
if ! file "$temporary/himesan-one" | grep -Fq "$expected"; then
	printf 'error: candidate has the wrong native executable format\n' >&2
	file "$temporary/himesan-one" >&2
	exit 1
fi

mkdir -p -- "$output"
go run ./cmd/himesan-release package \
	--version "$version" --commit "$commit" --tree "$tree" \
	--go-version "$go_version" --goos "$target_os" --goarch "$target_arch" \
	--binary "$temporary/himesan-one" --output "$output" \
	--source-date-epoch "$source_date_epoch"
printf 'Unsigned native package created. Signing and notarization were not performed.\n'
