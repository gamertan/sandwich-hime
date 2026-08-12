<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Security evidence ledger

This ledger records what was actually inspected and executed. It is a
maintainer-led self-assessment, not an independent audit, certification, formal
verification, or guarantee that no vulnerability exists.

## Assessment identity

| Field | Value |
| --- | --- |
| Assessment date | 2026-08-12 |
| Evidence sets | Clean security self-assessment plus an exact-commit pre-beta platform baseline; neither is evidence for the later Beta 1 candidate |
| Public commit | `113c95c21e57227b4675c9fda015ada59cc9e9a6` |
| Public tree | `a2aeb4dac22853cb3894e3e487b94bbeff5051e5` |
| Maintainer-run environments | Windows 11/amd64 on NTFS; Ubuntu 20.04/amd64 under WSL2 on ext4; Linux/amd64 server containers |
| Supported Go lanes exercised | Go 1.25.12 and Go 1.26.5 |
| Declared minimum Go | Go 1.25 |
| Assessor | Project maintainer with AI-assisted code review; human responsibility retained |

The named platform runs used the exact public commit and tree above. Hostnames,
network addresses, account names, private paths, private repository identities,
and private commit mappings are intentionally absent from this public ledger.

Beta 1 necessarily changes the tree through versioning, provenance,
documentation, or source fixes. Therefore this baseline cannot be relabeled as
Beta 1 evidence. The required Windows/Linux campaign must pass again on the
exact Beta 1 candidate before either tag is published. Native macOS execution
remains pending and is provisional for the beta.

## Observed security self-assessment evidence

These commands were observed on clean remediated source during the dated
assessment. Except where the exact-commit platform matrix below says otherwise,
the table does not claim that every command was rerun on the named public
baseline commit.

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
| Platform behavior | Native Windows and executed Linux matrices; macOS cross-compilation | Windows/Linux pass for tested lanes; native macOS pending |

Coverage measures statements executed by tests. It is not branch completeness
and is not evidence that the executed behavior is secure.

`govulncheck` reports vulnerabilities known to the Go vulnerability database
and reachable through its analysis. A clean result cannot detect unknown flaws,
design errors, or vulnerabilities outside its model.

## Pre-beta native compatibility matrix

These are maintainer-run, point-in-time results, not continuous CI and not an
independent audit.

| Environment | Go lanes | Commands and focused evidence | Result and limits |
| --- | --- | --- | --- |
| Windows 11/amd64, NTFS | 1.25.12, 1.26.5 | Native PowerShell verifier with race; root/runtime tests, vet, trimpath build, freshness, two generation passes, process-tree cleanup, watcher boundaries, and temporary consumer compilation | Pass. Symlink-output rejection skipped because the test account lacked symlink privilege; the read-only-directory case is POSIX-only |
| Ubuntu 20.04/amd64 under WSL2, native ext4 checkout | 1.25.12, 1.26.5 | Race-enabled verifier; root/runtime tests, vet, build, two generation passes, ten focused filesystem cases, five focused development-process/watcher cases, and license check | Pass. This is Linux execution under WSL2, not bare-metal or Linux/arm64 evidence |
| Linux/amd64 server containers | 1.25.12, 1.26.5 | Root/runtime tests, vet, builds, race, licensing, and deterministic generation in sequential isolated official Go containers | Pass. Container resources were capped at 1 CPU and 2 GiB; this is not Linux/arm64 evidence |
| macOS | — | Cross-compilation only | Native maintainer execution pending; provisional for Beta 1 |

The generated golden `basic.sando.go` was 1,399 bytes and had SHA-256
`63fa75a3049a3a8a12d769d7f9b6b510dfe763baacf706775b75cef2c57a984f`
on every tested Windows and Linux lane. Repeated generation also preserved its
timestamp. This demonstrates cross-host agreement for one compiler-owned
fixture, not equivalence for every possible template.

Mac learners and Go developers are warmly invited to report ordinary
compatibility results with macOS version, architecture, `go version`, exact
command, and a minimal reproduction. Suspected vulnerabilities use the private
route in [SECURITY.md](../SECURITY.md). Community reports help find gaps;
maintainers remain responsible for reproducing security-relevant behavior,
triage, remediation, and release decisions.

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

The 2026-08-12 assessment identified six concrete gaps. Their status in the
named public pre-beta baseline is recorded here:

| Finding | Current remediation | Executable evidence |
| --- | --- | --- |
| Generated code named an ABI but did not enforce the exact contract | Generated code now requires the version-specific `sando.ABISandoV1` symbol | `TestGeneratedCodeRequiresVersionedRuntimeABIMarker`; `TestRuntimeABIMarker` |
| Deleted or renamed sources could leave owned `.sando.go` orphans invisible to directory-level `check` | Directory discovery now reports owned outputs whose adjacent source is absent or non-regular and blocks the operation before writes | `TestDirectoryOperationsRejectOrphanedOwnedOutputBeforeWrites` |
| Component-context prose included arbitrary handwritten implementations in the generated-component guarantee | Runtime documentation, policy, and threat model now classify handwritten components as trusted output capabilities | API documentation plus policy review; generated-balance tests retain their narrower scope |
| An exited development child left its former upstream selected | Exit notification now clears only the matching active target immediately, independent of the watcher poll interval | `TestClearTargetOnlyClearsSelectedUpstream`; `TestSupervisorClearsTargetWhenCurrentApplicationExits` |
| Trusted-value warnings were described more broadly than their analysis supports | Policy and threat-model copy now call them best-effort lexical audit hints rather than type or taint analysis | Documentation assertion and review |
| Public copy implied a completed systematic `html/template` differential campaign | Policy and public security copy now describe fixed adversarial cases and list systematic differential work as open | Documentation assertion and review |

The clean remediated assessment source passed the race-enabled repository
verifier, sanitized-snapshot tests, both bounded fuzz-smoke targets,
compiler/runtime known-vulnerability scans, and Windows/macOS cross-compilation
on 2026-08-12. Separately, the exact public pre-beta commit passed the native
Windows and executed Linux matrices recorded above. These results still do not
become Beta 1 evidence: both sets of required checks must run on the exact
candidate after all candidate changes. Native macOS and the other gaps below
remain separate release decisions.

## Open assurance gaps

- the exact Beta 1 candidate Windows/Linux matrix and post-tag install checks
  must still run;
- confidential mailbox delivery and response/recovery procedure must be tested;
- SSH tag-signing rehearsal passed, but the candidate tags still require
  post-publication verification; prebuilt-artifact signing, checksums, SBOM,
  reproducible provenance, and key recovery remain incomplete;
- native macOS, Linux/arm64, and Windows/arm64 execution remain outstanding;
- Windows symlink rejection was not natively exercised because the test account
  lacked symlink privilege;
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
