<!-- SPDX-License-Identifier: AGPL-3.0-only -->

# RC1 renderer measurements

These are historical, maintainer-run measurements of the **RC1 runtime**, not
new measurements of a final v1 artifact and not an independent performance audit.
They were collected on August 24, 2026 from public commit
`e730dd1b56061501881818c7000364a55cd49e35`, tree
`931023c7d42591b5280ff81d64ce86d2ebba85b7`.

## What was measured

The committed `sando/benchmark_test.go` corpus renders one synthetic typed view
to `io.Discard` during timing. `TestBenchmarkCorpusEquivalent` first requires
identical buffered output.
`BenchmarkV1CorpusSandwichHime` mirrors generated runtime writer calls;
`BenchmarkV1CorpusHTMLTemplate` executes a pre-parsed Go standard-library template.
Both include the measured render allocations. Template parsing is not charged to
the baseline. Compiler execution, HTTP, routing, databases and deployment are
outside this measurement.

Each renderer has ten samples, with two seconds requested per sample. The result
is the median of the ten `ns/op` values, not the fastest sample. There was no
separately recorded application warmup phase; Go's benchmark harness performs
its normal iteration calibration. See the [predeclared policy](BENCHMARKS.md).

| Host | Go | Hime median ns/op | html/template median ns/op | Hime/baseline time |
| --- | --- | ---: | ---: | ---: |
| Apple M1, Darwin/arm64, 8 logical CPUs | 1.26.7 | 1,161.0 | 4,958.0 | 23.4% |
| Apple M1, Darwin/arm64, 8 logical CPUs | 1.27.0 | 1,149.5 | 4,800.5 | 23.9% |
| Xeon E5-2620 v2, Linux/amd64, 4-CPU container limit | 1.26.7 | 4,362.5 | 22,837.5 | 19.1% |
| Xeon E5-2620 v2, Linux/amd64, 4-CPU container limit | 1.27.0 | 3,877.5 | 22,003.5 | 17.6% |

All samples recorded **688 B/op and 21 allocations/op** for Sandwich Hime,
versus **1,600 B/op and 58 allocations/op** for `html/template`. Linux ran
natively in a read-only, network-disabled,
capability-dropped container limited to 4 GiB and 512 processes. macOS ran
natively. Exact OS patch/kernel versions were not captured in these benchmark
logs; do not infer them from the toolchain versions.

The baseline took approximately **4.2–5.7 times as long** on these lanes.
This does not establish whole-site speed, request throughput, a comparison
against every Go template engine, or a universal result on other workloads.
The samples are sequential, not a statistical study of concurrent application
load. No confidence interval or causal claim about the difference between the
two machines is implied. We do not claim to be the fastest engine.

### September 9 repeat check

A local repeat on Apple M1, Darwin/arm64, Go 1.27.0 used the same command and
runtime source unchanged from the public RC1 commit above. Output equivalence
passed. Median times were **1,034.5 ns/op**
for Hime and **4,044.0 ns/op** for the baseline (25.6%, approximately 3.9 times
as long for the baseline). Allocation counts and bytes remained 21/688 and
58/1,600 respectively. This is a local repeat, not a final-release artifact gate.

- Hime: `1028, 1021, 1028, 1031, 1033, 1097, 1063, 1065, 1041, 1036`
- Baseline: `4024, 4218, 4053, 4035, 4156, 4098, 4022, 3993, 4133, 3980`

The changed timing ratio is a useful reminder to rerun the workload on the
deployment hardware rather than treating a historical speedup as a guarantee.

## Reproduce

Check out the public commit above, select one of the recorded Go versions and,
from its `sando` directory, run:

```sh
GOWORK=off go test -run '^TestBenchmarkCorpusEquivalent$' \
  -bench '^BenchmarkV1Corpus' -benchmem -benchtime=2s -count=10
```

Use `GOMAXPROCS=8` for the recorded Darwin lane or `GOMAXPROCS=4` for the Linux
lane, and document the actual hardware/resource limits. Compare within a host;
do not treat absolute times from different hardware as a controlled comparison.

## Recorded samples

Values below are `ns/op` in original sample order. Each vector contains ten
measurements. Raw-log digests identify the retained original evidence; these
vectors expose the measurements without publishing private runner metadata.

### Darwin/arm64, Go 1.26.7

- Hime: `1168, 1163, 1159, 1159, 1164, 1171, 1159, 1160, 1159, 1162`
- Baseline: `4771, 4802, 5117, 4846, 4885, 4928, 5017, 4988, 5036, 5105`
- Raw-log SHA-256: `7b68a643d685168dbd77cf3df617e8de59aa8063f6728de6c2d664173b0b05bd`

### Darwin/arm64, Go 1.27.0

- Hime: `1102, 1107, 1120, 1125, 1144, 1155, 1167, 1169, 1167, 1180`
- Baseline: `4686, 4924, 5132, 4750, 4822, 4741, 4791, 4787, 4810, 4822`
- Raw-log SHA-256: `b85481317da41bc63ad8afc317da2fd1075184405e7704a69714f42453d7dd4c`

### Linux/amd64, Go 1.26.7

- Hime: `4291, 4496, 4112, 4357, 4400, 3984, 4368, 4511, 4145, 4375`
- Baseline: `23663, 22300, 22364, 22946, 22863, 22812, 22078, 24476, 23041, 20900`
- Raw-log SHA-256: `277eee8f2488b0aa0b752584a46d3737df87defd311fd0bcf722aea81612a960`

### Linux/amd64, Go 1.27.0

- Hime: `4314, 3939, 3950, 3861, 3797, 3821, 3756, 3651, 3894, 4170`
- Baseline: `21346, 21015, 22272, 22066, 22198, 21059, 22473, 21318, 21941, 22447`
- Raw-log SHA-256: `a5d760c0fb76012b5923376dfcd80a1f1f44bf0043145f4352e4ec820aa475b3`
