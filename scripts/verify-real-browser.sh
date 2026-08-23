#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

chrome=${HIMESAN_CHROME:-}
if [[ -z "$chrome" ]]; then
	case "$(uname -s)" in
	Darwin) chrome='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' ;;
	Linux)
		for candidate in google-chrome-stable google-chrome chromium chromium-browser; do
			if command -v "$candidate" >/dev/null 2>&1; then
				chrome=$(command -v "$candidate")
				break
			fi
		done
		;;
	esac
fi
if [[ -z "$chrome" || ! -f "$chrome" || ! -x "$chrome" ]]; then
	printf 'error: set HIMESAN_CHROME to a reviewed Chrome or Chromium executable\n' >&2
	exit 1
fi

printf 'Real-browser executable: '
"$chrome" --version
printf 'Go toolchain: '
go version
HIMESAN_CHROME="$chrome" go test -count=1 -tags=himesan_browser_evidence \
	./internal/devserver -run '^TestRealBrowserDevelopmentClient$' -v
go test -count=1 ./internal/devserver \
	-run '^(TestSupervisorBuildsSwapsAndCleansUp|TestSupervisorClearsTargetWhenCurrentApplicationExits)$' -v
