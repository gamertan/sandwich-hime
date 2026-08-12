#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

usage() {
	cat <<'EOF'
Usage: scripts/release-check.sh --version vX.Y.Z [--public]

Runs a read-only release preflight. It never creates tags, commits, release
artifacts in the repository, pushes, or deploys.

  --version  Candidate compiler version. The corresponding runtime tag is
             sando/vX.Y.Z.
  --public   Require the human-reviewed RC/final launch evidence bundle named
             by HIMESAN_RELEASE_EVIDENCE_DIR. Canonical beta prereleases may
             run their narrower publication preflight without this flag.
EOF
}

version=''
public_release=0
while (( $# > 0 )); do
	case "$1" in
		--version)
			[[ $# -ge 2 ]] || { usage >&2; exit 2; }
			version=$2
			shift 2
			;;
		--public)
			public_release=1
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*)
			printf 'unknown argument: %s\n' "$1" >&2
			usage >&2
			exit 2
			;;
	esac
done

if [[ ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$ ]]; then
	printf 'error: --version must be a canonical semantic version beginning with v (build metadata is not allowed)\n' >&2
	exit 2
fi
prerelease=${BASH_REMATCH[5]:-}
if [[ "$prerelease" =~ (^|[.-])(0\.)?[0-9]{14}-[0-9a-f]{12,}$ ]]; then
	printf 'error: --version must be a signed release tag, not a Go pseudo-version\n' >&2
	exit 2
fi
if [[ -n "$prerelease" ]]; then
	IFS=. read -r -a prerelease_identifiers <<<"$prerelease"
	for identifier in "${prerelease_identifiers[@]}"; do
		if [[ "$identifier" =~ ^[0-9]+$ && "$identifier" =~ ^0[0-9]+$ ]]; then
			printf 'error: numeric prerelease identifiers must not contain leading zeroes: %s\n' "$identifier" >&2
			exit 2
		fi
	done
fi

beta_release=0
if [[ "$prerelease" == beta || "$prerelease" == beta.* ]]; then
	beta_release=1
fi
if (( public_release == 0 && beta_release == 0 )); then
	printf 'error: RC and final release preflights require --public and the human-reviewed evidence bundle\n' >&2
	exit 2
fi

runtime_tag="sando/$version"

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
	printf 'error: release preflight requires a clean canonical checkout\n' >&2
	exit 1
fi

origin_url=$(git remote get-url origin)
case "$origin_url" in
	ssh://git@gitea.speelman.ca:2222/gamertan/sandwich-hime.git | \
	git@gitea.speelman.ca:gamertan/sandwich-hime.git | \
	https://gitea.speelman.ca/gamertan/sandwich-hime.git)
		;;
	*)
		printf 'error: origin is not the canonical Gitea repository: %s\n' "$origin_url" >&2
		exit 1
		;;
esac

branch=$(git symbolic-ref --quiet --short HEAD || true)
if [[ "$branch" != main ]]; then
	printf 'error: release preflight must run from canonical main, not %s\n' "${branch:-detached HEAD}" >&2
	exit 1
fi

for tag in "$version" "$runtime_tag"; do
	if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
		printf 'error: candidate tag already exists locally: %s\n' "$tag" >&2
		exit 1
	fi
	if ! remote_tags=$(git ls-remote --tags origin "refs/tags/$tag" "refs/tags/$tag^{}" 2>/dev/null); then
		printf 'error: could not verify candidate tag against canonical origin: %s\n' "$tag" >&2
		exit 1
	fi
	if [[ -n "$remote_tags" ]]; then
		printf 'error: candidate tag already exists on canonical origin: %s\n' "$tag" >&2
		exit 1
	fi
done

artifact_dir=$(mktemp -d "${TMPDIR:-/tmp}/himesan-release-check.XXXXXXXX")
cleanup() {
	if [[ -n "${artifact_dir:-}" && -d "$artifact_dir" ]]; then
		rm -rf -- "$artifact_dir"
	fi
}
trap cleanup EXIT HUP INT TERM

compiler_linker_flags="-X gamertan.com/sandwich-hime/internal/version.Compiler=$version"
candidate_binary="$artifact_dir/himesan-candidate"

printf '\n==> exact release candidate identity\n'
go build -trimpath -ldflags "$compiler_linker_flags" -o "$candidate_binary" ./cmd/himesan
candidate_go_version=$(go env GOVERSION)
expected_human_version="himesan $version (runtime ABI sando.v1, $candidate_go_version)"
actual_human_version=$("$candidate_binary" version)
if [[ "$actual_human_version" != "$expected_human_version" ]]; then
	printf 'error: candidate human version mismatch\nexpected: %s\nactual:   %s\n' \
		"$expected_human_version" "$actual_human_version" >&2
	exit 1
fi
expected_json_version=$(printf '{"compiler":"%s","runtime_abi":"sando.v1","go":"%s"}' "$version" "$candidate_go_version")
actual_json_version=$("$candidate_binary" version --json)
if [[ "$actual_json_version" != "$expected_json_version" ]]; then
	printf 'error: candidate JSON version mismatch\nexpected: %s\nactual:   %s\n' \
		"$expected_json_version" "$actual_json_version" >&2
	exit 1
fi

file_mtime() {
	if stat --printf='%y' "$1" >/dev/null 2>&1; then
		stat --printf='%y' "$1"
	else
		stat -f '%m' "$1"
	fi
}

printf '\n==> candidate generated-output provenance compatibility\n'
golden_source=internal/compiler/testdata/golden/basic.sando
golden_output="$golden_source.go"
if ! grep -Fqx '// himesan:compiler 0.1.0-dev' "$golden_output"; then
	printf 'error: golden fixture no longer provides development-to-release provenance coverage: %s\n' "$golden_output" >&2
	exit 1
fi
golden_hash_before=$(git hash-object "$golden_output")
golden_mtime_before=$(file_mtime "$golden_output")
check_summary=$("$candidate_binary" check "$golden_source")
if [[ "$check_summary" != 'checked 1 .sando files: 1 current' ]]; then
	printf 'error: candidate did not consider the development-produced golden current: %s\n' "$check_summary" >&2
	exit 1
fi
for pass in 1 2; do
	generate_summary=$("$candidate_binary" generate "$golden_source")
	if [[ "$generate_summary" != 'generated 0, unchanged 1 (1 .sando files)' ]]; then
		printf 'error: candidate generation pass %d reported unexpected changes: %s\n' "$pass" "$generate_summary" >&2
		exit 1
	fi
	if [[ "$(git hash-object "$golden_output")" != "$golden_hash_before" ]]; then
		printf 'error: candidate generation pass %d changed golden bytes\n' "$pass" >&2
		exit 1
	fi
	if [[ "$(file_mtime "$golden_output")" != "$golden_mtime_before" ]]; then
		printf 'error: candidate generation pass %d changed the golden mtime\n' "$pass" >&2
		exit 1
	fi
done

./scripts/check-licenses.sh
HIMESAN_RACE=1 ./scripts/verify.sh

printf '\n==> bounded compiler fuzz gates\n'
go test ./internal/compiler -run '^$' -fuzz '^FuzzCompileNeverPanics$' -fuzztime=20s
go test ./internal/compiler -run '^$' -fuzz '^FuzzGoDelimiterNeverPanics$' -fuzztime=20s

printf '\n==> vulnerability scan (pinned golang.org/x/vuln v1.6.0)\n'
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
(
	cd sando
	go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
)

printf '\n==> cross-compiling release binary smoke set\n'
for target in \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64; do
	target_os=${target%/*}
	target_arch=${target#*/}
	extension=''
	if [[ "$target_os" == windows ]]; then
		extension='.exe'
	fi
	CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
		go build -trimpath -ldflags "$compiler_linker_flags" \
		-o "$artifact_dir/himesan-$target_os-$target_arch$extension" ./cmd/himesan
done

for required in \
	scripts/verify-public-install.sh \
	RELEASE.md \
	SECURITY.md \
	TRADEMARKS.md \
	CLA.md; do
	[[ -f "$required" ]] || { printf 'error: required release file is missing: %s\n' "$required" >&2; exit 1; }
done

if (( public_release == 1 )); then
	evidence_dir=${HIMESAN_RELEASE_EVIDENCE_DIR:-}
	if [[ -z "$evidence_dir" || ! -d "$evidence_dir" ]]; then
		printf 'error: --public requires HIMESAN_RELEASE_EVIDENCE_DIR\n' >&2
		exit 1
	fi
	for evidence in \
		legal-review.md \
		cross-platform.md \
		security.md \
		development-supervisor.md \
		benchmark-methodology.md \
		vanity-imports.md \
		signing-and-recovery.md; do
		if [[ ! -s "$evidence_dir/$evidence" ]]; then
			printf 'error: public release evidence is missing or empty: %s\n' "$evidence_dir/$evidence" >&2
			exit 1
		fi
	done
fi

if [[ -n "$(git status --porcelain=v1 --untracked-files=all)" ]]; then
	printf 'error: release preflight left tracked changes or untracked artifacts in the canonical checkout\n' >&2
	git status --short >&2
	exit 1
fi

if (( public_release == 1 )); then
	printf '\nHuman review is still required; evidence presence is not automatic approval.\n'
else
	printf '\nBeta technical publication preflight passed. This does not establish RC/final launch evidence or production stability.\n'
fi

printf 'No tag, push, publication, or deployment was performed for %s / %s.\n' "$version" "$runtime_tag"
