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

Adopt a **tree-sitter backend compiled in by default**:

- Go analysis uses `go/parser` (pure Go).
- TypeScript/JavaScript and Python use the official tree-sitter Go bindings
  (runtime `github.com/tree-sitter/go-tree-sitter` plus the JS/TS and Python
  grammar bindings) compiled unconditionally into the binary.

## Revision 1 (2026-09-08)

The original decision named esbuild as the TS/JS parser on the strength of its
"pure Go" parser. That is **not buildable from another module**: esbuild's
parser is in `internal/js_parser`, and Go's internal-package rule forbids
importing it outside the esbuild module. There is no importable pure-Go
TypeScript parser.

## Revision 2 (2026-09-08)

The pure-Go constraint was lifted: tree-sitter (and therefore cgo) is now
allowed, and TS/JS + Python ship as first-class languages. Tree-sitter's Go
bindings require cgo (the runtime and every grammar binding use `#cgo`), so the
tool now **requires CGO_ENABLED=1 and a C toolchain** for all builds. This
replaces the earlier "cgo behind build tags / pure-Go core" posture.

## Consequences

- The whole tool requires cgo: no static/cross builds without a C toolchain.
- Language support is additive: each grammar is a self-contained adapter over
  the same element model (ADR 0006).
- C and C++ remain future milestones; they will reuse the same tree-sitter
  adapter architecture.
- Later language milestones only touch the parser adapter layer, not the metric
  pipeline (ADR 0006).
