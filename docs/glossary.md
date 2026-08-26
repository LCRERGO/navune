# Navune Glossary

Terms as defined by Navune's ADRs. Metric definitions are the authoritative
formulas behind every number Navune reports.

## Units & structure

- **File (a.k.a. module, analysis unit)** — the unit of analysis everywhere.
  All dependency edges, coupling metrics, and cycles are computed at file
  granularity (ADR 0006). In Go, package-level imports are flattened to edges
  to each analyzed file of the imported package.
- **Second layer** — types/classes (not present for C) nested under files.
- **Third layer** — functions/methods nested under types or files.
- **Test file** — analyzed and reported in a separate `tests` namespace,
  excluded from budgets, coupling/cycle math, and the composite (ADR 0011).
- **Generated / vendored file** — skipped entirely (built-in per-language
  exclusion patterns, overridable) (ADR 0011).

## Metrics

- **Physical SLOC** — count of physical source lines in a file (non-empty,
  excluding pure-comment lines).
- **Logical SLOC (NCLOC)** — statements/declarations per AST, language-aware.
- **Cyclomatic complexity** — per function: 1 + number of decision points
  (`if`, `for`, `while`, `case`, `&&`, `||`, `?`, exception handlers, etc.),
  language-aware. Rolled up to worst-case and average per file.
- **Ca (afferent coupling)** — number of internal files that depend on this file.
- **Ce (efferent coupling)** — number of internal files this file depends on.
- **Instability I** — Ce / (Ca + Ce). 0 = maximally stable, 1 = maximally
  unstable. Internal edges only (ADR 0010).
- **Abstractness A** — fraction of a file's elements that are abstract
  (interfaces/abstract classes), where the language supports it.
- **Distance from main sequence** — |A + I − 1|. Low = balanced; high = "zone of
  pain/uselessness".
- **Cycle / SCC** — a strongly connected component in the internal dependency
  graph; structural debt (ADR 0003). Reported with members and candidate edges
  to break.
- **Duplication** — language-aware token-normalized blocks shared across or
  within files, above minimum-length thresholds, expressed as count and % of
  tokens/lines duplicated.

## Quality model

- **Budget** — a per-metric limit configured in `navune.yaml` (e.g., max average
  complexity, max % duplication). Breach of an `error`-tier budget → nonzero
  exit (ADR 0009).
- **Composite index (0–100)** — transparent blend: per-metric distance-from-
  budget scores combined with config-file weights. Formula documented in the
  README (ADR 0009).
- **Internal graph** — dependency edges between files *within* the analysis
  root; the only edges that participate in metrics/gates (ADR 0010).
- **External edge** — an import/resolution pointing outside the analysis root;
  recorded for reporting only (fan-out), never in metric math.

## Runtime / backend

- **Language adapter** — the layer translating a parsed AST into Navune's
  uniform element model (file → type → function) plus token stream. One per
  language (ADR 0006).
- **Pure-Go adapter** — `go/parser` for Go. No cgo.
- **tree-sitter adapter** — tree-sitter grammar-driven adapters for TS/JS and
  Python (`internal/lang/treescript`, `internal/lang/python`). Require cgo;
  compiled in by default (ADR 0007).
- **navune.yaml** — configuration: exclusions, budgets, composite weights,
  severity tiers. Discovered upward from the analyze target; `navune init`
  scaffolds it.
- **Workspace-root import resolution** — for TS/JS and Python, an import is
  internal only if it resolves to an analyzed file under the analysis root
  (relative specifiers / dotted modules; best-effort). Everything else is
  external and excluded from metric math (ADR 0010).

## CLI

- **Exit codes** — 0 pass; 1 budget breach (error tier); 2 usage/config error;
  3+ reserved (ADR 0012).
- **Text / JSON / Mermaid** — the three output formats of `navune analyze`
  (ADR 0008). JSON is the versioned machine contract.
