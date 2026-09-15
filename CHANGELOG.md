# Changelog

All notable changes to Navune are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-09-15

### Added

- `navune analyze` — measures structural quality of a codebase against
  configurable budgets and emits a transparent 0–100 composite.
- `navune init` — writes a commented `navune.yaml` template.
- `navune version` and `navune help [command]`, with per-command help and
  documented exit codes (0 pass, 1 breach, 2 usage/config, 3 internal).
- Multi-language support: Go via `go/parser`, TypeScript/JavaScript and Python
  via the official tree-sitter bindings (cgo).
- Metrics: physical and logical SLOC, cyclomatic complexity, token-normalized
  duplication, dependency cycles (Tarjan SCC), and Martin coupling metrics
  (Ca/Ce, instability, abstractness, distance from the main sequence).
- Quality-gate budgets with `error`/`warn` tiers, and a transparent weighted
  composite index derived from distance-to-budget.
- Output formats: human-readable text, a versioned JSON schema
  (`schema_version: 1`), and a Mermaid dependency-graph export with cycles
  grouped into subgraphs.
- Discovery that skips generated/vendored code, excludes nested modules, and
  reports test files in a separate `tests` namespace.
- Verification: committed golden fixtures, a self-analysis smoke test, and an
  opt-in fixture-evolution suite (`-tags evolution`).
