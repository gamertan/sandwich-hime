<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# Benchmark policy

Benchmarks compare equivalent typed views and output against Go's `html/template` baseline. Reports include hardware, operating system, Go version, repository commit, dataset identity, exact commands, warmup/run counts, `ns/op`, bytes and allocations per operation, end-to-end response latency where relevant, output size, and statistical method.

The v1 gate is no material regression for the selected EQL pages under the published method. Only reproduced improvements become marketing claims. Microbenchmarks do not justify claims about request throughput, database-heavy pages, or whole-application latency.

Benchmark fixtures must contain synthetic or approved public data. The EQL production database is never copied into this repository or a public artifact.
