# AGENTS.md

Navune is a Go CLI that statically measures structural code quality (SLOC,
cyclomatic complexity, duplication, dependency cycles, Martin's coupling
metrics) of Go, TypeScript/JavaScript, and Python codebases. Design decisions
and metric definitions live in `docs/adr/*.md` and `docs/glossary.md`; read
those before changing metric semantics.

## Build & test (cgo is mandatory)

- Requires **cgo and a C toolchain**: tree-sitter grammar bindings do not
  build with `CGO_ENABLED=0`. If you see `build constraints exclude all Go
  files in .../tree-sitter-*`, that is why.
- `make build` → writes `bin/navune` (plain `go build ./...` emits no binary).
- `make test` / `go test ./...`; a single package: `go test ./internal/lang/python/`.
- `make analyze` runs Navune on its own repo (always-on self-analysis smoke test).
- `make evolve` runs the opt-in fixture-evolution suite
  (`go test -tags evolution ./internal/evolution/...`): staged mutations of tiny
  multi-language codebases asserting metrics move in controlled directions
  (complexity rises, cycles appear, duplication appears, budget breach flips
  exit code).
- Verification order: `gofmt -l .` → `go vet ./...` → `go test ./...`.
- Commits use conventional prefixes (`feat:`, `fix:`, `docs:`, `test:`, `chore:`),
  each a small logical slice.

## Repo layout & the fixture gotcha

- `cmd/navune` is the only main; everything under `internal/` is private.
- `internal/lang/<golang|treescript|python>` are **adapters** producing a uniform
  `lang.FileResult` (files → types → functions, normalized token stream).
  Analysis, graph, duplication, and report code is language-agnostic.
- **New test fixtures under `test/` MUST be self-contained modules** (own
  `go.mod`, e.g. `test/fixture`, `test/fixture-ts`, `test/fixture-py`). Without
  one they get swept into `go test ./...` and into Navune's self-analysis,
  polluting metrics. Discovery deliberately skips nested `go.mod` dirs.
- Integration tests reference fixtures by relative path, e.g.
  `internal/analysis/multilang_test.go` uses `../../test/fixture-ts`.

## Adding a language (or extending one)

Each language requires edits in several places — easy to miss one:
1. `internal/lang/lang.go`: add a `Lang` const.
2. `internal/discover/discover.go`: extension→`Lang` map (`extLang`) and
   `IsTestFile` filename patterns.
3. New adapter package under `internal/lang/` implementing `lang.Parser`
   (Parse, Lang, Extensions). Adapt record imports into `FileResult.Imports`;
   Go edges resolve via package `ImportPath` (see graph), scripts need a
   `resolveScriptDeps` branch plus candidate rules.
4. `internal/analysis/analysis.go`: `parserFor()` dispatch.
5. Import→file resolution for scripts lives in `internal/analysis/resolve.go`
   (TS: relative/index candidates; Python: dotted + relative + submodule guess);
   tests in `resolve_test.go`.
6. `cmd/navune/main.go`: `languages:` string in `runVersion`.
7. Golden fixture module under `test/` + an integration test.

Tree-sitter adapters key off grammar **node kinds** (e.g. `method_definition`,
`elif_clause`). If a construct behaves wrong, inspect the grammar's real kinds
before hand-editing maps: each installed grammar module ships a
`node-types.json` under `$GOMODCACHE/github.com/tree-sitter/...` — e.g.
`tree-sitter-typescript@…/typescript/src/node-types.json` (nested), but
`tree-sitter-python@…/src/node-types.json`. TS and JS use separate grammars
(TS/TSX vs plain JS) — extension determines which.

## Metrics & pipeline invariants

- Unit of analysis is the **file** in every language. Graph edges are built
  only for imports that resolve to analyzed files (internal); everything else
  (stdlib, `node_modules`, site-packages) never becomes an edge and is not
  reported today.
- Test files (`*_test.go`, `*.test.ts`/`*.spec.ts`, `*_test.py`/`test_*`) are
  analyzed into a separate `tests` namespace: excluded from budgets, cycles,
  coupling, and the composite.
- Duplication is token-based and language-agnostic: comments stripped,
  string/number literals normalized to `STR`/`NUM`, identifiers kept verbatim.
- Report output must stay **deterministic** (stable ordering) and the JSON
  schema is a frozen contract (`schema_version: 1`) — don't rename `json` tags
  casually.
