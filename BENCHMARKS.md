# goddddocr Benchmarks

This file records local benchmark baselines for tuning the OCR service. Numbers
are hardware, OS, image, and concurrency dependent; use them as a regression
reference, not a universal throughput claim.

## Worker Pool Baseline

Date: 2026-05-16

Host:

```text
Darwin TenSafeMacBook-Pro-2.local 25.3.0 Darwin Kernel Version 25.3.0: Wed Jan 28 20:53:15 PST 2026; root:xnu-12377.81.4~5/RELEASE_ARM64_T6000 arm64
```

Command:

```bash
GODDDDOCR_BENCH_OUT=/tmp/goddddocr-baseline-20260516 scripts/bench_workers.sh
```

Inputs:

- image: `samples/yzm1.png`
- expected text: `3n3d`
- requests per worker setting: `100`
- concurrency: `4`
- model: `old`
- ONNX Runtime: local darwin/arm64 runtime

Results:

| workers | qps | p50 ms | p95 ms | p99 ms | errors | mismatches | rss mb |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 48.94 | 76.26 | 106.19 | 189.00 | 0 | 0 | 116.30 |
| 2 | 118.47 | 32.22 | 42.91 | 47.94 | 0 | 0 | 135.27 |
| 4 | 112.55 | 31.49 | 50.41 | 87.43 | 0 | 0 | 181.06 |
| 8 | 108.87 | 33.82 | 52.95 | 65.52 | 0 | 0 | 250.05 |

Notes:

- `workers=2` is the current best default candidate on this machine for the
  bundled sample at concurrency 4.
- Increasing from 2 to 4 or 8 workers did not improve throughput in this run and
  raised RSS memory substantially.
- Keep `workers=1` as the conservative default for small deployments, then tune
  with `scripts/bench_workers.sh` on the target host.
