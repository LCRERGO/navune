# ADR 0015 — Extended structural measures

- Status: accepted
- Date: 2026-09-17
- Decision-maker: product owner via design interview (Q7–Q9, Q17, Q20–Q22, Q29–Q30, Q52, Q61)

## Context

ADR 0005 fixed Navune's v1 metric model: size, cyclomatic complexity,
duplication, dependency graph, cycles, and Martin coupling. SonarQube's
structural surface is broader: it additionally reports cognitive complexity,
comment lines/density, function and file size, nesting depth, parameter count,
and duplicated lines.

Navune's existing model has no cognitive complexity; comments are stripped from
the token stream and excluded from physical SLOC, so nothing counts them; and
anonymous-function complexity is silently dropped — closures are neither
attributed to an enclosing function nor emitted as function units.

Closing the structural gap requires extending the element model and every
adapter, without touching the language-agnostic pipeline.

## Decision

Add the following measures.

- **Cognitive complexity** (per function), implementing the SonarSource model:
  `+1` for each break in linear flow (`if`, `else if`, `else`, ternary,
  `switch`, loops, `catch`, `goto`, labeled `break`/`continue`), plus a nesting
  penalty per nesting level for nesting-increasing structures, plus `+1` per
  sequence of like logical operators (`&&`/`||`), plus `+1` per direct
  self-call (recursion). `else if` chains do not add nesting. Indirect/mutual
  recursion is out of scope, matching Sonar. The full model lands in one
  change (Q52).
- **Comment lines and comment density** (per file): significant comment lines
  only (blank and decorative comment lines excluded; commented-out code counts).
  Density is `comment_lines / (physical_sloc + comment_lines)`.
- **Function length** (per function): non-comment physical lines.
- **Nesting depth** (per function): maximum nesting of control structures
  (`if`, `for`, `while`, `switch`, `try`), excluding nested function bodies.
- **Parameter count** (per function): declared parameters, excluding
  receiver/`self`.
- **Duplicated lines** (per file): physical lines spanned by duplicated token
  blocks.
- **Anonymous functions are first-class functions**: every lambda, closure, and
  arrow function is emitted as a `Function` named `<anonymous@Lstart>` with
  `Enclosing` set, and is measured by every metric. This is a deliberate
  behavior change: it corrects silently dropped complexity but shifts the
  `functions`, `avg_complexity`, and `worst_complexity` values.
- **`lang.Function` gains `Calls []string`** (direct callees), used for
  recursion and available for future call-graph work.

Test and generated files are excluded from these measures (ADR 0011).

## Consequences

- The adapter contract expands: each adapter must extract comments, function
  shape (length, nesting, parameters), callees, and anonymous functions, and
  compute cognitive complexity.
- Direct recursion only; mutual recursion is not counted (matches Sonar).
- Anonymous-function emission is a breaking behavior change for consumers that
  compare `functions` or complexity aggregates; it is documented in
  `CHANGELOG.md`.
- Comment lines and `physical_sloc` stay disjoint (comments are excluded from
  `physical_sloc`), so the two measures sum cleanly.
- The new measures feed the smell layer (ADR 0016) and are budgetable opt-in
  (ADR 0009 revision 1).
