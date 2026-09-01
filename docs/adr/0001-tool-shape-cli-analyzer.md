# ADR 0001 — Navune is a CLI analyzer, not a server

- Status: accepted
- Date: 2026-09-08
- Decision-maker: product owner via design interview (Q1)

## Context

Quality-analysis tooling comes in two broad shapes: long-running servers with
history storage, a web dashboard, and quality-gate orchestration; or
IDE/tooling-centric products. Building "one of those" could mean a daemon with
persistence and a UI, or a self-contained analyzer. A server's
storage/history/UI problem dwarfs the analysis problem itself and would bury
the interesting part.

## Decision

Navune v1 is a **command-line analyzer**: `navune analyze <path>` parses a
codebase, computes metrics, prints a report, and exits with a quality-gate
status code. There is no daemon, no database, no built-in web server in v1.

## Consequences

- Single distributable binary; trivial to run locally and in CI.
- No persistence layer: each run is a fresh analysis of the on-disk state.
- The analysis core must be designed as a reusable pipeline so a future server
  or IDE wrapper can consume it without rework.
- Historical trending (server-style dashboards over time) is explicitly out of
  scope for v1.
