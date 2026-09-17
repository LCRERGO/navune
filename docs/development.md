# Developing Navune

Navune is a Go CLI. Everything below the command lives under `internal/` and
is private; `cmd/navune` is the only binary.

## Orientation

```
internal/lang            uniform element model (file → type → function) + parser interface
internal/lang/golang     Go adapter over go/parser (pure Go)
internal/lang/treescript TS/JS adapter over tree-sitter JS/TS grammars (cgo)
internal/lang/python     Python adapter over the tree-sitter Python grammar (cgo)
internal/lang/cfamily    C/C++ adapter over tree-sitter C/C++ grammars (cgo, shared)
internal/discover        file walking, exclusions, nested-module detection, test classification
internal/analysis        pipeline orchestration; script import resolution; the Report model
internal/dup             token-normalized duplication detection
internal/graph           internal dependency graph, SCC cycle detection, coupling metrics
internal/smell           deterministic structural rules → located issues
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

## Releasing

Releases are built and published by GitHub Actions on the GitHub mirror
(`github.com/LCRERGO/Navune`); the canonical repository stays on Codeberg. See
[ADR 0014](adr/0014-release-and-ci.md) for the rationale.

To cut a release:

1. Add the version's section to `CHANGELOG.md` (Keep-a-Changelog format, heading
   `## [<version>]`). The publish job extracts it as the release notes and fails
   if the section is missing.
2. Push an annotated tag `v<version>` (e.g. `v0.2.0`). Tags containing a hyphen
   (`v0.2.0-rc1`) are published as GitHub pre-releases.
3. CI runs gofmt/vet/test, builds linux/amd64, linux/arm64, darwin/amd64,
   darwin/arm64, and windows/amd64 natively, then attaches
   `navune_<version>_<os>_<arch>.tar.gz`/`.zip` and `checksums.txt`.

Run the release workflow manually (`workflow_dispatch`) for a dry-run: it builds
and packages everything as a workflow artifact but creates no release.

## Metrics invariants

- **File is the unit of analysis** in every language (ADR 0006).
- Only **internal edges** participate in graph metrics and gates; unresolved
  imports (stdlib, `node_modules`, site-packages, system headers) never create
  edges (ADR 0010). C/C++ `#include` resolution is quoted-relative plus
  `include_paths` (ADR 0017).
- **Test files** are analyzed but reported in a separate `tests` namespace —
  excluded from budgets, cycles, coupling, the composite, and the smell layer
  (ADR 0011). Test classification is filename patterns **or** a `test`/`tests`
  path component below the root, for every language.
- **Anonymous functions** are first-class function units (`<anonymous@Lstart>`)
  and are measured by every metric (ADR 0015).
- Duplication is token-based and language-agnostic: comments stripped,
  string/number literals normalized, identifiers verbatim.
- **Budgets are upper-bound-only**; per-language budgets are evaluated in
  addition to global ones, and the composite uses global values only
  (ADR 0009, ADR 0016). Smell rules never gate unless a budget is configured.
- Report output is deterministic; the JSON schema (`schema_version: 1`) is a
  frozen contract — additive fields only, and do not rename `json` tags
  casually. Issues are sorted by (path, line, rule).

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
