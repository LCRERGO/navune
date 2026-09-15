# ADR 0014 — Releases: GitHub Actions, native cgo runners, plain build matrix

- Status: accepted
- Date: 2026-09-15
- Decision-maker: product owner via design interview

## Context

Navune requires cgo (tree-sitter grammar bindings), so release binaries cannot
be produced with `CGO_ENABLED=0`. The canonical repository is on Codeberg, with
an automatic push mirror to GitHub (`github.com/LCRERGO/Navune`). Codeberg's
hosted CI runners are Linux/amd64 only, so a Codeberg-native pipeline would have
to cross-compile the macOS and Windows targets with `osxcross` (plus a macOS SDK)
and `mingw-w64` — fragile and legally awkward.

GitHub-hosted runners, by contrast, are native per OS (`ubuntu-latest`,
`ubuntu-24.04-arm`, `macos-latest`, `windows-latest`, the latter shipping a
mingw-w64 toolchain), which removes cross-compilation entirely. GoReleaser's
multi-runner "split and merge" is a Pro (paid) feature, so OSS GoReleaser cannot
fan out a single release across native OS runners.

## Decision

Releases are built and published by **GitHub Actions** on the GitHub mirror.

- **Workflows.** `.github/workflows/ci.yml` runs gofmt/vet/test on pushes to
  `main` and on pull requests. `.github/workflows/release.yml` runs on `v*` tags
  (real release) and on manual `workflow_dispatch` (dry-run).
- **Native build matrix.** One job per target on a native runner:
  `ubuntu-latest` → linux/amd64, `ubuntu-24.04-arm` → linux/arm64,
  `macos-latest` → darwin/arm64 **and** darwin/amd64 (Go sets `-arch`; a single
  runner builds both), `windows-latest` → windows/amd64. `CGO_ENABLED=1`,
  `-trimpath`.
- **Plain matrix, not GoReleaser.** Each job builds its binary and uploads it as
  a workflow artifact; a single `ubuntu-latest` publish job downloads them all
  and does archiving, checksums, and release creation in one place.
- **Artifacts.** `navune_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows),
  containing the binary, `LICENSE`, and `README.md`; plus a `checksums.txt`
  (sha256) over all archives.
- **Version injection.** `version`, `commit`, `date`, and `builtBy` are `var`s
  in `package main`, set via `-ldflags "-X main.version=…"`. The version is the
  tag with the leading `v` stripped; manual dry-runs use
  `git describe --tags --always --dirty`.
- **Quality gate.** gofmt, `go vet`, and `go test` must pass before any build.
- **Changelog.** `CHANGELOG.md` is hand-maintained in Keep-a-Changelog format;
  the publish job extracts the section matching the tag and **fails** if it is
  absent. The same text is the release body.
- **Publishing.** `gh release create` (no third-party action), with the release
  published immediately and `--prerelease` for hyphenated tags. Re-running a
  published tag fails loudly rather than mutating the release.
- **No package-manager channels** in v1 (no Homebrew, scoop, install script);
  downloadable archives are the distribution channel.

## Consequences

- No C cross-toolchains or macOS SDK licensing concerns: every binary is built
  on its own OS.
- The release logic is a small, readable amount of workflow YAML rather than a
  GoReleaser config plus a Pro license.
- GitHub is required for releases even though Codeberg is the source of truth;
  the workflows live in the repository and reach GitHub through the mirror.
- A release requires a matching `CHANGELOG.md` entry, so notes can never be
  empty by accident.
- Dual-publishing assets to Codeberg remains possible later via the Gitea API
  and a `GITEA_TOKEN`.
