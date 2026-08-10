# ADR 0004 — Audience: a defensible tool for others

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q4)

## Context

A quality-metrics tool is only credible if its numbers are. Two possible bars:
a quick personal project, or a tool strangers can trust in CI to fail builds
on their code. The second bar constrains everything downstream — metric
definitions, portability, documentation, and contracts.

## Decision

Navune is engineered **as a product others could adopt**:

- Metrics must be defined precisely enough to survive a skeptical engineer's
  review; every number must be traceable to a documented formula.
- A stable, documented JSON output contract is a first-class deliverable.
- The tool builds and runs portably; cgo is isolated behind build tags so the
  core (Go + TS/JS) ships as a pure-Go, statically linkable binary.
- Golden tests and documentation are part of the definition of done, not
  afterthoughts (see ADR 0013).

## Consequences

- Higher engineering bar on every feature.
- Cross-compilation and static builds are supported constraints (feeds ADR 0007).
- Time to first usable release is longer than a scratch project would take —
  deliberately traded against trustworthiness.
