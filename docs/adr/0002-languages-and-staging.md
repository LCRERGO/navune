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

| Language      | Backend                    | Status |
|---------------|----------------------------|--------|
| Go            | `go/parser` (pure Go)      | v1     |
| TS/JavaScript | tree-sitter (cgo)          | v1.1   |
| Python        | tree-sitter (cgo)          | v1.1   |
| C             | tree-sitter (cgo)          | later  |
| C++           | tree-sitter (cgo)          | later  |

**v1 ships Go only, with the full metric model. TS/JS and Python ship in the
next release** via the official tree-sitter Go bindings; C → C++ land as
subsequent milestone releases. Each language inherits a frozen, tested
pipeline rather than a half-built one. The parser layer is behind a language
interface so later languages slot in without touching metric, graph, or
reporting code.

## Revision 1 (2026-09-08)

The original decision put TypeScript/JavaScript in v1 via esbuild's pure-Go
parser. That is **not buildable**: esbuild's parser lives in its `internal/`
package, which Go forbids importing from outside that module, and no importable
pure-Go TypeScript parser exists. v1 was therefore narrowed to Go only. See
ADR 0007.

## Revision 2 (2026-09-08)

The pure-Go-only constraint was lifted: **tree-sitter is allowed**, and with it
cgo. TypeScript/JavaScript and Python are analyzed by tree-sitter grammars
(`internal/lang/treescript`, `internal/lang/python`) producing the same
uniform element model as Go. The binary now requires CGO and a C toolchain to
build. This reverses the earlier "portable pure-Go core" consequence: it is
traded for real multi-language structural analysis.

## Consequences

- v1 is genuinely deliverable and defensible instead of several half-languages.
- TS/JS and Python share one tree-sitter-driven adapter architecture; C/C++
  are deferred but anticipated behind the language interface.
- cgo is now a build requirement for the whole tool (see ADR 0007).
- The full metric model (ADR 0005) is proven on Go, then replayed across the
  tree-sitter grammars.
- Import/dependency resolution for TS/JS and Python is best-effort and
  workspace-root based (relative specifiers and dotted modules mapped to
  analyzed files); unresolved references count as external (ADR 0010).
