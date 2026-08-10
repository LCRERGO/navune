# ADR 0002 — Target languages and staging roadmap

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q2, Q5, Q10)

## Context

Wide-coverage analyzers handle dozens of languages; narrowly-focused ones cover
a small set (e.g. a managed or native ecosystem plus a couple of others).
Multi-language support is the single largest cost driver: every extra grammar
multiplies parser, mapping, and test effort. The tool is written in Go, which
also makes Go the first language it can analyze for free via the standard
toolchain.

## Decision

Navune targets **five languages** over its product lifetime:

| Language      | Backend                    | Release |
|---------------|----------------------------|---------|
| Go            | `go/parser` (pure Go)      | v1      |
| TS/JavaScript | tree-sitter (cgo)          | later   |
| Python        | tree-sitter (cgo)          | later   |
| C             | tree-sitter (cgo)          | later   |
| C++           | tree-sitter (cgo)          | later   |

**v1 ships Go only, with the full metric model.** TS/JS → Python → C → C++
land as subsequent milestone releases, each inheriting a frozen, tested
pipeline rather than a half-built one. The parser layer is behind a language
interface so later languages slot in without touching metric, graph, or
reporting code.

## Revision (2026-09-08)

The original decision put TypeScript/JavaScript in v1 via esbuild's pure-Go
parser. That is **not buildable**: esbuild's parser lives in its `internal/`
package, which Go forbids importing from outside that module, and no importable
pure-Go TypeScript parser exists. v1 was therefore narrowed to Go only; TS/JS
moves to the tree-sitter (cgo) milestone track. This preserves the portable
pure-Go core and keeps every metric promise, at the cost of a smaller first
release. See ADR 0007.

## Consequences

- v1 is genuinely deliverable and defensible instead of several half-languages.
- Grammar work for TS/JS, Python, C/C++ is deferred but architecturally
  anticipated behind the language interface.
- cgo is required only for post-v1 languages (see ADR 0007).
- The v1 default binary is pure Go: portable, static, no C toolchain needed.
- The full metric model (ADR 0005) is proven on Go before being replayed across
  other grammars.
