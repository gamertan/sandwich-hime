#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

usage() {
	cat >&2 <<'EOF'
Usage: scripts/sign-notarize-macos.sh --archive FILE --sha256 DIGEST --output DIR --keychain-profile NAME

Run this manually from Cole's signed-in macOS account. It never runs in CI.
It signs the native CLI, creates and signs a DMG, submits that DMG to Apple's
notary service, staples its ticket, and validates the distribution.
EOF
}

archive=''
archive_sha256=''
output=''
profile=''
identity='Developer ID Application: Cole Speelman (5BXR9JCUBL)'
identifier='com.gamertan.sandwich-hime.himesan'
while (( $# > 0 )); do
	case "$1" in
	--archive) archive=$2; shift 2 ;;
	--sha256) archive_sha256=$2; shift 2 ;;
	--output) output=$2; shift 2 ;;
	--keychain-profile) profile=$2; shift 2 ;;
	*) usage; exit 2 ;;
	esac
done
if [[ -z "$archive" || -z "$archive_sha256" || -z "$output" || -z "$profile" ]]; then usage; exit 2; fi
if [[ ! "$archive_sha256" =~ ^[0-9a-f]{64}$ ]]; then
	printf 'error: --sha256 must be the approved lowercase archive digest\n' >&2
	exit 2
fi
if [[ "$(uname -s)/$(uname -m)" != Darwin/arm64 ]]; then
	printf 'error: signing must run natively on Apple Silicon macOS\n' >&2
	exit 1
fi

temporary=$(mktemp -d "${TMPDIR:-/tmp}/himesan-notarize.XXXXXXXX")
temporary=$(CDPATH= cd -- "$temporary" && pwd -P)
cleanup() { rm -rf -- "$temporary"; }
trap cleanup EXIT HUP INT TERM

go run ./cmd/himesan-release extract-macos \
	--archive "$archive" --sha256 "$archive_sha256" --output "$temporary"
root=$(find "$temporary" -mindepth 1 -maxdepth 1 -type d -print -quit)
binary="$root/himesan"
[[ -x "$binary" ]] || { printf 'error: archive does not contain executable himesan\n' >&2; exit 1; }

codesign --force --options runtime --timestamp \
	--identifier "$identifier" --sign "$identity" "$binary"
codesign --verify --strict --verbose=2 "$binary"
go run ./cmd/himesan-release finalize-macos \
	--directory "$root" \
	--unsigned-archive-sha256 "$archive_sha256" \
	--identity "$identity" --identifier "$identifier" \
	--finalized-at "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
codesign --verify --strict --verbose=2 "$binary"

mkdir -p -- "$output"
version=$(basename "$root")
dmg="$output/$version.dmg"
if [[ -e "$dmg" || -e "$dmg.sha256" ]]; then
	printf 'error: signed distribution output already exists: %s\n' "$dmg" >&2
	exit 1
fi
hdiutil create -quiet -fs HFS+ -format UDZO -volname "$version" -srcfolder "$root" "$dmg"
codesign --force --timestamp --sign "$identity" "$dmg"
codesign --verify --strict --verbose=2 "$dmg"
xcrun notarytool submit "$dmg" --keychain-profile "$profile" --wait
xcrun stapler staple "$dmg"
xcrun stapler validate "$dmg"
codesign --verify --strict --verbose=2 "$dmg"
spctl --assess --type open --context context:primary-signature --verbose=2 "$dmg"
dmg_sha256=$(shasum -a 256 "$dmg" | awk '{print $1}')
printf '%s  %s\n' "$dmg_sha256" "$(basename "$dmg")" >"$dmg.sha256"
chmod 0444 "$dmg" "$dmg.sha256"
printf 'Signed, notarized, and stapled distribution: %s\n' "$dmg"
