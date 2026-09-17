# Changelog

All notable changes to Navune are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `--format yaml`: the `schema_version: 1` report schema serialized as YAML, a
  peer machine contract to JSON (ADR 0008 revision 1).
- Java support via a tree-sitter adapter: full measure set, package-index import
  resolution, type/abstractness layer, `@Generated` detection, and
  `*Test.java`/`Test*.java` classification (ADR 0019).
- Rust support via a tree-sitter adapter: full measure set, `mod`/`use` module
  resolution, struct/enum/trait type layer, and `tests/`/`*_test.rs`
  classification (ADR 0019). Inline `#[cfg(test)]` code counts as production.
- Built-in exclusions for Java/Rust build output (`target`, `.gradle`, `out`).

## [1.0.0] - 2026-09-15

### Added

- `navune analyze` — measures structural quality of a codebase against
  configurable budgets and emits a transparent 0–100 composite.
- `navune init` — writes a commented `navune.yaml` template.
- `navune version` and `navune help [command]`, with per-command help and
  documented exit codes (0 pass, 1 breach, 2 usage/config, 3 internal).
- Multi-language support: Go via `go/parser`; TypeScript/JavaScript, Python, C,
  and C++ via the official tree-sitter bindings (cgo).
- Metrics: physical and logical SLOC, cyclomatic complexity, token-normalized
  duplication, dependency cycles (Tarjan SCC), and Martin coupling metrics
  (Ca/Ce, instability, abstractness, distance from the main sequence).
- Extended structural measures: cognitive complexity, comment density, function
  shape (length/nesting/parameters), duplicated lines, and direct callees.
- The deterministic structural smell layer: ten threshold rules producing
  located, severity-ranked issues (ADR 0016).
- Quality-gate budgets with `error`/`warn` tiers, per-language budgets, and a
  transparent weighted composite index derived from distance-to-budget.
- Output formats: human-readable text, a versioned JSON schema
  (`schema_version: 1`), and a Mermaid dependency-graph export with cycles
  grouped into subgraphs.
- Discovery that skips generated/vendored code, excludes nested modules and
  build output, and reports test files in a separate `tests` namespace.
- Bash completion (`completions/navune.bash`) and a man page (`man/navune.1`),
  installed by `make install-completions`/`make install-man` and shipped in
  release archives (ADR 0018).
- Verification: committed golden fixtures, a self-analysis smoke test, and an
  opt-in fixture-evolution suite (`-tags evolution`).
