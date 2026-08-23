<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Sanitized public source snapshots

`export-public-snapshot.sh` creates a host-neutral filesystem snapshot from a
committed Git tree. It does not initialize a repository, copy `.git`, configure
a remote, commit, tag, push, or publish anything.

Release use requires a clean worktree, a ref resolving exactly to `HEAD`, and
the exact-file policy committed at `scripts/public-snapshot.allow` in that
ref:

```sh
scripts/export-public-snapshot.sh \
  --mode release \
  --ref HEAD \
  --destination ../sandwich-hime-public-review
```

The destination must not exist and its canonical parent must be outside the
source worktree, its worktree-specific Git directory, and its shared Git common
directory. This includes ordinary `.git` directories and linked-worktree
metadata stored elsewhere. The exporter creates a private sibling staging
directory and renames it into place only after all checks pass. It never clears
or replaces an existing destination; failure cleanup is limited to a staging
directory carrying the exporter's ownership marker.

Review mode may use an externally reviewed exact-file policy while changes to
the exporter itself await a commit. Its provenance is conspicuously marked
`review` and is not a release artifact:

```sh
scripts/export-public-snapshot.sh \
  --mode review \
  --allowlist /path/to/reviewed-exact-files.allow \
  --destination ../snapshot-for-review
```

The policy accepts individual files only—never directories or globs. The
export fails for missing/duplicate/forbidden entries, non-regular Git objects,
symlinks, binary or oversized content, aggregate size limits, private developer
filesystem indicators, common private-key/token indicators, database or build
artifacts, and explicitly private integration material. Host workflow folders,
private trees, prototype/history trees, and application-specific integrations
are not in the reviewed policy.

`PUBLIC-SNAPSHOT.sha256` records every exported source file. The deterministic
`PUBLIC-SNAPSHOT.json` records only the project identifier, export policy and
mode, file count, and policy/manifest digests. Private commit and tree IDs,
commit timestamps, author or committer identity, email, hostname, branch name,
remote URL, and checkout path stay outside the exported tree. Filesystem
timestamps are normalized to the Unix epoch. A separate private release ledger
may map the private source commit to the resulting public commit and signed
tags.

When an exported tree is reviewed into an existing public checkout, compare and
copy files by content (for example, checksum-aware synchronization or a fresh
tree replacement). Size-and-modification-time shortcuts are unsafe here because
the exporter deliberately gives every snapshot the same normalized timestamp;
the manifest and provenance records must be verified again before publication.

Run the focused checks with:

```sh
bash scripts/test-public-snapshot.sh
```

The implementation expects Bash, Git, tar, GNU-compatible core utilities, and
a filesystem supporting an atomic rename within the destination parent.
