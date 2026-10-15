---
title: Repos that use each other
nav_order: 10
---

# Repos that use each other

A repo pins what it takes from another, and the pins are the only record. Renovate moves them when there is a release.

| What | Pinned in | How |
|---|---|---|
| charter-ts | `package.json`, `devDependencies` | The URL of its GitHub Release, version and all (`ts-vX.Y.Z`) |
| charter-lib (a TypeScript Worker) | `package.json`, `dependencies` | The same, from the same release |
| The Go library (a Go Worker) | `go.mod` | `github.com/joeblew999/charter/go/lib`, at its tag `go/lib/vX.Y.Z`, which every release has |
| A Go SDK another project publishes | `go.mod` | The module path of its `sdk/go`, at a commit or a tag |
| Tools | `mise.toml` | Exact versions |

## Renovate

`npm run adopt` writes `renovate.json`, charter's own. It does nothing until the [Renovate app](https://github.com/apps/renovate) is installed on the repo: the file is its settings, not the bot. Then:

| It watches | What it does |
|---|---|
| `mise.toml` tools, `go.mod`, `package.json` | Opens a pull request when a pinned project releases (managers `mise`, `gomod`, `npm`) |
| charter-ts's release URL in `package.json` | The same, from charter's releases tagged `ts-v*` |
| Minor and patch updates | Merges them once the checks pass; a major one waits for a person |
| charter's own | One pull request for charter-ts, charter-lib and the Go library together |

`"charter": { "renovate": false }` in `package.json` leaves the file to the repo. Moving to a release by hand: what `docs/charter/README.md` says, and in Go, `go get github.com/joeblew999/charter/go/lib@vX.Y.Z && go mod tidy`.

## Breaking changes

A contract change that removes or renames an operation, a field or a parameter breaks the SDKs another repo pinned. Before a release, compare the specs with the last one's: `git diff <last tag> -- fern/`.
