# Navune usage

Navune is a command-line analyzer: it measures a codebase's structural quality
(size, complexity, duplication, dependency cycles, coupling) and evaluates the
result against configurable budgets.

## Commands

```
navune analyze <path> [options]    measure a codebase's structural quality
navune init [path]                 write a commented navune.yaml template
navune version                     print version and supported languages
navune help [command]              show help for navune or one command
```

Run `navune help <command>` for per-command help and examples.

### analyze

```
navune analyze <path> [options]
```

Analyzes the codebase rooted at `<path>` (defaults to `.`) and writes a
report to stdout.

| Option          | Description                                                     |
|-----------------|-----------------------------------------------------------------|
| `--config FILE` | Use `FILE` instead of discovering `navune.yaml` upward from the target. |
| `--format FMT`  | Output format: `text` (default), `json`, or `mermaid`.          |
| `--out FILE`    | Write the report to `FILE` instead of stdout.                     |

Exit code reflects the quality gate: see [Exit codes](#exit-codes).

### init

```
navune init [path]
```

Writes a commented `navune.yaml` template into the directory `path` (default
`.`). Fails if the file already exists.

### version

Prints the version, platform, and the languages Navune can analyze.

### help

`navune help` prints the top-level help. `navune help <command>` prints help
for that command. `-h` / `--help` on a command does the same.

## Exit codes

| Code | Meaning                                                        |
|------|----------------------------------------------------------------|
| 0    | Pass — all `error`-tier budgets satisfied.                     |
| 1    | Breach — at least one `error`-tier budget was exceeded.        |
| 2    | Usage or configuration error.                                  |
| 3    | Internal error (e.g. a source file could not be parsed).       |

`warn`-tier breaches are reported but do not change the exit code; only
`error`-tier budgets fail a run (ADR 0009).

## Configuration discovery

`navune analyze` looks for `navune.yaml` by walking upward from `<path>` to the
filesystem root, and uses the first one found. Pass `--config` to bypass
discovery. `navune init` scaffolds a commented template you can edit.

See [Configuration](../README.md#configuration-navuneyaml) in the README for
the schema, or run `navune init` and read the generated file.

## Examples

```sh
# Analyze the current directory (default text report, exit code gates CI).
navune analyze .

# Machine-readable report for CI consumers.
navune analyze ./src --format json

# Dependency graph for your editor (cycles rendered as subgraphs).
navune analyze . --format mermaid --out deps.mmd

# Tighten the gate with a project-local config.
navune init
navune analyze . --config ./navune.yaml

# Show help.
navune help analyze
```
