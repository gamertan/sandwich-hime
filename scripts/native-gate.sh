#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only

set -euo pipefail

repo_root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

usage() {
	printf 'Usage: scripts/native-gate.sh --repository OWNER/REPOSITORY --go-version goX.Y.Z --runner-version VERSION --runner-name NAME --output DIR\n' >&2
}

repository=''
expected_go=''
runner_version=''
runner_name=''
output=''
while (( $# > 0 )); do
	case "$1" in
	--repository) repository=$2; shift 2 ;;
	--go-version) expected_go=$2; shift 2 ;;
	--runner-version) runner_version=$2; shift 2 ;;
	--runner-name) runner_name=$2; shift 2 ;;
	--output) output=$2; shift 2 ;;
	*) usage; exit 2 ;;
	esac
done
if [[ ! "$repository" =~ ^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$ || -z "$expected_go" || -z "$runner_version" || -z "$runner_name" || -z "$output" ]]; then
	usage
	exit 2
fi
if [[ "$(go env GOVERSION)" != "$expected_go" ]]; then
	printf 'error: expected %s, found %s\n' "$expected_go" "$(go env GOVERSION)" >&2
	exit 1
fi

mkdir -p -- "$output"
./scripts/check-licenses.sh
./scripts/test-public-snapshot.sh
HIMESAN_RACE=1 ./scripts/verify.sh
go test ./internal/compiler -run '^$' -fuzz '^FuzzCompileNeverPanics$' -fuzztime=15s -parallel=1
go test ./internal/compiler -run '^$' -fuzz '^FuzzGoDelimiterNeverPanics$' -fuzztime=15s -parallel=1
go test ./internal/lsp -run '^$' -fuzz '^FuzzFrameReaderNeverPanics$' -fuzztime=15s -parallel=1
go test ./internal/lsp -run '^$' -fuzz '^FuzzDocumentPositionNeverPanics$' -fuzztime=15s -parallel=1
(
	cd sando
	go test -run '^$' -fuzz '^FuzzWriteURLPolicy$' -fuzztime=15s -parallel=1
)
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
(
	cd sando
	go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
)
./scripts/verify-consumer.sh

./scripts/package-native.sh --version v0.0.0-verification.1 --output "$output"
artifact=$(find "$output" -maxdepth 1 -type f -name '*.tar.gz' -print -quit)
artifact_sha=$(awk '{print $1}' "$artifact.sha256")
completed_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
go run ./cmd/himesan-release receipt \
	--output "$output/TEND-CI-VERIFICATION.json" \
	--repository "$repository" \
	--commit "$(git rev-parse HEAD)" \
	--tree "$(git rev-parse 'HEAD^{tree}')" \
	--goos "$(go env GOOS)" --goarch "$(go env GOARCH)" \
	--go-version "$expected_go" --runner-version "$runner_version" \
	--runner-name "$runner_name" --artifact-sha256 "$artifact_sha" \
	--completed-at "$completed_at" \
	--gates test,vet,build,race,generation,contracts,public-snapshot,fuzz,vulnerability,consumer,package \
	--generated-files internal/compiler/testdata/golden/basic.sando.go
printf 'Native verification evidence: %s\n' "$output"
