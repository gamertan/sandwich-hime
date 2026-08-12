#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

failures=0

fail() {
	printf 'license error: %s\n' "$*" >&2
	failures=$((failures + 1))
}

expected_spdx() {
	case "$1" in
		sando/*)
			printf 'Apache-2.0\n'
			;;
		*)
			printf 'AGPL-3.0-only\n'
			;;
	esac
}

# Read one complete SPDX expression from the first eight lines. Removing only
# recognized comment closers makes expressions such as "AGPL-3.0-only OR MIT"
# fail instead of passing a substring search.
has_exact_spdx() {
	local path=$1
	local expected=$2
	local lines line value count

	lines=$(head -n 8 -- "$path" | grep 'SPDX-License-Identifier:' || true)
	count=$(printf '%s\n' "$lines" | sed '/^$/d' | wc -l)
	count=${count//[[:space:]]/}
	[[ $count -eq 1 ]] || return 1
	line=$lines
	value=${line#*SPDX-License-Identifier:}
	value=${value%%-->*}
	value=${value%%\*/*}
	value=$(printf '%s' "$value" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
	[[ $value == "$expected" ]]
}

check_sha256() {
	local path=$1
	local expected=$2
	local actual
	actual=$(sha256sum -- "$path" | awk '{print $1}')
	[[ $actual == "$expected" ]] || fail "$path does not match the reviewed legal text ($actual)"
}

is_comment_capable_project_file() {
	case "$1" in
		COPYRIGHT | */COPYRIGHT | .editorconfig | .gitattributes | .gitignore | *.go | *.mod | *.md | *.sh | *.ps1 | *.yml | *.yaml | *.html | *.css | *.js | *.toml | *.allow)
			return 0
			;;
		*)
			return 1
			;;
	esac
}

list_project_files() {
	if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		git ls-files --cached --others --exclude-standard -z
	else
		find . -type d \( -name .git -o -name vendor -o -name bin -o -name dist -o -name coverage \) -prune -o \
			-type f -print0 | sed -z 's#^\./##'
	fi
}

[[ -f LICENSE ]] || fail 'root LICENSE is missing'
[[ -f sando/LICENSE ]] || fail 'sando/LICENSE is missing'
[[ -f DCO.txt ]] || fail 'DCO.txt is missing'

if [[ -f LICENSE ]]; then
	check_sha256 LICENSE 0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0
fi
if [[ -f sando/LICENSE ]]; then
	check_sha256 sando/LICENSE c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4
fi
if [[ -f DCO.txt ]]; then
	check_sha256 DCO.txt f7ac75b443f4ca16b503241344b41aeff9503b0c30bedc2b119551d83cb0fa90
fi

for required in COPYRIGHT OUTPUT_EXCEPTION.md sando/COPYRIGHT; do
	[[ -f $required ]] || fail "$required is required for ownership/output licensing"
done

if [[ -f COPYRIGHT ]]; then
	has_exact_spdx COPYRIGHT AGPL-3.0-only || fail 'COPYRIGHT must carry exactly AGPL-3.0-only'
	grep -Fq 'SPDX-FileCopyrightText: 2025-2026 Cole Speelman' COPYRIGHT || \
		fail 'COPYRIGHT must identify Cole Speelman original work'
fi
if [[ -f sando/COPYRIGHT ]]; then
	has_exact_spdx sando/COPYRIGHT Apache-2.0 || fail 'sando/COPYRIGHT must carry exactly Apache-2.0'
	grep -Fq 'SPDX-FileCopyrightText: 2025-2026 Cole Speelman' sando/COPYRIGHT || \
		fail 'sando/COPYRIGHT must identify Cole Speelman original runtime work'
fi
if [[ -f OUTPUT_EXCEPTION.md ]]; then
	grep -Fq 'additional permission under section 7' OUTPUT_EXCEPTION.md || \
		fail 'OUTPUT_EXCEPTION.md must contain the AGPL section 7 additional permission'
	grep -Fq 'Himesan-Output-Permission: v1.0' OUTPUT_EXCEPTION.md || \
		fail 'OUTPUT_EXCEPTION.md must define the contributor grant marker'
fi
if [[ -f CONTRIBUTING.md ]]; then
	grep -Fq 'Himesan-Output-Permission: v1.0' CONTRIBUTING.md || \
		fail 'CONTRIBUTING.md must require the emitted-scaffolding permission grant'
	grep -Fq 'DCO sign-off does not supply that separate grant' CONTRIBUTING.md || \
		fail 'CONTRIBUTING.md must distinguish DCO from the output permission'
fi

while IFS= read -r -d '' path; do
	[[ -f $path ]] || continue

	case "$path" in
		LICENSE | sando/LICENSE | DCO.txt)
			# These are reviewed legal texts with their own notices.
			continue
			;;
		*.sum)
			# Cryptographic dependency records are externally covered data.
			continue
			;;
		private/*.png | private/**/*.png)
			# A tracked private binary needs an exact entry in the private map.
			# The map and material remain outside reviewed public snapshots.
			grep -Fq "\`$path\`" private/LICENSES.md || \
				fail "$path needs an exact private license-map entry"
			continue
			;;
		PUBLIC-SNAPSHOT.json | PUBLIC-SNAPSHOT.sha256)
			# Generated factual provenance; covered by LICENSES.md.
			continue
			;;
		*.sando.go)
			if grep -Fq 'SPDX-License-Identifier:' "$path"; then
				fail "$path is compiler-managed output and must use its module-level license map"
			fi
			if grep -Fq 'Copyright (c) 2025-2026 Cole Speelman' "$path"; then
				fail "$path must not receive a compiler copyright claim"
			fi
			continue
			;;
		*.sando)
			expected=$(expected_spdx "$path")
			count=$(grep -Fc "SPDX-License-Identifier: $expected" "$path" || true)
			[[ $count -eq 1 ]] || fail "$path must carry one template comment for $expected"
			continue
			;;
		*.json)
			fail "$path cannot carry a comment and needs an explicit license-map entry"
			continue
			;;
	esac

	if ! is_comment_capable_project_file "$path"; then
		fail "$path has no fail-closed license policy"
		continue
	fi

	expected=$(expected_spdx "$path")
	has_exact_spdx "$path" "$expected" || \
		fail "$path must carry exactly one SPDX identifier: $expected"
done < <(list_project_files)

if find sando -type f -name '*.go' -exec grep -En \
	'"gamertan\.com/sandwich-hime/(cmd|internal)(/|"|$)' {} + | grep -q .; then
	fail 'the Apache runtime imports AGPL compiler or CLI code'
fi

if (( failures > 0 )); then
	printf '\n%d license/SPDX policy violation(s) found.\n' "$failures" >&2
	exit 1
fi

printf 'Reviewed license texts, ownership records, SPDX boundaries, generated-output permission, and runtime separation are consistent.\n'
