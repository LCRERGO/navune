# ADR 0012 — Subcommand-first CLI surface

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q15)

## Context

Go-ecosystem tools split between lint-style single-command invocation
(`staticcheck`, `go vet`) and subcommand suites (`go build`). Navune needs room
to grow (later languages, config management) while keeping the primary analysis
action obvious.

## Decision

Navune uses a **subcommand-first** surface:

```
navune analyze <path> [--config file] [--format text|json|mermaid] [--out file]
navune init    [path]          # writes a commented navune.yaml template
navune version                 # prints version and supported languages
```

- `analyze` requires a path (defaults to `.`) and auto-discovers `navune.yaml`
  upward from the target; `--config` overrides discovery.
- `init` scaffolds the config for the given path.
- Unknown subcommands print usage and exit nonzero.

## Consequences

- Clear extension point: future `navune audit`, `navune languages`, etc. slot in
  as subcommands.
- The analyze invocation is a single obvious verb for CI use.
- Exit codes are reserved: 0 = pass, 1 = budget breach (error tier), 2 = usage/
  configuration error, 3+ reserved for future severities.
