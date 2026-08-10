# Navune

**Structural code-quality analysis for your codebase — in one CLI.**

Navune (NAV + UNE, from Latin *nāvis*, "ship/navigate") is a command-line
analyzer that computes *structural quality* metrics for Go
codebases: size, cyclomatic complexity, duplication, internal dependency
graphs, cyclic dependencies, and Martin's coupling metrics. It judges the
result against configurable budgets and emits a transparent 0–100 composite
score. Additional languages (TypeScript/JavaScript, Python, C, C++) are on
the roadmap via build-tagged tree-sitter support.

```sh
navune analyze .                  # default text report
navune analyze ./src --format json
navune analyze . --format mermaid --out deps.mmd
navune init                       # writes a commented navune.yaml
```

```
$ navune analyze ./internal/graph
┌ Modules                                NCLOC  Cyc(avg)  Cyc(worst)  Dup%  In-cycle
├── internal/graph/graph.go              120       2.3           7   3.2%       yes
├── internal/graph/graph_test.go          44       1.0           2   0.0%        no
...
Cycle: graph.go → graph.go (self)          (break: internal dependency)
Composite: 84 / 100    Budget: 2 breached (error)   Exit: 1
```

## Why structural quality?

Navune is deliberately **structural**, not a linter and not a security scanner. It measures how a codebase is *structured* — dependency cycles,
coupling, complexity, duplication — the properties that determine whether a
codebase stays maintainable as it grows. Bug hunting and vulnerability scanning
are the domain of language-native tools (`go vet`, `staticcheck`,
`govulncheck`) and Navune does not try to replace them.

## Highlights

- **Unit of analysis is the file, in every language** — one uniform element
  model (`file → type/class → function/method`), one metric pipeline.
- **Full v1 metric set**: physical & logical SLOC, cyclomatic complexity,
  duplication, dependency cycles (SCCs), Ca/Ce → instability, abstractness,
  and distance from the main sequence.
- **Budgets first, composite second.** `navune.yaml` defines per-metric limits;
  breach an `error`-tier budget and the exit code says so (0 pass, 1 breach,
  2 usage error). The 0–100 composite is a *transparent, documented blend* of
  per-metric distance-from-budget — no opaque industry numerology.
- **Pure-Go core.** v1 analyzes Go with the standard `go/parser` — zero cgo,
  clean static builds, cross-compilable, run anywhere. Other languages are
  added later as build-tagged optional features.
- **Machine contracts.** A versioned JSON schema is the primary integration
  surface; a Mermaid exporter renders the dependency graph (cycles visible in
  your editor) for free.

## Language support

| Language            | Backend                    | Status |
|---------------------|----------------------------|--------|
| Go                  | `go/parser` (pure Go)      | v1     |
| TypeScript/JavaScript | tree-sitter (cgo)         | planned |
| Python              | tree-sitter (cgo)          | planned |
| C                   | tree-sitter (cgo)          | planned |
| C++                 | tree-sitter (cgo)          | planned |

> Why not more languages in v1? The TS/JS parser in esbuild is internal and
> cannot be imported, and no importable pure-Go TypeScript parser exists (see
> [ADR 0007](docs/adr/0007-parsing-backend-cgo.md)). Post-v1 languages require
> a `cgo`-enabled build and are compiled in via build tags; without them the
> tool still builds and runs with full Go support.

## What counts as "the codebase"

- **Generated & vendored code is skipped** (`vendor/`, `node_modules/`,
  `*.pb.go`, `*.min.js`, lockfiles, …) with sensible built-in patterns,
  overridable in config.
- **Test files are analyzed but kept in a separate `tests` namespace** — visible
  in reports, excluded from budgets, cycles, coupling, and the composite.
- **Only internal edges count** toward metrics and gates. External imports
  (stdlib, third-party) are recorded and shown as fan-out, never in the math.

## Install

```sh
go install github.com/lcr/navune/cmd/navune@latest
```

## Usage

```
navune analyze <path> [--config file] [--format text|json|mermaid] [--out file]
navune init    [path]     # write a commented navune.yaml template
navune version            # print version and supported languages
```

`navune analyze` auto-discovers `navune.yaml` by walking upward from `<path>`;
`--config` overrides discovery. Exit codes: **0** pass · **1** budget breach
(error tier) · **2** usage/configuration error.

## Configuration (`navune.yaml`)

```yaml
version: 1

exclude:            # extra globs on top of built-in patterns
  - "**/legacy/**"

tests:
  include: []       # extra test-file patterns; built-ins are language-aware

budgets:            # per-metric limits; tier sets severity (error|warn)
  avg_complexity:        { limit: 10,  tier: error }
  worst_complexity:      { limit: 50,  tier: error }
  max_duplication_pct:   { limit: 5.0, tier: warn }
  max_cycle_members:     { limit: 8,   tier: error }
  max_in_cycle_pct:      { limit: 10,  tier: warn }

weights:            # composite blend, weights per metric (see formula below)
  avg_complexity:        25
  max_duplication_pct:   20
  max_in_cycle_pct:      25
  ...
```

Run `navune init` to get the full commented template.

## The composite index (transparent by design)

The 0–100 composite is *not* an invented industry score. For each weighted
metric, Navune computes how far the measured value is from its configured
budget:

```
metric_score = clamp(100 * (1 - (value / budget)), 0, 100)   # smaller is better
composite    = Σ (weight_m × metric_score_m) / Σ weight_m     # 0..100
```

Every number in the report is traceable to a formula and a config line. The
budget is the anchor; breach one and the exit code flips regardless of the
composite — budgets decide, the composite informs.

## Architecture

The repository follows the [golang-standards project layout](https://github.com/golang-standards/project-layout).

```
cmd/navune             CLI entry point (analyze / init / version)
internal/              private application & analysis code (not importable by others)
  analysis             pipeline orchestration; summary aggregation; stable Report model
  config               navune.yaml parsing, defaults, budgets, weights, exclusions
  discover             file walking, exclusions, nested-module detection, go.mod lookup
  dup                  token-normalized duplication detection (cross-file + intra-file)
  gate                 budget evaluation, verdict, exit codes, transparent composite
  graph                internal dependency graph, SCC cycle detection (Tarjan),
                       coupling Ca/Ce, instability/abstractness, main-sequence distance
  lang                 uniform element model (file → type → function); parser interface
  lang/golang          Go adapter over go/parser → element model + normalized tokens
  report               text summary, versioned JSON schema, Mermaid graph export
configs/               sample navune.yaml configuration
docs/                  ADRs and the metric glossary (design record)
test/                  external test data: committed golden fixtures (Go), nested module
```

Language adapters convert a parsed AST into Navune's uniform element model and
a normalized token stream, so the entire metric/graph/report pipeline is
written once and reused by every future language.

### Repository layout notes

- `internal/` holds all Go packages; they are private by compiler enforcement.
- `test/` contains `fixture/`, a self-contained Go module used as golden test
  data. Being a nested module (`go.mod` inside), it is excluded from `go
  build ./...`, `go test ./...`, and Navune's own analysis runs, exactly like
  the Go toolchain skips nested modules.
- `configs/navune.yaml.example` documents a full configuration file.

## Roadmap

- **v1** — Go, full metric model including duplication.
- **v1.x** — HTML report; directory-level aggregation views.
- **later milestones** — TypeScript/JavaScript, then Python, then C/C++ via
  build-tagged tree-sitter.

## Development

```sh
make build    # or: go build ./...
make test     # or: go test ./...
make vet      # or: go vet ./...
make analyze  # run Navune on its own source tree
```

Verification is golden-test based: committed synthetic Go fixtures under
`test/fixture/` cover every metric family, and Navune analyzes its own source
tree as an always-on smoke test. Performance is a CI contract (~<30 s for a
100–500k LOC repo, parallelized across files).

## Design record

Every design decision is recorded as an ADR in [`docs/adr/`](docs/adr/) —
tool shape, metric model, file-level units, parser backend, quality-gate
semantics, output contracts, and the verification bar. A shared vocabulary
lives in [`docs/glossary.md`](docs/glossary.md). Metric definitions there are
authoritative.

## License

MIT.
