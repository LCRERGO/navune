# ADR 0010 — Internal-only graph for metrics and gates

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q14)

## Context

A Go file importing `net/http` or a TS file importing `react` creates a
dependency edge. Whether external edges participate in cycle detection and
coupling metrics changes their meaning: "cycles" through external libraries are
meaningless (they're not part of the analyzed codebase), and Ce-to-externals is
a different, noisier signal than internal coupling.

## Decision

- **Internal-only graph** is used for cycle detection, Ca/Ce, instability,
  abstractness, budgets, and the composite.
- **External imports are recorded** (per file and module) and appear in the
  JSON/Mermaid report as fan-out information for humans, but never pollute
  computed metrics or gate math.

## Consequences

- Coupling and instability numbers are comparable across codebases regardless
  of how many libraries a project uses.
- Report consumers can still see external fan-out ("31 files depend on 14
  external libs from this module") as a separate signal.
- The graph builder must distinguish internal vs. external resolution per
  language (Go: within module; TS: resolved within analysis root).
