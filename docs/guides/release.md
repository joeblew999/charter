---
title: Release
nav_order: 5
parent: Guides
---

# Release from your machine

From the project's folder, in minutes:

```sh
mise run release -- vX.Y.Z -dry-run    # every check and the build; changes nothing
mise run release -- vX.Y.Z
```

It stops unless the tree is clean, `HEAD` is the default branch on GitHub, every workflow GitHub ran on that commit passed (on every OS it runs; a repo with no workflows skips this) and the tag is new. Then:

1. `setup` then `check`, as CI runs them.
2. `spec:diff`: a breaking change to the specs since the last release stops it unless the tag is a major release (in v0, the minor number) ([Repos that use each other](repos.md)).
3. `dist` into an empty `dist/`: the SDKs, the specs and the CLI.
4. The tag, pushed.
5. The GitHub Release. Its notes are the commit subjects since the last tag; `-notes-footer <text or file>` adds a line, `-prerelease` marks it.
6. `release:publish` attaches `dist/` and writes `SHA256SUMS`; `release:tags` tags `sdk/go/vX.Y.Z`.

A repo with no `dist` task (one that ships no files) gets the same, without files. Nothing goes to a package registry. A new project's release took about four minutes on an Apple silicon Mac (3 Oct 2026), most of it the six CLI builds.

The `release` workflow then builds it all again on the tag and attaches only what the Release lacks; `check` runs on the tag on Linux, Windows and macOS. A tag pushed by hand gets its Release from the workflow, without the darwin CLI.

## The CLI for every OS

`sdk:dist:cli` builds `dist/<project>-cli-<os>-<arch>` (`.exe` on Windows) for `darwin`, `linux` and `windows` on `amd64` and `arm64`: darwin with Apple's linker, so only on a Mac; the others with zig (`cargo-zigbuild`, pinned in `mise.toml`) from any machine. Linux is a static musl binary. `-- -target linux-arm64,...` builds fewer. Install it with mise, `bin` being `binaryName` in `fern/generators.yml`:

```sh
mise use "github:<owner>/<repo>[bin=<binary>]"
```

Proven on 3 Oct 2026 from a release cut on that Mac: `mise` installed and ran darwin-arm64, and both Linux binaries ran under Docker (Alpine and Debian). Windows amd64 is started on Windows by the `release` workflow's `cli-windows` job. darwin-amd64 and windows-arm64 have not been started on a machine of their kind yet.
