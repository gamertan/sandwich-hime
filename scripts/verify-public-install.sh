#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

usage() {
	cat <<'EOF'
Usage: scripts/verify-public-install.sh --version vX.Y.Z

Post-publication verification for gamertan.com vanity metadata and the exact
documented install commands. It uses fresh temporary Go caches and never writes
to the repository. Signed compiler and sando tags must already be public.
EOF
}

version=''
while (( $# > 0 )); do
	case "$1" in
		--version)
			[[ $# -ge 2 ]] || { usage >&2; exit 2; }
			version=$2
			shift 2
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
	printf 'error: --version must identify a signed release tag, not a Go pseudo-version\n' >&2
	exit 2
fi
if [[ -n "$prerelease" ]]; then
	IFS=. read -r -a prerelease_identifiers <<<"$prerelease"
	for identifier in "${prerelease_identifiers[@]}"; do
		if [[ "$identifier" =~ ^0[0-9]+$ ]]; then
			printf 'error: numeric prerelease identifiers must not contain leading zeroes: %s\n' "$identifier" >&2
			exit 2
		fi
	done
fi

for command_name in curl go git false; do
	command -v "$command_name" >/dev/null 2>&1 || {
		printf 'error: required command is unavailable: %s\n' "$command_name" >&2
		exit 1
	}
done
false_command=$(command -v false)

public_origin=${HIMESAN_PUBLIC_ORIGIN:-https://gamertan.com}
public_origin=${public_origin%/}
case "$public_origin" in
	https://*) ;;
	*)
		printf 'error: HIMESAN_PUBLIC_ORIGIN must use HTTPS\n' >&2
		exit 2
		;;
esac

compiler_meta='<meta name="go-import" content="gamertan.com/sandwich-hime git https://gitea.speelman.ca/gamertan/sandwich-hime.git">'
runtime_meta='<meta name="go-import" content="gamertan.com/sandwich-hime/sando git https://gitea.speelman.ca/gamertan/sandwich-hime.git sando">'

check_metadata() {
	local path=$1
	local expected=$2
	local body
	body=$(curl --fail --silent --show-error --location \
		--proto '=https' --max-redirs 3 --connect-timeout 10 --max-time 30 \
		"$public_origin$path")
	if [[ "$body" != *"$expected"* ]]; then
		printf 'error: expected vanity metadata missing at %s%s\n' "$public_origin" "$path" >&2
		exit 1
	fi
}

printf '==> exact vanity-import discovery routes\n'
check_metadata '/sandwich-hime?go-get=1' "$compiler_meta"
check_metadata '/sandwich-hime/cmd/himesan?go-get=1' "$compiler_meta"
check_metadata '/sandwich-hime/sando?go-get=1' "$runtime_meta"
check_metadata '/sandwich-hime/sando/future-package?go-get=1' "$runtime_meta"

browser_status=$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' \
	--proto '=https' --max-redirs 0 --connect-timeout 10 --max-time 30 \
	"$public_origin/sandwich-hime/not-a-browser-route")
if [[ "$browser_status" != 404 ]]; then
	printf 'error: query-scoped metadata fallback leaked into ordinary browser routing (status %s)\n' "$browser_status" >&2
	exit 1
fi

scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/himesan-public-install.XXXXXXXX")
cleanup() {
	if [[ -n "${scratch_dir:-}" && -d "$scratch_dir" ]]; then
		chmod -R u+w -- "$scratch_dir" 2>/dev/null || true
		rm -rf -- "$scratch_dir"
	fi
}
trap cleanup EXIT HUP INT TERM

run_install_pair() {
	local mode=$1
	local proxy=$2
	local no_sum_db=$3
	local mode_dir="$scratch_dir/$mode"
	local installed_binary installed_version go_executable_suffix
	mkdir -p "$mode_dir/gopath" "$mode_dir/modcache" "$mode_dir/buildcache" "$mode_dir/consumer"

	printf '\n==> %s clean-cache runtime then compiler install\n' "$mode"
	(
		cd "$mode_dir/consumer"
		go mod init example.invalid/himesan-public-install >/dev/null
		env \
			GIT_TERMINAL_PROMPT=0 \
			GIT_CONFIG_NOSYSTEM=1 \
			GIT_CONFIG_GLOBAL=/dev/null \
			GIT_ASKPASS="$false_command" \
			SSH_ASKPASS="$false_command" \
			GOPATH="$mode_dir/gopath" \
			GOMODCACHE="$mode_dir/modcache" \
			GOCACHE="$mode_dir/buildcache" \
			GOPROXY="$proxy" \
			GOPRIVATE= \
			GONOPROXY=none \
			GONOSUMDB="$no_sum_db" \
			GOSUMDB=sum.golang.org \
			GOINSECURE= \
			GOAUTH=off \
			go get "gamertan.com/sandwich-hime/sando@$version"
	)

	env \
		GIT_TERMINAL_PROMPT=0 \
		GIT_CONFIG_NOSYSTEM=1 \
		GIT_CONFIG_GLOBAL=/dev/null \
		GIT_ASKPASS="$false_command" \
		SSH_ASKPASS="$false_command" \
		GOPATH="$mode_dir/gopath" \
		GOMODCACHE="$mode_dir/modcache" \
		GOCACHE="$mode_dir/buildcache" \
		GOPROXY="$proxy" \
		GOPRIVATE= \
		GONOPROXY=none \
		GONOSUMDB="$no_sum_db" \
		GOSUMDB=sum.golang.org \
		GOINSECURE= \
		GOAUTH=off \
		go install "gamertan.com/sandwich-hime/cmd/himesan@$version"

	go_executable_suffix=$(go env GOEXE)
	installed_binary="$mode_dir/gopath/bin/himesan$go_executable_suffix"
	installed_version=$("$installed_binary" version --json)
	if [[ "$installed_version" != *"\"compiler\":\"$version\""* ]]; then
		printf 'error: installed compiler did not report module version %s: %s\n' "$version" "$installed_version" >&2
		exit 1
	fi
	cat >"$mode_dir/version_probe.sando" <<'EOF'
<?sando go
package probe
func VersionProbe()
?>
<p>version probe</p>
EOF
	"$installed_binary" generate "$mode_dir/version_probe.sando" >/dev/null
	if ! grep -Fq "// himesan:compiler $version" "$mode_dir/version_probe.sando.go"; then
		printf 'error: generated provenance did not record installed compiler version %s\n' "$version" >&2
		exit 1
	fi
}

run_install_pair direct direct gamertan.com/sandwich-hime
run_install_pair public-proxy 'https://proxy.golang.org' ''

printf '\nPublic vanity metadata and exact install commands passed for %s.\n' "$version"
