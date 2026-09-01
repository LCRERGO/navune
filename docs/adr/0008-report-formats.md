# ADR 0008 — Text + JSON + Mermaid graph export

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q7)

## Context

A structural-quality tool's value proposition is visual — you *see* the cycle
in a diagram. A CLI has no window. The audience constraint (ADR 0004) demands a
stable machine contract; the structural-quality ambition (ADR 0003) demands the
cycles be visible somehow.

## Decision

`navune analyze` emits three output forms via a `--format` flag:

- **text** — human-readable summary tables (default).
- **json** — a stable, versioned, documented schema; the machine contract that
  CI and other tools consume.
- **mermaid** — a `flowchart` graph (and per-SCC subgraphs) written to stdout or
  `--out`, so users can render dependency cycles in their editor/tooling.

A self-contained HTML report is explicitly deferred to a later milestone — it
is its own maintenance surface and not needed to satisfy the "see the cycle"
promise.

## Consequences

- Mermaid generator is a small, dependency-free string emitter (~tens of lines).
- JSON schema must be versioned and frozen before v1 ships; it is the primary
  integration contract.
- Graph export (DOT/Mermaid) derives directly from the internal dependency
  graph, so it is cheap to add once the graph exists.
