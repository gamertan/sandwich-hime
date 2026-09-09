#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
# shellcheck source=public-snapshot-lib.sh
source "$project_root/scripts/public-snapshot-lib.sh"
exporter=$project_root/scripts/export-public-snapshot.sh
temporary=$(mktemp -d)
cleanup() {
	rm -rf -- "$temporary"
}
trap cleanup EXIT

bash -n "$project_root/scripts/public-snapshot-lib.sh" "$exporter"
if grep -Eq 'git[[:space:]]+(init|commit|tag|push|remote)([[:space:]]|$)' "$exporter"; then
	echo "exporter contains a forbidden Git mutation command" >&2
	exit 1
fi

for required in COPYRIGHT OUTPUT_EXCEPTION.md sando/COPYRIGHT; do
	grep -Fxq "$required" "$project_root/scripts/public-snapshot.allow" || {
		echo "required legal boundary is absent from public allowlist: $required" >&2
		exit 1
	}
done

snapshot_status_is_clean ""
if snapshot_status_is_clean " M reviewed.go"; then
	echo "dirty status was accepted" >&2
	exit 1
fi
snapshot_path_is_at_or_below /var/tmp / || {
	echo "filesystem-root boundary did not contain an absolute path" >&2
	exit 1
}
if snapshot_path_is_at_or_below /safe-ish /safe; then
	echo "path boundary accepted a sibling prefix" >&2
	exit 1
fi
for path in .gitea/workflows/verify.yml .github/workflows/verify.yml private/notes.md \
	internal/integration/product/test.go go.work build/output.exe data/private.db history/prototype.go; do
	if ! snapshot_forbidden_path "$path"; then
		echo "private/build path was not rejected: $path" >&2
		exit 1
	fi
done
snapshot_forbidden_path README.md && { echo "safe path was rejected" >&2; exit 1; }

# Exercise the scanner against the complete proposed policy, including these
# uncommitted exporter files, so the next clean commit cannot reveal a
# self-triggering detector or a missing reviewed path.
proposed_tree=$temporary/proposed
mkdir -p "$proposed_tree"
while IFS= read -r line || [[ -n $line ]]; do
	[[ -n $line && ${line:0:1} != '#' ]] || continue
	[[ -f $project_root/$line && ! -L $project_root/$line ]] || {
		echo "reviewed allowlist path is missing or not regular: $line" >&2
		exit 1
	}
	mkdir -p "$proposed_tree/$(dirname -- "$line")"
	cp -p -- "$project_root/$line" "$proposed_tree/$line"
done <"$project_root/scripts/public-snapshot.allow"
snapshot_validate_export_tree "$proposed_tree"
(cd "$proposed_tree" && bash scripts/check-licenses.sh)
private_home_pattern='/'home'/'cole
private_commit_pattern='80bed136''75e8'
private_tag_pattern='prototype-''2025'
if LC_ALL=C grep -IRq -e "$private_home_pattern" -e "$private_commit_pattern" -e "$private_tag_pattern" "$proposed_tree"; then
	echo "public allowlist contains a private path or history identifier" >&2
	exit 1
fi

safe_tree=$temporary/safe
mkdir -p "$safe_tree"
printf 'ordinary reviewed source\n' >"$safe_tree/source.go"
snapshot_validate_export_tree "$safe_tree"

# Public prose and ordinary public links are allowed. Source-code policy
# fixtures may name excluded roots without becoming documentation links.
printf '%s\n' 'Private development records remain separate.' \
	'[Releases](docs/releases.md)' '[History](https://example.invalid/history/releases)' \
	'[Design](private-design.md)' >"$safe_tree/README.md"
printf '%s\n' '// Reject private/notes.md in an export policy.' >"$safe_tree/scanner_test.go"
snapshot_validate_export_tree "$safe_tree"

markdown_tree=$temporary/markdown
mkdir -p "$markdown_tree"
for reference in '[Audit](private/audit.md)' '[Audit](./private/audit.md#resume)' \
	'[Audit](../../private/audit.md)' '[audit]: ../private/audit.md' '[audit]:private/audit.md' \
	'`private/audit.md`' '<a href="private/audit.md">Audit</a>' \
	'[Archive](history/story.md)' '[Audit](PRIVATE/AUDIT.md)'; do
	printf '%s\n' "$reference" >"$markdown_tree/README.Md"
	if snapshot_validate_export_tree "$markdown_tree" >"$temporary/markdown.log" 2>&1; then
		echo "excluded documentation reference was accepted: $reference" >&2
		exit 1
	fi
	grep -q 'excluded documentation reference in README.Md' "$temporary/markdown.log"
done
mv "$markdown_tree/README.Md" "$markdown_tree/notes.markdown"
if snapshot_validate_export_tree "$markdown_tree" >/dev/null 2>&1; then
	echo "excluded reference in .markdown documentation was accepted" >&2
	exit 1
fi

empty_tree=$temporary/empty
mkdir -p "$empty_tree"
: >"$empty_tree/empty.txt"
snapshot_validate_export_tree "$empty_tree"

symlink_tree=$temporary/symlink
mkdir -p "$symlink_tree"
printf 'target\n' >"$symlink_tree/target"
ln -s target "$symlink_tree/link"
if snapshot_validate_export_tree "$symlink_tree" >/dev/null 2>&1; then
	echo "symlink tree was accepted" >&2
	exit 1
fi

oversized_tree=$temporary/oversized
mkdir -p "$oversized_tree"
printf '123456789\n' >"$oversized_tree/large.txt"
if SNAPSHOT_MAX_FILE_BYTES=8 snapshot_validate_export_tree "$oversized_tree" >/dev/null 2>&1; then
	echo "oversized file was accepted" >&2
	exit 1
fi

binary_tree=$temporary/binary
mkdir -p "$binary_tree"
printf 'text\000binary\n' >"$binary_tree/blob.dat"
if snapshot_validate_export_tree "$binary_tree" >/dev/null 2>&1; then
	echo "binary file was accepted" >&2
	exit 1
fi

private_tree=$temporary/private
mkdir -p "$private_tree"
printf '/%s/%s/project/private.db\n' home developer >"$private_tree/path.txt"
if snapshot_validate_export_tree "$private_tree" >/dev/null 2>&1; then
	echo "private filesystem path was accepted" >&2
	exit 1
fi

private_commit_tree=$temporary/private-commit
mkdir -p "$private_commit_tree"
printf 'Private development source: %040d\n' 0 >"$private_commit_tree/ledger.txt"
if snapshot_validate_export_tree "$private_commit_tree" >/dev/null 2>&1; then
	echo "private commit identifier was accepted" >&2
	exit 1
fi

private_repository_tree=$temporary/private-repository
mkdir -p "$private_repository_tree"
printf 'gamertan/%s%s\n' 'sandwich-hime-' 'dev' >"$private_repository_tree/source.txt"
if snapshot_validate_export_tree "$private_repository_tree" >/dev/null 2>&1; then
	echo "private repository identifier was accepted" >&2
	exit 1
fi

credential_tree=$temporary/credential
mkdir -p "$credential_tree"
printf '%s%s\n' '-----BEGIN ' 'PRIVATE KEY-----' >"$credential_tree/secret.txt"
if snapshot_validate_export_tree "$credential_tree" >/dev/null 2>&1; then
	echo "private key indicator was accepted" >&2
	exit 1
fi

# Until these new exporter files themselves are committed, construct a
# review-only policy containing the intersection of the reviewed policy and
# the selected committed source ref. No repository or Git object is mutated.
review_policy=$temporary/review.allow
while IFS= read -r line || [[ -n $line ]]; do
	[[ -n $line && ${line:0:1} != '#' ]] || continue
	if git -C "$project_root" cat-file -e "HEAD:$line" 2>/dev/null &&
		git -C "$project_root" diff --quiet HEAD -- "$line"; then
		printf '%s\n' "$line" >>"$review_policy"
	fi
done <"$project_root/scripts/public-snapshot.allow"

# A destination beneath the source worktree (including .git) must fail before
# staging creation. This test never removes anything from the source tree.
inside_name=himesan-export-must-not-exist-$$
inside_destination=$project_root/.git/$inside_name
[[ ! -e $inside_destination && ! -L $inside_destination ]] || {
	echo "in-worktree destination unexpectedly exists before test" >&2
	exit 1
}
if "$exporter" --source "$project_root" --ref HEAD --mode review \
	--allowlist "$review_policy" --destination "$inside_destination" >/dev/null 2>&1; then
	echo "exporter accepted a destination inside the source worktree" >&2
	exit 1
fi
[[ ! -e $inside_destination && ! -L $inside_destination ]] || {
	echo "failed in-worktree export created its destination" >&2
	exit 1
}
if find "$project_root/.git" -maxdepth 1 -name ".${inside_name}.himesan-public-export.*" -print -quit | grep -q .; then
	echo "failed in-worktree export created a staging directory" >&2
	exit 1
fi

# A linked worktree stores its private Git directory and shared common Git
# directory outside that worktree root. Neither metadata location may become
# an export destination. The isolated repositories live entirely in $temporary.
linked_main=$temporary/linked-main
linked_worktree=$temporary/linked-worktree
git init -q "$linked_main"
printf 'reviewed linked-worktree source\n' >"$linked_main/source.go"
git -C "$linked_main" add source.go
git -C "$linked_main" -c user.name='Snapshot Test' -c user.email='snapshot@example.invalid' \
	commit -qm 'seed isolated exporter test'
git -C "$linked_main" worktree add -q --detach "$linked_worktree" HEAD
linked_policy=$temporary/linked.allow
printf 'source.go\n' >"$linked_policy"
linked_git_dir=$(git -C "$linked_worktree" rev-parse --absolute-git-dir)
linked_common_dir=$(git -C "$linked_worktree" rev-parse --git-common-dir)
if [[ $linked_common_dir != /* ]]; then
	linked_common_dir=$linked_worktree/$linked_common_dir
fi
linked_common_dir=$(snapshot_realpath_existing "$linked_common_dir")

assert_metadata_destination_rejected() {
	local label=$1
	local parent=$2
	local name=$3
	local rejected_destination=$parent/$name
	[[ ! -e $rejected_destination && ! -L $rejected_destination ]] || {
		echo "$label destination unexpectedly exists before test" >&2
		exit 1
	}
	if "$exporter" --source "$linked_worktree" --ref HEAD --mode review \
		--allowlist "$linked_policy" --destination "$rejected_destination" >/dev/null 2>&1; then
		echo "exporter accepted destination inside $label" >&2
		exit 1
	fi
	[[ ! -e $rejected_destination && ! -L $rejected_destination ]] || {
		echo "failed $label export created its destination" >&2
		exit 1
	}
	if find "$parent" -maxdepth 1 -name ".${name}.himesan-public-export.*" -print -quit | grep -q .; then
		echo "failed $label export created a staging directory" >&2
		exit 1
	fi
}

assert_metadata_destination_rejected 'linked-worktree Git directory' "$linked_git_dir" linked-private-destination
assert_metadata_destination_rejected 'shared Git common directory' "$linked_common_dir" linked-common-destination

first=$temporary/public-one
second=$temporary/public-two
"$exporter" --source "$project_root" --ref HEAD --mode review \
	--allowlist "$review_policy" --destination "$first" >/dev/null
"$exporter" --source "$project_root" --ref HEAD --mode review \
	--allowlist "$review_policy" --destination "$second" >/dev/null
diff -r --no-dereference "$first" "$second" >/dev/null
(cd "$first" && sha256sum -c PUBLIC-SNAPSHOT.sha256 >/dev/null)

for excluded in .git .gitea .github private history internal/integration go.work; do
	[[ ! -e $first/$excluded && ! -L $first/$excluded ]] || {
		echo "excluded path reached snapshot: $excluded" >&2
		exit 1
	}
done
[[ -f $first/sando/component.go ]] || { echo "reviewed runtime source was not exported" >&2; exit 1; }
[[ -f $first/PUBLIC-SNAPSHOT.json && -f $first/PUBLIC-SNAPSHOT.sha256 ]] || {
	echo "public provenance files are missing" >&2
	exit 1
}
grep -q '"schema_version":2' "$first/PUBLIC-SNAPSHOT.json"
grep -q '"export_mode":"review"' "$first/PUBLIC-SNAPSHOT.json"
if grep -Fq "$project_root" "$first/PUBLIC-SNAPSHOT.json" \
	|| grep -Fq "$project_root" "$first/PUBLIC-SNAPSHOT.sha256" \
	|| grep -q '@' "$first/PUBLIC-SNAPSHOT.json" \
	|| grep -Eq '"(commit|tree|source_date_epoch)"' "$first/PUBLIC-SNAPSHOT.json"; then
	echo "public provenance exposed a checkout path or email" >&2
	exit 1
fi

occupied=$temporary/occupied
mkdir -p "$occupied"
printf 'do not delete\n' >"$occupied/owner-marker"
if "$exporter" --source "$project_root" --ref HEAD --mode review \
	--allowlist "$review_policy" --destination "$occupied" >/dev/null 2>&1; then
	echo "exporter accepted an existing destination" >&2
	exit 1
fi
grep -q 'do not delete' "$occupied/owner-marker"

status=$(git -C "$project_root" status --porcelain=v1 --untracked-files=all)
release_destination=$temporary/release
if snapshot_status_is_clean "$status"; then
	"$exporter" --source "$project_root" --ref HEAD --mode release \
		--destination "$release_destination" >/dev/null
else
	if "$exporter" --source "$project_root" --ref HEAD --mode release \
		--destination "$release_destination" >"$temporary/release.log" 2>&1; then
		echo "release export accepted a dirty source" >&2
		exit 1
	fi
	grep -q 'release source worktree is dirty' "$temporary/release.log"
	[[ ! -e $release_destination ]] || { echo "failed release created a destination" >&2; exit 1; }
fi

echo "public snapshot export checks passed"
