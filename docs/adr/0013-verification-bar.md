# ADR 0013 — Verification: golden fixtures + self-analysis + perf contract

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q16)

## Context

A metrics tool's correctness is its whole product (ADR 0004). Numbers that are
wrong undermine trust faster than numbers that are absent. Full OSS-repo
validation corpora drag in licensing and network concerns.

## Decision

v1 verification bar:

- **Committed synthetic golden fixtures** for Go covering every metric: cycle
  cases, coupling triangles, complexity ladders, duplication hits and
  near-misses, test/generated-file classification. Each fixture has a committed
  expected report.
- **Self-analysis smoke test**: Navune analyzes its own source tree on every CI
  run as an always-on real-world exercise of the full pipeline.
- **Performance as a CI-benchmarked contract**: comfortably under ~30 s for a
  mid-size repo (roughly 100–500k LOC), analysis parallelized across files.
  Budget regressions fail CI.

External OSS-repo corpus validation is explicitly deferred (licensing/network
drag not worth it while the math is exercised synthetically and against Navune
itself).

## Consequences

- Regression safety for every metric formula via golden files.
- Performance is a tracked contract, not a hope.
- Fixture growth is cheap because fixtures are small and synthetic.
