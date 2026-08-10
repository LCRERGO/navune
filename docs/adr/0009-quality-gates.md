# ADR 0009 — Quality gates: budgets + transparent composite

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q8, Q12)

## Context

A metrics tool needs to answer "is this codebase healthy?" without degenerating
into numerology. Two existing models: per-metric thresholds that fail CI (a
quality gate) plus a 0–100 rating; and industry composite formulas like
Maintainability Index or SQALE, which are arbitrary legacy formulas that are
hard to reproduce faithfully.

## Decision

Quality is judged by **budgets first, composite second**:

- **Budgets** live in a config file (`navune.yaml`): per-metric limits such as
  max average complexity, max worst-case complexity, max % duplication, max SCC
  size, max % files in cycles, etc. A breach produces a nonzero exit code
  (quality-gate style). Sensible defaults ship embedded in the binary;
  `navune init` writes them out for editing.
- **Composite index (0–100)** is a **Navune-transparent blend**: each metric
  contributes 0–100 = how far from *its configured budget* it is, combined with
  documented, config-file weights. The exact formula is documented in the README
  so any number is traceable to a config line.

Industry formulas (MI, SQALE) are explicitly rejected: defensible beats
familiar, and a transparent config-derived score survives a skeptical
engineer's review.

## Consequences

- Verdict and score are both derived from the same budgets — no invented second
  scale.
- Composite semantics are fully documented and reproducible.
- CI integration is a single nonzero exit code; severity tiers (error/warn/info)
  are configurable per metric.
