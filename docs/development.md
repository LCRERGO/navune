# Developing Navune

Navune is a Go CLI. Everything below the command lives under `internal/` and
is private; `cmd/navune` is the only binary.

## Orientation

```
internal/lang            uniform element model (file → type → function) + parser interface
internal/lang/golang     Go adapter over go/parser (pure Go)
internal/lang/treescript TS/JS adapter over tree-sitter JS/TS grammars (cgo)
internal/lang/python     Python adapter over the tree-sitter Python grammar (cgo)
internal/discover        file walking, exclusions, nested-module detection, test classification
internal/analysis        pipeline orchestration; script import resolution; the Report model
internal/dup             token-normalized duplication detection
internal/graph           internal dependency graph, SCC cycle detection, coupling metrics
internal/gate            budget evaluation → verdict, exit codes, composite index
internal/report          text / JSON / Mermaid renderers
internal/config          navune.yaml parsing, defaults, budgets, weights, exclusions
internal/evolution       opt-in fixture-evolution tests (build tag `evolution`)
```

The element model is the contract: language adapters turn a source file into a
`lang.FileResult` (elements + a normalized token stream). Every metric, graph,
duplication, and report step then runs language-agnostically. Keep it that way:
new languages are adapters, not pipeline changes.

## Building & testing

See [AGENTS.md](../AGENTS.md) for exact commands. In short:

```sh
make build      # writes bin/navune
make test       # default suite (unit + golden fixtures + self-analysis)
make vet        # go vet ./...
make evolve     # opt-in fixture-evolution suite (-tags evolution)
```

cgo is mandatory (tree-sitter grammars); do not try `CGO_ENABLED=0`.

## Metrics invariants

- **File is the unit of analysis** in every language (ADR 0006).
- Only **internal edges** participate in graph metrics and gates; unresolved
  imports (stdlib, `node_modules`, site-packages) never create edges
  (ADR 0010).
- **Test files** are analyzed but reported in a separate `tests` namespace —
  excluded from budgets, cycles, coupling, and the composite (ADR 0011).
- Duplication is token-based and language-agnostic: comments stripped,
  string/number literals normalized, identifiers verbatim.
- Report output is deterministic; the JSON schema (`schema_version: 1`) is a
  frozen contract — do not rename `json` tags casually.

## Extending a language / adding one

Adding a language touches several places; see the checklist in
[AGENTS.md](../AGENTS.md) ("Adding a language"). The hard parts are usually
import/dependency resolution (Go: module import paths; scripts:
`internal/analysis/resolve.go`) and grammar node-kind mapping in the
tree-sitter adapters. Inspect the installed grammar's `node-types.json` under
`$GOMODCACHE/github.com/tree-sitter/...` before hand-editing kind maps — node
kind names are grammar-specific and the file layout differs per grammar
(TS nests under `typescript/src`, Python does not).

## Docs

- `README.md` — the primary user-facing document: quickstart, language table,
  configuration summary, composite formula, architecture, links onward.
- [`docs/usage.md`](usage.md) — full CLI reference (commands, flags, exit
  codes, examples).
- [`docs/metrics.md`](metrics.md) — per-metric definitions and formulas.
- [`docs/glossary.md`](glossary.md) — shared vocabulary (authoritative metric
  wording).
- [`docs/adr/`](adr/) — the design record; read before changing metric
  semantics.

Keep the split clean: README sells and orients; usage/metrics reference;
glossary and ADRs are the record. When you change a metric, update
`docs/metrics.md` and the glossary together, and re-check the fixture-evolution
suite.
