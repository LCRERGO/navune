# ADR 0018 — Packaged shell completion and man page

- Status: accepted
- Date: 2026-09-17
- Decision-maker: product owner via design interview

## Context

The CLI is hand-rolled (`cmd/navune/main.go`): a `switch` over subcommands and a
manual flag loop, with the help text as hardcoded string literals. There is no
framework (no Cobra) and therefore no metadata source from which a completion
script or a man page could be generated.

Distribution is `go install` plus the plain-matrix release archives from
[ADR 0014](0014-release-and-ci.md) (binary, `LICENSE`, `README.md`). There are
no package-manager channels (Homebrew, scoop, apt), so nothing installs shell
completion or a man page today. The CLI surface itself is deliberately minimal
and governed by [ADR 0012](0012-cli-surface.md).

## Decision

Ship a **bash completion** and a **man page** as committed, static artifacts:

- **Files.** `completions/navune.bash` and `man/navune.1` live at the repository
  root (not under `docs/`, which holds prose and ADRs).
- **Bash only.** One completion script for bash, matching the request. zsh/fish
  can be added later without reworking this decision.
- **Committed roff.** The man page is hand-written roff, edited directly. No
  `scdoc`/pandoc build-time generator: the project deliberately avoids extra
  toolchain (cf. ADR 0014's "plain matrix, not GoReleaser").
- **No new subcommands, no embedding.** The artifacts are not embedded in the
  binary and are not emitted through `navune completion`/`navune man`; the
  subcommand surface stays exactly as ADR 0012 defines it.
- **Install targets.** `make install-completions` and `make install-man` copy
  the files under `PREFIX` (default `/usr/local`), honoring `DESTDIR` for
  staging. The existing `make install` (a `go install`) is unchanged, since its
  `GOBIN` destination is unrelated to a system `PREFIX`. There is no umbrella
  target.
- **Release archives.** The publish job adds `completions/navune.bash` and
  `man/navune.1` to every `navune_<version>_<os>_<arch>.tar.gz`/`.zip`, so
  tarball users receive them too.
- **Drift guard.** Because the CLI has no metadata source, a test
  (`cmd/navune/artifacts_test.go`) asserts that every subcommand, flag, and
  `--format` value appears in both artifacts. It is the only automated link
  between the hand-rolled CLI and the packaged files.
- **Naming.** The repo file keeps the `.bash` suffix; it installs as
  `.../bash-completion/completions/navune` (extensionless, per bash-completion
  convention). The man page installs as `.../man/man1/navune.1`.

## Consequences

- `go install` users do not get the artifacts automatically (no package manager
  to place them); they copy them from the source tree or a release archive.
- The completion and man page are maintained by hand and can rot silently except
  for the drift-guard test, which covers commands, flags, and format values but
  not prose.
- Adding a subcommand or flag now touches four places: the CLI, its help text,
  `completions/navune.bash`, and `man/navune.1` — and the drift test fails until
  the artifacts follow.
- No build-time dependency, no binary growth, and no change to the CLI surface
  frozen by ADR 0012.
