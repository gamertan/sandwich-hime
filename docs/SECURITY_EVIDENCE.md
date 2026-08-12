<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Security evidence ledger

This ledger records what was actually inspected and executed. It is a
maintainer-led self-assessment, not an independent audit, certification, formal
verification, or guarantee that no vulnerability exists.

## Assessment identity

| Field | Value |
| --- | --- |
| Assessment date | 2026-08-12 |
| Public evidence identity | Exact file checksums in the co-published `PUBLIC-SNAPSHOT.sha256`; private/public commit mapping is retained only in the non-exported operational ledger |
| Assessment phases | Clean pre-remediation source followed by clean remediated source |
| Primary environment | Linux amd64 under WSL, Go 1.26.5 |
| Declared minimum Go | Go 1.25 |
| Assessor | Project maintainer with AI-assisted code review; human responsibility retained |

Security remediation discovered during this assessment was committed and the
named checks were rerun from a clean source state. Before this ledger can support
a versioned release, the complete campaign must be rerun from the exact
sanitized public release commit. Private-to-public commit mappings are retained
outside the exported source rather than being disclosed here.

## Observed evidence

| Property examined | Enforcement or test surface | Result observed on 2026-08-12 |
| --- | --- | --- |
| Root correctness | `go test -count=1 ./...` | Pass |
| Concurrent access | `go test -race -count=1 ./...` | Pass |
| Runtime concurrency | `(cd sando && go test -race -count=1 ./...)` | Pass |
| Standard static analysis | `go vet ./...` and runtime equivalent | Pass |
| Reachable known vulnerabilities | `govulncheck@v1.6.0` on both modules | No vulnerabilities found on 2026-08-12 |
| Dependency surface | `go list -m -json all` in both modules | Zero third-party module requirements |
| Statement coverage | Go cover profiles on remediated source | compiler 75.2%; devserver 80.2%; runtime 96.6% |
| Parser robustness smoke | Two bounded Go fuzz targets | Pass; no panic found |
| Deterministic generation | repeated generate/check/hash/mtime gates | Pass |
| Writer failures | runtime error/short-write/nil-writer tests | Pass |
| HTML text/attribute/RCDATA escaping | compiler and runtime adversarial cases | Pass for enumerated cases |
| URL scheme handling | ordinary/trusted URL test matrix | Pass for enumerated cases |
| Filesystem boundaries | symlink, nested-module, VCS, ownership, stale-output tests | Pass for tested cases; see open findings |
| Development proxy browser boundary | Host, Origin, Fetch Metadata, CSP, fragment and response tests | Pass for tested cases |
| Native platform behavior | Linux execution; Windows/macOS cross-compilation | Native Windows/macOS execution not yet evidenced |

Coverage measures statements executed by tests. It is not branch completeness
and is not evidence that the executed behavior is secure.

`govulncheck` reports vulnerabilities known to the Go vulnerability database
and reachable through its analysis. A clean result cannot detect unknown flaws,
design errors, or vulnerabilities outside its model.

## Security-relevant design evidence

### Production boundary

The production `sando` module contains rendering contracts and contextual write
helpers. It contains no HTTP server, router, middleware, template discovery,
development proxy, plugin loader, or production process manager. Both compiler
and runtime modules currently have no third-party Go module requirements.

### Compiler behavior

Compilation builds and formats outputs in memory before generation writes.
Recursive discovery rejects or skips observed symlinks, nested modules, VCS
trees, vendor trees, and detected filesystem crossings. Existing non-owned,
symlink, and non-regular output files are rejected. Each changed file uses an
atomic replacement primitive; the whole set is not a filesystem transaction if
a later replacement fails.

`generate` and `check` do not run project code, invoke the Go toolchain, fetch
dependencies, or edit module metadata. `himesan dev` is intentionally separate:
it builds and executes trusted project code and may fetch modules under the
user's normal Go configuration.

### Output contexts

The compiler accepts dynamic values only in its enumerated contexts. It rejects
dynamic markup construction, unquoted attributes, event-handler values,
dynamic style attributes, unsupported URL lists, foreign content, meta refresh,
malformed tags, and unbalanced generated components. Runtime helpers escape
ordinary text/attributes, validate ordinary whole-URL schemes before writing,
and escape every trusted wrapper in RCDATA.

Script and style output require opaque trusted types. Those types deliberately
move responsibility to trusted application code; they are not sanitizers.

## Reproduction commands

Run from a clean canonical checkout. Networked scans contact the Go module proxy
and vulnerability database.

```sh
go version
git status --short
git rev-parse HEAD^{commit} HEAD^{tree}

./scripts/check-licenses.sh
HIMESAN_RACE=1 ./scripts/verify.sh

go test -count=1 -cover ./...
(cd sando && go test -count=1 -cover ./...)

go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
(cd sando && go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...)

go test ./internal/compiler -run '^$' \
  -fuzz '^FuzzCompileNeverPanics$' -fuzztime=20s
go test ./internal/compiler -run '^$' \
  -fuzz '^FuzzGoDelimiterNeverPanics$' -fuzztime=20s
```

The fuzz targets currently assert process robustness and result bounds. They do
not yet prove semantic HTML safety.

## Assessment findings and remediation status

The 2026-08-12 assessment identified six concrete gaps. Their current
working-tree status is recorded here without rewriting the original baseline:

| Finding | Current remediation | Executable evidence |
| --- | --- | --- |
| Generated code named an ABI but did not enforce the exact contract | Generated code now requires the version-specific `sando.ABISandoV1` symbol | `TestGeneratedCodeRequiresVersionedRuntimeABIMarker`; `TestRuntimeABIMarker` |
| Deleted or renamed sources could leave owned `.sando.go` orphans invisible to directory-level `check` | Directory discovery now reports owned outputs whose adjacent source is absent or non-regular and blocks the operation before writes | `TestDirectoryOperationsRejectOrphanedOwnedOutputBeforeWrites` |
| Component-context prose included arbitrary handwritten implementations in the generated-component guarantee | Runtime documentation, policy, and threat model now classify handwritten components as trusted output capabilities | API documentation plus policy review; generated-balance tests retain their narrower scope |
| An exited development child left its former upstream selected | Exit notification now clears only the matching active target immediately, independent of the watcher poll interval | `TestClearTargetOnlyClearsSelectedUpstream`; `TestSupervisorClearsTargetWhenCurrentApplicationExits` |
| Trusted-value warnings were described more broadly than their analysis supports | Policy and threat-model copy now call them best-effort lexical audit hints rather than type or taint analysis | Documentation assertion and review |
| Public copy implied a completed systematic `html/template` differential campaign | Policy and public security copy now describe fixed adversarial cases and list systematic differential work as open | Documentation assertion and review |

The remediated clean source passed the race-enabled repository verifier,
sanitized-snapshot tests, both bounded fuzz-smoke targets, compiler/runtime
known-vulnerability scans, and Windows/macOS cross-compilation on 2026-08-12.
Those results do not become release evidence until the changes are committed,
exported to the sanitized canonical public tree, and re-run from that exact
public commit. Native Windows/macOS execution and the other gaps below remain
separate release decisions.

## Open assurance gaps

- confidential mailbox delivery and response/recovery procedure must be tested;
- release signing, checksum, SBOM, and provenance rehearsal is incomplete;
- native Windows/macOS execution remains outstanding;
- browser-parser differential and semantic property testing need expansion;
- compiler input size, CPU, and memory have no built-in hard budget;
- filesystem checks do not defend against a hostile local actor racing path
  components between inspection and use;
- the watcher is a convenience mechanism, not a filesystem-integrity monitor;
- human-readable diagnostics can include hostile local filenames or child-tool
  output and should not be treated as a sanitized log protocol;
- development CSP rewriting is convenience, not production CSP validation;
- deliberately detached child descendants may evade process-tree cleanup;
- rendering has no built-in recursion, output-size, allocation, CPU, panic, or
  deadline enforcement;
- static cycle detection and trust-use warnings are best-effort analyses; and
- the project has no independent security audit or bug-bounty program.

## Interpreting this ledger

“Pass” means the named command or case produced its expected result in the named
environment on the assessment date. It does not mean “secure.” Confidence comes
from keeping the boundary small, making risky capabilities explicit, preserving
ordinary generated Go for review, publishing reproducible tests, recording
failures, and correcting claims when evidence is weaker than the prose.
