<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Benchmark policy

Benchmarks compare equivalent typed views and output against Go's `html/template` baseline. Reports include hardware, operating system, Go version, repository commit, dataset identity, exact commands, warmup/run counts, `ns/op`, bytes and allocations per operation, end-to-end response latency where relevant, output size, and statistical method.

The v1 gate is no material regression against equivalent repository-owned
synthetic cases under the published method. Only reproduced improvements become
marketing claims. Microbenchmarks do not justify claims about request
throughput, database-heavy pages, or whole-application latency.

The threshold was fixed before measuring the RC. On each maintained native
platform and toolchain, ten benchmark samples use the exact output-equivalent
`BenchmarkV1Corpus*` pair. Sandwich Hime passes when its median `ns/op` and
`B/op` are each no more than 125% of `html/template`, and its median
allocations/op are no more than two allocations above `html/template`.
Any failed platform/toolchain pair is a material regression. Timing is reviewed
from raw samples rather than enforced in ordinary CI, where host contention
would turn a performance policy into a flaky correctness gate.

Run:

```sh
cd sando
go test -run '^TestBenchmarkCorpusEquivalent$' \
  -bench '^BenchmarkV1Corpus' -benchmem -benchtime=2s -count=10
```

`benchmarkSandoComponent` intentionally mirrors generated writer calls and
captures the same typed view used by the parsed standard template. This is a
runtime renderer microbenchmark; it excludes parsing, compiler execution,
HTTP, routing, logging, databases, and deployment.

Benchmark fixtures must be self-contained, synthetic, reviewable, and committed
to this repository. Application-specific datasets and deployment measurements
belong with their applications and are neither copied here nor treated as core
release gates.
