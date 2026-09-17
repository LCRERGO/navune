# Metrics reference

This page defines every metric Navune computes and how it is reported.
Authoritative definitions also live in [the glossary](glossary.md) and the
ADRs under [`adr/`](adr/).

Navune computes metrics from a **uniform element model** (ADR 0006): every
analyzed file yields `file → types/classes → functions/methods`, a normalized
token stream, and a set of imports. All downstream metric, graph, duplication,
and reporting code is language-agnostic.

On top of the measures, a deterministic **smell layer** (ADR 0016) turns
threshold breaches into located `issues`.

## What is measured per file

For each file (production or test), Navune reports:

| Field | Meaning |
|---|---|
| `lang` | Resolved language of the file (canonical key; `.h` is sniffed, ADR 0017). |
| `physical_sloc` | Physical source lines containing at least one code token (comments-only and blank lines excluded). |
| `logical_loc` | Statement/declaration count per AST (language-aware). |
| `comment_lines` / `comment_pct` | Significant comment lines, and `comment_lines / (physical_sloc + comment_lines)`. |
| `functions` | Number of functions/methods, including anonymous functions. |
| `avg_complexity` | Mean cyclomatic complexity across the file's functions. |
| `worst_complexity` | Highest single-function complexity in the file. |
| `avg_cognitive_complexity` / `cognitive_complexity` | Mean and worst per-function cognitive complexity. |
| `max_function_length` | Longest function in non-comment physical lines. |
| `max_nesting` | Deepest control-structure nesting in any function. |
| `max_params` | Largest parameter count of any function. |
| `types` / `abstract_types` | Named types, and how many are abstract (interfaces in Go/TS, pure-virtual classes in C++). |
| `ca` / `ce` | Afferent / efferent coupling (internal files only). |
| `instability`, `abstractness`, `distance` | Coupling-model roll-ups (below). |
| `in_cycle` / `cycle` | Whether the file belongs to a dependency cycle (SCC). |
| `dup_tokens` / `dup_pct` | Duplicated tokens in this file and their share of its tokens. |
| `duplicated_lines` | Physical lines spanned by duplicated token blocks. |

## Cyclomatic complexity

Per function: `1 + number of decision points`, where decision points are
language constructs that branch execution:

- Go (`go/parser`): `if`, `for`, `range`, `case`/`comm` clauses, and `&&`/`||`.
- TS/JavaScript (tree-sitter): `if`, `for`/`for-in`/`for-of`, `while`,
  `do`, `switch` cases, `catch`, ternary, and `&&`/`||`.
- Python (tree-sitter): `if`/`elif`, `for`, `while`, `except`, `case`,
  ternary, and `and`/`or`.
- C/C++ (tree-sitter): `if`, `for`, `while`, `do while`, `case`/`default`,
  `&&`/`||`, ternary, and lambda definitions.

Anonymous functions (Go func literals, JS arrows/expressions, Python lambdas,
C++ lambdas) are emitted as first-class function units named
`<anonymous@Lstart>` with their enclosing function recorded, and are measured by
every metric (ADR 0015).

## Cognitive complexity

Per function, implementing the SonarSource model (ADR 0015): `+1` for each break
in linear flow (`if`, `else if`, `else`, ternary, `switch`, loops, `catch`,
`goto`, labeled `break`/`continue`), plus a nesting penalty per nesting level for
nesting-increasing structures, plus `+1` per sequence of like logical operators
(`&&`/`||`), plus `+1` per direct self-call (recursion). `else if` chains do not
add nesting, and indirect/mutual recursion is not counted.

## Function shape

Per function (ADR 0015, Sonar-like definitions):

- **Length** — non-comment physical lines.
- **Nesting depth** — maximum nesting of control structures (`if`, `for`,
  `while`, `switch`, `try`), excluding nested function bodies.
- **Parameter count** — declared parameters, excluding receiver/`self`.

Rolled up per file as `max_function_length`, `max_nesting`, and `max_params`.

## Comments

`comment_lines` counts **significant** comment lines: blank and purely
decorative comment lines are excluded, commented-out code counts. Comment lines
are disjoint from `physical_sloc` (comments are stripped from code tokens), so
`comment_pct = comment_lines / (physical_sloc + comment_lines)`; 50% means the
file has as many comment lines as code lines.

## Duplication

Duplication is detected over the **normalized token stream**: comments and
whitespace are removed; string/number literals are normalized to `STR`/`NUM`;
identifiers and keywords are kept verbatim. A duplicated block is an identical
token sequence of length ≥ 20 shared between two files or two distinct regions
of one file.

Reported as: total/duplicated tokens, duplicated-token percentage (`dup_pct`),
and the number of distinct maximal blocks. `duplicated_lines` counts the
physical lines spanned by those blocks.

## Dependency graph & coupling

Imports only create edges when they resolve to an **analyzed file** (ADR 0010):

- Go resolves through module import paths.
- TS/JS and Python use workspace-root best-effort resolution
  (see [ADR 0002](adr/0002-languages-and-staging.md)).
- C/C++ resolve `#include`: quoted includes relative to the including file then
  against `include_paths`; angle includes only against `include_paths`
  (ADR 0017).

For each file Navune computes:

- **Ca (afferent)** — internal files that depend on it.
- **Ce (efferent)** — internal files it depends on.
- **Instability** `I = Ce / (Ca + Ce)` — 0 is maximally stable.
- **Abstractness** `A = abstract types / types`.
- **Distance from main sequence** `|A + I − 1|` — the "zone of pain" indicator.

Cycles are strongly connected components (SCCs) of size > 1 in the internal
file graph; each is reported with its member files.

## Quality gate & the composite index

Budgets and weights come from `navune.yaml` (ADR 0009). Budgets are
**upper-bound-only**: a measured value greater than its limit breaches. The
metrics that ship with a default budget:

| Budget key | Meaning | Default limit | Default tier |
|---|---|---|---|
| `avg_complexity` | Mean cyclomatic complexity per function | 10 | error |
| `worst_complexity` | Worst single-function complexity | 50 | error |
| `max_duplication_pct` | Duplicated tokens as % of all tokens | 5 | warn |
| `max_cycle_members` | Largest dependency cycle (files) | 8 | error |
| `max_in_cycle_pct` | Files sitting in a dependency cycle | 10 | warn |

Additional budget keys are available **opt-in** (no default limit, ADR 0016):

| Budget key | Meaning |
|---|---|
| `avg_cognitive_complexity` / `worst_cognitive_complexity` | Cognitive complexity, per function |
| `max_function_length` | Longest function, non-comment physical lines |
| `max_nesting` | Deepest control-structure nesting |
| `max_params` | Largest parameter count |
| `max_duplicated_lines_pct` | Duplicated lines as % of physical lines |
| `max_issues` | Total issues raised by the smell layer |
| `max_blocker_issues`, `max_high_issues`, `max_medium_issues`, `max_low_issues`, `max_info_issues` | Issues by severity |

**Per-language budgets** live under `language_budgets` and use the same keys;
they are evaluated over that language's production files in addition to the
global budgets. The composite uses global values only. `comment-density` is a
lower-bound measure and therefore a smell rule, not a budget key.

The 0–100 **composite index** is a weighted, transparent blend of each metric's
distance from its budget:

```
metric_score = clamp(100 * (1 - (value / limit)), 0, 100)
composite    = Σ (weight_m × metric_score_m) / Σ weight_m
```

Budgets decide the verdict and exit code; the composite is informational
(ADR 0009).

## Smell rules

The smell layer (ADR 0016) evaluates ten deterministic rules over the measures
above and emits `issues`. All rules are enabled by default; none gate unless a
budget is configured. Severities use the MQR scale (`blocker`, `high`, `medium`,
`low`, `info`).

| Rule | Target | Default |
|---|---|---|
| `cognitive-complexity` | function | 15 |
| `cyclomatic-complexity` | function | 10 |
| `function-length` | function | 60 lines (Python 50) |
| `nesting-depth` | function | 4 (TS/JS 3) |
| `parameter-count` | function | 7 |
| `file-length` | file | 750 lines |
| `duplicated-file` | file | 10% of lines |
| `comment-density` | file | below 25%, files ≥ 50 SLOC |
| `boolean-complexity` | expression | more than 3 `&&`/`||` conditions |
| `too-many-methods` | type | 35 |

Per-language thresholds can be overridden under `language_smells`; see
[ADR 0016](adr/0016-deterministic-smell-layer.md) for the full defaults and the
configuration schema.

## Summary and per-language breakdown

The report's `summary` aggregates the per-file measures and adds
`issue_count`. `summary.by_language` repeats the summary fields per canonical
language key (`go`, `typescript`, `javascript`, `python`, `c`, `cpp`), excluding
cycle and coupling metrics, which are inherently cross-language. With
`--verbose`/`-v`, each file also carries a full `functions_detail` array (ADR 0016).

## Test files

A file is a test if its filename matches the language's pattern
(`*_test.go`; `*.test.ts`/`*.spec.ts`; `*_test.py`/`test_*`; C/C++
`test_*`/`*_test` with `.c/.cc/.cpp/.cxx/.h/.hpp/.hh`) **or** it sits under a
`test`/`tests` path component below the analysis root (ADR 0011). Test files are
analyzed into a separate `tests` namespace: reported, but excluded from budgets,
cycles, coupling, the composite, and the extended measures and smells
(ADR 0015, ADR 0016).
