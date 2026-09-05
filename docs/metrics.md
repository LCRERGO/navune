# Metrics reference

This page defines every metric Navune computes and how it is reported.
Authoritative definitions also live in [the glossary](glossary.md) and the
ADRs under [`adr/`](adr/).

Navune computes metrics from a **uniform element model** (ADR 0006): every
analyzed file yields `file → types/classes → functions/methods`, a normalized
token stream, and a set of imports. All downstream metric, graph, duplication,
and reporting code is language-agnostic.

## What is measured per file

For each file (production or test), Navune reports:

| Field | Meaning |
|---|---|
| `physical_sloc` | Physical source lines containing at least one code token (comments-only and blank lines excluded). |
| `logical_loc` | Statement/declaration count per AST (language-aware). |
| `functions` | Number of named functions/methods. |
| `avg_complexity` | Mean cyclomatic complexity across the file's functions. |
| `worst_complexity` | Highest single-function complexity in the file. |
| `types` / `abstract_types` | Named types, and how many are abstract (interfaces in Go/TS). |
| `ca` / `ce` | Afferent / efferent coupling (internal files only). |
| `instability`, `abstractness`, `distance` | Coupling-model roll-ups (below). |
| `in_cycle` / `cycle` | Whether the file belongs to a dependency cycle (SCC). |
| `dup_tokens` / `dup_pct` | Duplicated tokens in this file and their share of its tokens. |

## Cyclomatic complexity

Per function: `1 + number of decision points`, where decision points are
language constructs that branch execution:

- Go (`go/parser`): `if`, `for`, `range`, `case`/`comm` clauses, and `&&`/`||`.
- TS/JavaScript (tree-sitter): `if`, `for`/`for-in`/`for-of`, `while`,
  `do`, `switch` cases, `catch`, ternary, and `&&`/`||`.
- Python (tree-sitter): `if`/`elif`, `for`, `while`, `except`, `case`,
  ternary, and `and`/`or`.

Anonymous closures (Go func literals, JS arrows/expressions, Python lambdas)
are not separate function units and their decision points are not attributed
to an enclosing function.

## Duplication

Duplication is detected over the **normalized token stream**: comments and
whitespace are removed; string/number literals are normalized to `STR`/`NUM`;
identifiers and keywords are kept verbatim. A duplicated block is an identical
token sequence of length ≥ 20 shared between two files or two distinct regions
of one file.

Reported as: total/duplicated tokens, duplicated-token percentage (`dup_pct`),
and the number of distinct maximal blocks.

## Dependency graph & coupling

Imports only create edges when they resolve to an **analyzed file** (ADR 0010):

- Go resolves through module import paths.
- TS/JS and Python use workspace-root best-effort resolution
  (see [ADR 0002](adr/0002-languages-and-staging.md)).

For each file Navune computes:

- **Ca (afferent)** — internal files that depend on it.
- **Ce (efferent)** — internal files it depends on.
- **Instability** `I = Ce / (Ca + Ce)` — 0 is maximally stable.
- **Abstractness** `A = abstract types / types`.
- **Distance from main sequence** `|A + I − 1|` — the "zone of pain" indicator.

Cycles are strongly connected components (SCCs) of size > 1 in the internal
file graph; each is reported with its member files.

## Quality gate & the composite index

Budgets and weights come from `navune.yaml` (ADR 0009). The budgetable metrics
and their built-in defaults:

| Budget key | Meaning | Default limit | Default tier |
|---|---|---|---|
| `avg_complexity` | Mean cyclomatic complexity per function | 10 | error |
| `worst_complexity` | Worst single-function complexity | 50 | error |
| `max_duplication_pct` | Duplicated tokens as % of all tokens | 5 | warn |
| `max_cycle_members` | Largest dependency cycle (files) | 8 | error |
| `max_in_cycle_pct` | Files sitting in a dependency cycle | 10 | warn |

The 0–100 **composite index** is a weighted, transparent blend of each metric's
distance from its budget:

```
metric_score = clamp(100 * (1 - (value / limit)), 0, 100)
composite    = Σ (weight_m × metric_score_m) / Σ weight_m
```

Budgets decide the verdict and exit code; the composite is informational
(ADR 0009).

## Test files

Test files (`*_test.go`, `*.test.ts`/`*.spec.ts`, `*_test.py`/`test_*`) are
analyzed into a separate `tests` namespace: reported, but excluded from
budgets, cycles, coupling, and the composite (ADR 0011).
