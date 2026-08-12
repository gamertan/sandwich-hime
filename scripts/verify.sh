#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

log() {
	printf '\n==> %s\n' "$*"
}

run_module_checks() {
	local module_dir=$1
	local label=$2

	log "$label: go test"
	(
		cd "$module_dir"
		go test ./...
	)

	log "$label: go vet"
	(
		cd "$module_dir"
		go vet ./...
	)
}

golden_sources() {
	find internal/compiler/testdata/golden \
		-type f -name '*.sando' -print | LC_ALL=C sort
}

generated_manifest() {
	local source output digest modified
	while IFS= read -r source; do
		[[ -n "$source" ]] || continue
		output="${source}.go"
		if [[ ! -f "$output" ]]; then
			printf 'missing  %s\n' "$output"
			continue
		fi
		if command -v sha256sum >/dev/null 2>&1; then
			digest=$(sha256sum -- "$output" | awk '{print $1}')
		elif command -v shasum >/dev/null 2>&1; then
			digest=$(shasum -a 256 -- "$output" | awk '{print $1}')
		else
			printf 'error: sha256sum or shasum is required for generation verification\n' >&2
			return 1
		fi
		if stat -c '%Y' -- "$output" >/dev/null 2>&1; then
			modified=$(stat -c '%Y' -- "$output")
		else
			modified=$(stat -f '%m' -- "$output")
		fi
		printf '%s  %s  %s\n' "$digest" "$modified" "$output"
	done < <(golden_sources)
}

log "repository scripts: shell syntax"
bash -n scripts/*.sh

run_module_checks . "compiler module"

log "compiler module: go build"
build_dir=$(mktemp -d "${TMPDIR:-/tmp}/himesan-verify.XXXXXXXX")
cleanup() {
	if [[ -n "${build_dir:-}" && -d "$build_dir" ]]; then
		rm -rf -- "$build_dir"
	fi
}
trap cleanup EXIT HUP INT TERM
go build -trimpath -o "$build_dir/himesan" ./cmd/himesan

if [[ ! -f sando/go.mod ]]; then
	printf 'error: nested Apache runtime module sando/go.mod is missing\n' >&2
	exit 1
fi
run_module_checks sando "sando runtime module"

sources=()
while IFS= read -r source; do
	[[ -n "$source" ]] || continue
	sources[${#sources[@]}]=$source
done < <(golden_sources)
if (( ${#sources[@]} == 0 )); then
	printf 'error: compiler-owned golden .sando fixture is missing\n' >&2
	exit 1
else
	log "golden generation: read-only freshness check"
	go run ./cmd/himesan check "${sources[@]}"

	manifest_before="$build_dir/generated-before.txt"
	manifest_first="$build_dir/generated-first.txt"
	manifest_second="$build_dir/generated-second.txt"
	generated_manifest >"$manifest_before"

	log "golden generation: first deterministic pass"
	go run ./cmd/himesan generate "${sources[@]}"
	generated_manifest >"$manifest_first"
	if ! cmp -s "$manifest_before" "$manifest_first"; then
		printf 'error: generation changed committed output after check declared it fresh\n' >&2
		diff -u "$manifest_before" "$manifest_first" || true
		exit 1
	fi

	log "golden generation: second deterministic pass"
	go run ./cmd/himesan generate "${sources[@]}"
	generated_manifest >"$manifest_second"
	if ! cmp -s "$manifest_first" "$manifest_second"; then
		printf 'error: repeated generation changed output bytes or an unchanged timestamp\n' >&2
		diff -u "$manifest_first" "$manifest_second" || true
		exit 1
	fi

	go run ./cmd/himesan check "${sources[@]}"
fi

if [[ "${HIMESAN_RACE:-0}" == 1 ]]; then
	log "compiler module: race tests"
	go test -race ./...
	log "sando runtime module: race tests"
	(
		cd sando
		go test -race ./...
	)
fi

log "verification complete"
