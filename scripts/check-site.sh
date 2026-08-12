#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

public_version=''
if (( $# > 0 )); then
	if [[ "$1" != --public || $# -ne 2 ]]; then
		printf 'usage: scripts/check-site.sh [--public vX.Y.Z]\n' >&2
		exit 2
	fi
	public_version=$2
fi

fail() {
	printf 'site error: %s\n' "$*" >&2
	exit 1
}

for page in site/index.html site/sando/index.html; do
	[[ -f "$page" ]] || fail "missing $page"
	grep -Fq '<html lang="en">' "$page" || fail "$page needs a document language"
	grep -Fq '<meta name="viewport"' "$page" || fail "$page needs responsive viewport metadata"
	grep -Fq 'class="skip-link"' "$page" || fail "$page needs a keyboard skip link"
	grep -Fq '<main id="main">' "$page" || fail "$page needs the skip-link target"
	grep -Fq '<h1>' "$page" || grep -Fq '<h1 ' "$page" || fail "$page needs an h1"
	grep -Fq 'Content-Security-Policy' "$page" || fail "$page needs a preview CSP"
	grep -Fq '<meta name="himesan-release-status" content="' "$page" || \
		fail "$page needs machine-readable release status"
done

grep -Fq '<meta name="go-import" content="gamertan.com/sandwich-hime git https://gitea.speelman.ca/gamertan/sandwich-hime.git">' \
	site/index.html || fail 'compiler vanity-import metadata is missing or changed'
grep -Fq '<meta name="go-import" content="gamertan.com/sandwich-hime/sando git https://gitea.speelman.ca/gamertan/sandwich-hime.git sando">' \
	site/sando/index.html || fail 'nested runtime vanity-import metadata is missing or changed'

if find site -type f -name '*.html' -exec grep -Ein '<script([[:space:]>])' {} + | grep -q .; then
	fail 'the static project site must not contain JavaScript'
fi
if find site -type f -name '*.html' -exec grep -Ein \
	'(src|href)="https?://[^" ]+\.(js|css)([?"#])' {} + | grep -q .; then
	fail 'the static project site must not load remote JavaScript or CSS'
fi
if find site -type f -name '*.css' -exec grep -Ein \
	"(@import|url\\()[[:space:]\"']*https?://" {} + | grep -q .; then
	fail 'the static project site must not load remote CSS assets'
fi

grep -Fq 'prefers-reduced-motion' site/assets/site.css || fail 'site CSS needs a reduced-motion preference'
grep -Fq 'forced-colors' site/assets/site.css || fail 'site CSS needs a forced-colors fallback'
grep -Fq 'class="wordmark" role="img"' site/index.html || \
	fail 'the ASCII wordmark needs an accessible semantic role'
grep -Fq 'class="code" tabindex="0" role="region"' site/index.html || \
	fail 'the scrollable code example needs keyboard access and a region role'

if [[ -n "$public_version" ]]; then
	for page in site/index.html site/sando/index.html; do
		grep -Fq "<meta name=\"himesan-release-status\" content=\"$public_version\">" "$page" || \
			fail "$page release status does not match $public_version"
	done
	if grep -Eiq 'not a public release|not released yet|private pre-release|public pre-1\.0|unsupported pre-1\.0|no (supported )?public .*tag' \
		site/index.html site/sando/index.html; then
		fail 'public-release site still contains a pre-release warning'
	fi
else
	if grep -Fq '<meta name="himesan-release-status" content="pre-release">' site/index.html; then
		grep -Eiq 'public pre-1\.0|unsupported pre-1\.0|not released yet' site/index.html || \
			fail 'the pre-release landing page must state its status in human-readable text'
	fi
fi

printf 'Static site metadata, local-asset policy, and accessibility scaffolding are present.\n'
