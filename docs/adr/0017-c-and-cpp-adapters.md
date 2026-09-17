# ADR 0017 — C and C++ support via a shared tree-sitter adapter

- Status: accepted
- Date: 2026-09-17
- Decision-maker: product owner via design interview (Q2–Q4, Q12–Q13, Q22, Q48–Q51, Q53, Q59)

## Context

ADR 0002 and ADR 0007 deferred C and C++ but committed to the same tree-sitter
adapter architecture. ADR 0006 already fixed the model: file is the unit, edges
come from `#include`, C has no type layer, and C++ does. The `tree-sitter-c`
and `tree-sitter-cpp` Go bindings are already present in `go.sum` and the module
cache, so the work is adapter code, not new infrastructure.

C/C++ differ from the existing languages in one important way: `#include` is
textual and depends on include paths, so import resolution is best-effort in the
same spirit as the workspace-root resolution for scripts (ADR 0010).

## Decision

- **One package**, `internal/lang/cfamily`, exposes `NewC()` and `NewCPP()`,
  mirroring how `treescript` hosts two grammars (TS/JS).
- **Extensions**: `.c` → C; `.cc`, `.cpp`, `.cxx`, `.c++` → C++; `.hpp`, `.hh`,
  `.hxx` → C++. `.h` is ambiguous and is **content-sniffed**: comments and
  string/char literals are stripped, then the markers `class`, `template`,
  `namespace`, `::`, and `extern "C"` select C++; absent any marker the default
  is C++. The resolved language is recorded per file.
- **`#include` edges** (internal-only, ADR 0010): quoted includes resolve
  relative to the including file's directory first, then against
  `include_paths`; angle includes resolve only against `include_paths`; first
  match wins; unresolved includes are external and never create edges.
- **`include_paths`** is a new config key: a list of directories resolved
  relative to the config file. The default is empty.
- **Types**: C has no type layer (`types`/`abstract_types` = 0). C++ types are
  classes and structs; an abstract type is a class with at least one pure
  virtual method (`= 0`).
- **Test detection**: C/C++ filename patterns (`test_*` and `*_test` with
  `.c/.cc/.cpp/.cxx/.h/.hpp/.hh`) plus the directory rule of ADR 0011.
- **Generated detection**: the `Code generated … DO NOT EDIT` marker is
  recognized in C/C++ comments, reusing `IsGenerated`/`MarkGenerated`.
- **Verification**: a self-contained `test/fixture-c` module (a three-file
  include cycle, one test file, one generated file, one excluded dependency
  directory), an integration test in the `multilang_test.go` style, and
  evolution-suite generators (complexity, cycle, duplication, gate) plus
  smell-mutation steps.

The adapters implement the full measure set of ADR 0015.

## Consequences

- C/C++ slot into the existing pipeline with no metric, graph, or report
  changes.
- `.h` classification depends on file content, so the resolved language is
  recorded per file and drives per-language smell defaults (ADR 0016).
- C++ headers are measured once (file unit), unlike compilers that measure a
  header once per translation unit.
- `include_paths` is deliberately minimal; compilation-database or build-system
  include discovery is out of scope.
- Adding the grammars makes the two already-present `go.sum` entries real
  dependencies in `go.mod`.
