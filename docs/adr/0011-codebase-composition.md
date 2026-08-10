# ADR 0011 — Codebase composition: generated/vendored skipped, tests separated

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q13)

## Context

Metric and gate semantics change a lot based on which files are in play. Test
files, generated code, and vendored dependency trees all distort structural
metrics if naively included — but excluding tests entirely hides a huge code
smell source (gigantic test helpers), while counting them punishes legitimate
test code.

## Decision

- **Generated and vendored code is skipped entirely** via per-language built-in
  exclusion patterns (`vendor/`, `node_modules/`, `*.pb.go`, `*.min.js`,
  lockfiles, build output, etc.), overridable in config.
- **Test files are analyzed but reported in a separate `tests` namespace**:
  - excluded from quality-gate budgets,
  - excluded from coupling/cycle/composite math (production code only),
  - reported separately so test-code complexity remains visible.
- Detection is language-aware: Go `*_test.go`; TS/JS `*.test.ts`, `*.spec.ts`,
  `__tests__/` (post-v1); plus user patterns in config.

## Consequences

- Production-code budgets measure production code; test hygiene is still
  visible in the report without being gated.
- Config schema needs an exclusion mechanism (built-in + user patterns).
- The analyzer must classify every file as production / test / skipped before
  metrics are computed.
