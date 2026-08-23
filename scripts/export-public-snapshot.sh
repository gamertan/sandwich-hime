#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_root/.." && pwd)
# shellcheck source=public-snapshot-lib.sh
source "$script_root/public-snapshot-lib.sh"
# Release safety limits are policy, not caller-tunable settings.
SNAPSHOT_MAX_FILE_BYTES=1048576
SNAPSHOT_MAX_TOTAL_BYTES=16777216
SNAPSHOT_MAX_FILES=2000

usage() {
	cat >&2 <<'USAGE'
Usage: export-public-snapshot.sh --destination PATH [options]

Options:
  --source PATH       Git worktree root (default: repository containing script)
  --ref REF           Committed source ref (default: HEAD)
  --mode MODE         release (default) or review
  --allowlist PATH    Review mode only: audited external exact-file policy

The destination must not exist and must be outside the source worktree. The
exporter creates it atomically and never initializes Git, configures a remote,
commits, tags, pushes, or copies .git.
USAGE
}

source_path=$project_root
source_ref=HEAD
destination=""
mode=release
allowlist_override=""
while [[ $# -gt 0 ]]; do
	case $1 in
	--source)
		[[ $# -ge 2 ]] || { usage; exit 2; }
		source_path=$2
		shift 2
		;;
	--ref)
		[[ $# -ge 2 ]] || { usage; exit 2; }
		source_ref=$2
		shift 2
		;;
	--destination)
		[[ $# -ge 2 ]] || { usage; exit 2; }
		destination=$2
		shift 2
		;;
	--mode)
		[[ $# -ge 2 ]] || { usage; exit 2; }
		mode=$2
		shift 2
		;;
	--allowlist)
		[[ $# -ge 2 ]] || { usage; exit 2; }
		allowlist_override=$2
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "public snapshot: unknown argument: $1" >&2
		usage
		exit 2
		;;
	esac
done

[[ -n $destination ]] || { usage; exit 2; }
[[ $mode == release || $mode == review ]] || { echo "public snapshot: mode must be release or review" >&2; exit 2; }
[[ $source_ref != -* && $source_ref != *$'\n'* && $source_ref != *$'\r'* ]] || {
	echo "public snapshot: invalid source ref" >&2
	exit 2
}
if [[ $mode == release && -n $allowlist_override ]]; then
	echo "public snapshot: release mode requires the allowlist committed in the source ref" >&2
	exit 2
fi

source_path=$(snapshot_realpath_existing "$source_path")
git_root=$(git -C "$source_path" rev-parse --show-toplevel 2>/dev/null) || {
	echo "public snapshot: source is not a Git worktree" >&2
	exit 1
}
git_root=$(snapshot_realpath_existing "$git_root")
[[ $source_path == "$git_root" ]] || { echo "public snapshot: --source must name the worktree root" >&2; exit 1; }
git_dir=$(git -C "$git_root" rev-parse --absolute-git-dir 2>/dev/null) || {
	echo "public snapshot: cannot resolve source Git metadata directory" >&2
	exit 1
}
git_common_dir=$(git -C "$git_root" rev-parse --git-common-dir 2>/dev/null) || {
	echo "public snapshot: cannot resolve source Git common directory" >&2
	exit 1
}
git_dir=$(snapshot_realpath_existing "$git_dir")
if [[ $git_common_dir != /* ]]; then
	git_common_dir=$git_root/$git_common_dir
fi
git_common_dir=$(snapshot_realpath_existing "$git_common_dir")
commit=$(git -C "$git_root" rev-parse --verify "${source_ref}^{commit}" 2>/dev/null) || {
	echo "public snapshot: source ref does not resolve to a commit" >&2
	exit 1
}
[[ $commit =~ ^[0-9a-f]{40}$ || $commit =~ ^[0-9a-f]{64}$ ]] || {
	echo "public snapshot: source commit is not a full object ID" >&2
	exit 1
}

if [[ $mode == release ]]; then
	head_commit=$(git -C "$git_root" rev-parse --verify HEAD^{commit})
	[[ $commit == "$head_commit" ]] || { echo "public snapshot: release ref must resolve to HEAD" >&2; exit 1; }
	status=$(git -C "$git_root" status --porcelain=v1 --untracked-files=all)
	snapshot_status_is_clean "$status" || {
		echo "public snapshot: release source worktree is dirty" >&2
		exit 1
	}
fi

destination_parent=$(dirname -- "$destination")
destination_name=$(basename -- "$destination")
[[ $destination_name != . && $destination_name != .. && -n $destination_name ]] || {
	echo "public snapshot: invalid destination name" >&2
	exit 2
}
destination_parent=$(snapshot_realpath_existing "$destination_parent")
[[ -d $destination_parent && ! -L $destination_parent ]] || {
	echo "public snapshot: destination parent must be an existing non-symlink directory" >&2
	exit 1
}
for protected_root in "$git_root" "$git_dir" "$git_common_dir"; do
	if snapshot_path_is_at_or_below "$destination_parent" "$protected_root"; then
		echo "public snapshot: destination must be outside the source worktree and Git metadata" >&2
		exit 1
	fi
done
destination=$destination_parent/$destination_name
[[ ! -e $destination && ! -L $destination ]] || {
	echo "public snapshot: destination already exists; refusing to alter it" >&2
	exit 1
}

staging=$(mktemp -d "$destination_parent/.${destination_name}.himesan-public-export.XXXXXX")
marker_name=.himesan-public-export-owned
marker=$staging/$marker_name
printf 'owned temporary public snapshot staging directory\n' >"$marker"
cleanup() {
	local status=$?
	if [[ -n ${staging:-} && -d $staging && -f $marker ]]; then
		case $staging in
		"$destination_parent"/."$destination_name".himesan-public-export.*)
			rm -rf -- "$staging"
			;;
		esac
	fi
	exit "$status"
}
trap cleanup EXIT

policy_file=$staging/.himesan-policy-input
if [[ -n $allowlist_override ]]; then
	[[ $mode == review ]] || { echo "public snapshot: external policy is review-only" >&2; exit 2; }
	[[ -f $allowlist_override && ! -L $allowlist_override ]] || {
		echo "public snapshot: external allowlist must be a regular non-symlink file" >&2
		exit 1
	}
	cp -- "$allowlist_override" "$policy_file"
else
	policy_path=scripts/public-snapshot.allow
	git -C "$git_root" cat-file -e "$commit:$policy_path" 2>/dev/null || {
		echo "public snapshot: committed ref lacks $policy_path" >&2
		exit 1
	}
	git -C "$git_root" show "$commit:$policy_path" >"$policy_file"
fi
policy_sha256=$(sha256sum "$policy_file" | awk '{print $1}')

seen_lines=$'\n'
paths=()
while IFS= read -r line || [[ -n $line ]]; do
	[[ -n $line && ${line:0:1} != '#' ]] || continue
	if [[ $line == *[[:space:]]* ]] || snapshot_forbidden_path "$line"; then
		echo "public snapshot: invalid or forbidden allowlist entry: $line" >&2
		exit 1
	fi
	case $seen_lines in
	*$'\n'"$line"$'\n'*)
		echo "public snapshot: duplicate allowlist entry: $line" >&2
		exit 1
		;;
	esac
	seen_lines=$seen_lines$line$'\n'
	record=$(git -C "$git_root" ls-tree "$commit" -- "$line")
	[[ -n $record && ${record#*$'\t'} == "$line" && $record != *$'\n'* ]] || {
		echo "public snapshot: allowlisted path is absent or ambiguous in source ref: $line" >&2
		exit 1
	}
	read -r object_mode object_type object_id <<<"${record%%$'\t'*}"
	[[ $object_type == blob && ($object_mode == 100644 || $object_mode == 100755) ]] || {
		echo "public snapshot: allowlisted path is not a regular file: $line" >&2
		exit 1
	}
	blob_size=$(git -C "$git_root" cat-file -s "$object_id")
	((blob_size <= SNAPSHOT_MAX_FILE_BYTES)) || {
		echo "public snapshot: allowlisted blob is oversized: $line" >&2
		exit 1
	}
	paths+=("$line")
done <"$policy_file"
[[ ${#paths[@]} -gt 0 ]] || { echo "public snapshot: allowlist selected no files" >&2; exit 1; }

sorted_paths=()
while IFS= read -r -d '' path; do
	sorted_paths[${#sorted_paths[@]}]=$path
done < <(printf '%s\0' "${paths[@]}" | LC_ALL=C sort -z)
git -C "$git_root" archive --format=tar "$commit" -- "${sorted_paths[@]}" | tar -xf - -C "$staging"
unlink "$policy_file"

exported_count=$(find "$staging" -type f ! -name "$marker_name" | wc -l)
exported_count=${exported_count//[[:space:]]/}
[[ $exported_count -eq ${#sorted_paths[@]} ]] || {
	echo "public snapshot: extracted file count does not match allowlist" >&2
	exit 1
}
snapshot_validate_export_tree "$staging" "$marker_name"

manifest=$staging/PUBLIC-SNAPSHOT.sha256
manifest_input=$staging/.himesan-manifest-input
(cd "$staging" && find . -type f ! -name "$marker_name" ! -name .himesan-manifest-input -print0 | LC_ALL=C sort -z | xargs -0 sha256sum) >"$manifest_input"
mv "$manifest_input" "$manifest"
manifest_sha256=$(sha256sum "$manifest" | awk '{print $1}')
provenance=$staging/PUBLIC-SNAPSHOT.json
printf '{"schema_version":2,"project":"sandwich-hime","export_policy":"exact-allowlist-v1","export_mode":"%s","file_count":%s,"allowlist_sha256":"%s","manifest_sha256":"%s"}\n' \
	"$mode" "$exported_count" "$policy_sha256" "$manifest_sha256" >"$provenance"

# Normalize filesystem metadata to a public constant as well as normalizing
# content. Private commit IDs, tree IDs, timestamps, identities, refs, remote
# URLs, and checkout paths do not enter the exported tree.
TZ=UTC find "$staging" -exec touch -t 197001010000 {} +
unlink "$marker"
if mv --help 2>&1 | grep -q -- '-T'; then
	mv -nT -- "$staging" "$destination"
else
	# BSD mv has no -T. The existing-destination preflight above preserves the
	# same no-overwrite policy for the local macOS review lane.
	mv -n "$staging" "$destination"
fi
if [[ -e $staging || ! -d $destination ]]; then
	echo "public snapshot: destination appeared during activation; staging was not published" >&2
	exit 1
fi
staging=""
printf 'public_snapshot=%s\ncommit=%s\nfiles=%s\n' "$destination" "$commit" "$exported_count"
