# ADR 0016 — Deterministic smell layer

- Status: accepted
- Date: 2026-09-17
- Decision-maker: product owner via design interview (Q1, Q10–Q11, Q15–Q19, Q25–Q47, Q53–Q61)

## Context

SonarQube's headline surface is issues (rules) and quality gates. ADR 0003
excluded code smells and ADR 0009 rejected opaque industry formulas, leaving
Navune purely descriptive. That is a real gap for users who want actionable
findings, not just measurements.

A **deterministic, structural** rule layer over the measures of ADR 0015 closes
the gap without turning Navune into a bug/security scanner: every rule is a
threshold predicate over a structural measure, not a pattern-matching
heuristic, and every finding is reproducible from the numbers already reported.

## Decision

### Issues

A new `internal/smell` package evaluates rules and emits
`Issue{rule, file, line, function, severity, value, threshold}`. The report
gains an additive top-level `issues[]` array and `summary.issue_count`; the text
report gains an issues section; Mermaid is unchanged. Issues are production-only
and deterministically sorted by (path, line, rule).

### Rules

Ten rules, all enabled by default. None gate unless budgeted.

| Rule | Target | Severity | Default |
|---|---|---|---|
| `cognitive-complexity` | function | high | 15 |
| `cyclomatic-complexity` | function | medium | 10 |
| `function-length` | function | high | 60 lines |
| `nesting-depth` | function | medium | 4 |
| `parameter-count` | function | medium | 7 |
| `file-length` | file | low | 750 lines |
| `duplicated-file` | file | medium | 10% of lines |
| `comment-density` | file | low | below 25% |
| `boolean-complexity` | expression | medium | more than 3 conditions |
| `too-many-methods` | type | low | 35 |

- `comment-density` is a lower-bound rule and is intentionally **not** a budget
  key: budgets are upper-bound-only (ADR 0009). It applies only to files with at
  least 50 physical SLOC.
- `boolean-complexity` flags a single boolean expression with more than three
  `&&`/`||` conditions and is line-localizable.
- `duplicated-file` uses duplicated lines as a share of physical lines, distinct
  from the token-based `max_duplication_pct` budget.

### Per-language defaults

Defaults vary per language only where there is a defensible rationale; all cells
are overridable.

| Rule | go | ts | js | py | c | c++ |
|---|---|---|---|---|---|---|
| `cognitive-complexity` | 15 | 15 | 15 | 15 | 15 | 15 |
| `cyclomatic-complexity` | 10 | 10 | 10 | 10 | 10 | 10 |
| `function-length` | 60 | 60 | 60 | 50 | 60 | 60 |
| `nesting-depth` | 4 | 3 | 3 | 4 | 4 | 4 |
| `parameter-count` | 7 | 7 | 7 | 7 | 7 | 7 |
| `file-length` | 750 | 750 | 750 | 750 | 750 | 750 |
| `duplicated-file` | 10% | 10% | 10% | 10% | 10% | 10% |
| `comment-density` | 25% | 25% | 25% | 25% | 25% | 25% |
| `boolean-complexity` | 3 | 3 | 3 | 3 | 3 | 3 |
| `too-many-methods` | 35 | 35 | 35 | 35 | 35 | 35 |

### Severity and gating

Severities use Sonar's MQR scale: `blocker`, `high`, `medium`, `low`, `info`.
Severities are informational. Gating is opt-in via `budgets`: `max_issues` plus
`max_blocker_issues`, `max_high_issues`, `max_medium_issues`, `max_low_issues`,
`max_info_issues`. No new default budgets ship, so existing runs and composites
are unchanged.

### Configuration

```yaml
smells:
  cognitive-complexity: { enabled: true, threshold: 15, severity: high }
  parameter-count:      { enabled: false }

language_smells:
  python:
    function-length: { threshold: 50 }
    nesting-depth:   { threshold: 4 }
```

`smells` is keyed by rule id and holds `enabled`, `threshold`, and `severity`.
`language_smells` is keyed by language and holds full rule objects. Precedence is
`language_smells` > `smells` > built-in per-language default, merged
field-by-field: an entry inherits any field it omits from the resolved global
entry. Config `version` stays `1`; the new blocks are additive.

### Per-language budgets

`language_budgets` is keyed by language and holds the same budget objects as
`budgets`:

```yaml
language_budgets:
  python:
    avg_complexity: { limit: 8, tier: warn }
```

Per-language budgets are evaluated over that language's production files **in
addition to** the global budget. The composite uses global values only.
`BudgetResult` gains a `language` field (`""` = global); language results appear
in the same `budgets` array, and the text report groups them under their
language.

### Report additions

Per file: `lang` (canonical key), `cognitive_complexity` (max),
`avg_cognitive_complexity`,
`comment_lines`, `comment_pct`, `max_function_length`, `max_nesting`,
`max_params`, `duplicated_lines`. Summary gains the same aggregates,
`issue_count`, and `by_language` — a map from canonical language key to the
summary fields, excluding cycle/coupling metrics (which are inherently
cross-language). JSON `schema_version` stays `1`; all additions are additive.

### CLI

`--verbose` / `-v` on `analyze` adds a full `functions_detail` array to every file in
JSON and prints functions carrying at least one smell in the text report.
`-V` becomes the short alias for `version` (a breaking CLI change, documented in
`CHANGELOG.md`). Language keys are canonical (`go`, `typescript`,
`javascript`, `python`, `c`, `cpp`); config accepts the aliases `ts`, `js`, and
`py`, normalized on load.

### Test classification

Directory-based test detection is added for **all** languages: any file under a
`test` or `tests` path component (strictly below the analysis root, exact
lowercase) is a test, unioned with the existing filename patterns. See ADR 0011
revision 1.

## Consequences

- Navune reports actionable, located issues without a hand-written bug database;
  every rule is a threshold over a measure defined in ADR 0015.
- Gating is opt-in, so adopting the new measures does not change existing exit
  codes or composites.
- `internal/smell` is language-agnostic; adapters only supply measures.
- The directory-based test rule is a behavior change for every language
  (ADR 0011).
- The gate model stays upper-bound-only, which is why `comment-density` is a
  rule rather than a budget.
