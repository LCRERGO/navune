# ADR 0006 — File is the unit of analysis everywhere

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q11)

## Context

Coupling metrics (Ca/Ce, instability, abstractness) and cycle detection need a
definition of "a module". Across languages the natural boundaries differ: a Go
package, a Python file-as-module, a TS module, a C translation unit. Some tools
model components at package granularity. C/C++ in particular has no modules or
classes — only translation units joined by `#include`, which is genuinely
ill-defined territory that every serious tool resolves differently.

## Decision

**File is the unit of analysis across all languages**, including C/C++:

- Dependency edges come from imports (`import`, `require`/`import`, `#include`).
- Functions, types/classes, and globals form the second layer beneath files.
- C files have no "type" layer; C++ files do (classes/structs).
- Ca/Ce, instability, abstractness, and cycles are computed at file granularity.

For Go specifically, package-level imports are flattened to file granularity
(a file importing a package edges to each analyzed file of that package). This
deliberately inflates coupling relative to a package-level graph; it is
documented behavior, chosen for a single consistent model across languages.

## Consequences

- One uniform element model (`file → type/class → function/method`) drives every
  language adapter, so metric code is written once.
- C/C++ file coupling is coarser than package-level coupling and is documented
  as such; still useful for catching circular `#include` / layering violations.
- An aggregation step (e.g., roll files up to directories) can be added later
  without disturbing the model.
