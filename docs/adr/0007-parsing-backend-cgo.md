# ADR 0007 — Parsing backend: hybrid, minimal cgo

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q9)

## Context

Each target language has a different parser reality in Go:

| Language      | Best available backend                | Pure Go? |
|---------------|---------------------------------------|----------|
| Go            | `go/parser`                           | yes      |
| TS/JS         | tree-sitter                           | no (cgo) |
| Python        | tree-sitter                           | no (cgo) |
| C             | tree-sitter                           | no (cgo) |
| C++           | tree-sitter                           | no (cgo) |

cgo breaks clean cross-compilation and static builds and forces a C toolchain
onto users — a real cost for a distributable CLI (ADR 0004). No production-grade
pure-Go parsers exist for TypeScript/JavaScript, Python, C, or C++.

## Decision

Adopt a **hybrid backend with cgo confined to build tags**:

- Go analysis uses `go/parser`, which is pure Go and always compiled in.
- TS/JS, Python, C, and C++ use tree-sitter behind `cgo` build-tagged
  optional features. Without the build tag, the tool builds and runs anywhere
  with full Go support and the other languages degrade gracefully (their
  analysis reports as unavailable rather than wrong).

## Revision (2026-09-08)

The original decision named esbuild as the TS/JS parser on the strength of its
"pure Go" parser. That is **not buildable from another module**: esbuild's
parser is in `internal/js_parser`, and Go's internal-package rule forbids
importing it outside the esbuild module. There is no importable pure-Go
TypeScript parser. Consequently TS/JS analysis joins the tree-sitter (cgo)
track. v1 is Go-only and ships as a pure-Go binary; every other language is a
`cgo`-tagged optional feature.

## Consequences

- The v1 core tool is portable: pure-Go static builds for Go analysis.
- Language support is additive: tree-sitter languages are compile-time opt-in.
- Later language milestones only touch the parser adapter layer, not the metric
  pipeline (ADR 0006).
