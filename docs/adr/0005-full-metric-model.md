# ADR 0005 — Full metric model in v1 (including duplication)

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q6)

## Context

Structural quality analysis spans several metric families. There is a
temptation to defer duplication detection, which is a whole algorithm of its
own (token normalization, fingerprinting, minimum-block thresholds).

## Decision

v1 ships the **full metric set** for the Go language (see ADR 0002/0007 for
the language staging):

- **Size** — physical and logical source lines (SLOC/NCLOC) per file and module.
- **Complexity** — cyclomatic complexity per function, rolled up to worst-case
  and average per file/module.
- **Dependency graph** — internal module-to-module edges derived from imports.
- **Cycles** — strongly connected components (structural debt).
- **Coupling** (Martin) — Ca, Ce, instability I = Ce/(Ca+Ce), abstractness A,
  and distance from the main sequence |A + I − 1|, computed per file.
- **Duplication** — language-aware token normalization + sliding-window
  fingerprint hashing; cross-file and intra-file, with minimum-length
  thresholds to suppress trivial boilerplate.

## Consequences

- All families exist in the v1 pipeline and composite, giving the quality gate
  real meaning rather than a single-metric hack.
- Duplication detection must be language-aware but is backend-agnostic: it
  operates on a token stream supplied by the language adapter.
- Larger surface area to test than a subset; golden fixtures cover each family
  (see ADR 0013).
