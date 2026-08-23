#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

if [[ "$(uname -s)/$(uname -m)" != Darwin/arm64 ]]; then
	printf 'error: the maintained macOS artifact must be built natively on darwin/arm64\n' >&2
	exit 1
fi
exec "$(dirname -- "${BASH_SOURCE[0]}")/package-native.sh" "$@"
