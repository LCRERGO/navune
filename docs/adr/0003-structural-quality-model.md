# ADR 0003 — Structural quality model

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q3)

## Context

Code-quality tooling splits along two very different axes:

- **Bug/security hunting**: bug detection, security hotspots, code smells, test
  coverage, quality gates. Typically shallow structural analysis across many
  languages. Replicating deep bug/security intelligence from scratch is a
  multi-year effort and largely duplicates `go vet`, `staticcheck`, and
  `govulncheck` for Go.
- **Architecture and dependency analysis**: dependency graphs, cyclic
  dependencies, layering violations, Martin's coupling metrics (Ca/Ce,
  instability, abstractness), quality budgets. Deep structural insight, not bug
  hunting.

## Decision

Navune's North Star is **AST-driven structural analysis**: dependency graphs,
cycles, coupling, complexity, duplication — plus a transparent *composite
quality model* layered on top.

Bug hunting and security scanning are explicitly **not** Navune's job; those
are the domain of language-native linters and scanners, which Navune does not
attempt to replace.

## Consequences

- All analysis is AST-driven structural measurement — well-defined, testable,
  and defensible without a hand-written bug database.
- We must be careful never to describe Navune as a "linter" or "security
  scanner" in docs and README.
- Complexity, duplication, size, coupling, and cycle metrics are the core
  deliverables (see ADR 0005).
