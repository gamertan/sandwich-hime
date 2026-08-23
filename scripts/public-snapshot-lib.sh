#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

# Sourced validation helpers for export-public-snapshot.sh and its focused
# tests. This file deliberately leaves the caller's shell options unchanged.

snapshot_status_is_clean() {
	[[ -z $1 ]]
}

snapshot_path_is_at_or_below() {
	local candidate=$1
	local boundary=$2

	[[ $boundary == / || $candidate == "$boundary" || $candidate == "$boundary/"* ]]
}

snapshot_realpath_existing() {
	local path=$1
	local directory base
	if [[ -d $path ]]; then
		(CDPATH= cd -- "$path" && pwd -P)
		return
	fi
	if [[ -f $path ]]; then
		directory=$(dirname -- "$path")
		base=$(basename -- "$path")
		directory=$(CDPATH= cd -- "$directory" && pwd -P) || return 1
		printf '%s/%s\n' "$directory" "$base"
		return
	fi
	return 1
}

snapshot_forbidden_path() {
	local path=$1
	local lower
	lower=$(printf '%s' "$path" | LC_ALL=C tr '[:upper:]' '[:lower:]')

	[[ $path != /* && $path != *\\* && $path != *//* ]] || return 0
	[[ $path != . && $path != .. && $path != ../* && $path != */../* && $path != */.. ]] || return 0
	[[ $path != *$'\n'* && $path != *$'\r'* && $path != *$'\t'* ]] || return 0

	case "/$lower/" in
	*/.git/* | */.gitea/* | */.github/* | */private/* | */prototype/* | */prototypes/* | */history/* | */legacy/* | */vendor/* | */bin/* | */dist/* | */coverage/* | */cmd/himetest/* | */cmd/himework/* | */internal/himesan/* | */internal/integration/* | */templates/*)
		return 0
		;;
	esac
	case $lower in
	go.work | go.work.sum | .env | .env.* | */.env | */.env.* | *.db | *.db-* | *.sqlite | *.sqlite3 | *.pem | *.key | *.p12 | *.pfx | */id_rsa | */id_ed25519 | *credentials* | *.exe | *.dll | *.dylib | *.so | *.a | *.o | *.test | *.prof | *.cover | *.zip | *.tar | *.tar.gz | *.tgz)
		return 0
		;;
	esac
	return 1
}

snapshot_validate_export_tree() {
	local root=$1
	local marker=${2:-}
	local max_file_bytes=${SNAPSHOT_MAX_FILE_BYTES:-1048576}
	local max_total_bytes=${SNAPSHOT_MAX_TOTAL_BYTES:-16777216}
	local max_files=${SNAPSHOT_MAX_FILES:-2000}
	local total=0
	local count=0
	local file rel size
	local users_word=Users
	local private_unix="/(home|${users_word})/[^/[:space:]]+"
	local private_windows='[A-Za-z]:[\\/]+Users[\\/]'
	local private_wsl="/mnt/[a-zA-Z]/${users_word}/"
	local pem_begin='-----BEGIN '
	local private_key="${pem_begin}([A-Z0-9]+ )?PRIVATE KEY-----|${pem_begin}PGP PRIVATE KEY BLOCK-----"
	local provider_token='AKIA[0-9A-Z]{16}|(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}'
	local private_commit_id='(private development (source|baseline)|private (development )?(commit|source))[^[:cntrl:]]*[0-9a-f]{12,64}'
	local private_repository='sandwich-hime-''dev'

	if find "$root" -type l -print -quit | grep -q .; then
		echo "public snapshot: symbolic links are forbidden" >&2
		return 1
	fi

	while IFS= read -r -d '' file; do
		rel=${file#"$root"/}
		[[ -z $marker || $rel != "$marker" ]] || continue
		if snapshot_forbidden_path "$rel"; then
			echo "public snapshot: forbidden path: $rel" >&2
			return 1
		fi
		size=$(wc -c <"$file")
		size=${size//[[:space:]]/}
		if ((size > max_file_bytes)); then
			echo "public snapshot: oversized file: $rel ($size bytes)" >&2
			return 1
		fi
		total=$((total + size))
		count=$((count + 1))
		if ((total > max_total_bytes || count > max_files)); then
			echo "public snapshot: export exceeds aggregate size/count limits" >&2
			return 1
		fi
		if [[ -s $file ]] && ! LC_ALL=C grep -Iq . "$file"; then
			echo "public snapshot: binary file rejected: $rel" >&2
			return 1
		fi
		if LC_ALL=C grep -Eq "$private_unix|$private_windows|$private_wsl" "$file"; then
			echo "public snapshot: private filesystem path indicator in $rel" >&2
			return 1
		fi
		if LC_ALL=C grep -Eq -- "$private_key|$provider_token" "$file"; then
			echo "public snapshot: key or credential indicator in $rel" >&2
			return 1
		fi
		if LC_ALL=C grep -Eiq -- "$private_commit_id" "$file"; then
			echo "public snapshot: private commit identifier in $rel" >&2
			return 1
		fi
		if LC_ALL=C grep -Fq -- "$private_repository" "$file"; then
			echo "public snapshot: private repository indicator in $rel" >&2
			return 1
		fi
	done < <(find "$root" -type f -print0 | LC_ALL=C sort -z)
}
