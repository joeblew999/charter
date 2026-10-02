---
title: Releases
nav_order: 5
parent: Reference
---

# Releases: what one contains, and how a project updates

What a release of charter is, its files and tags, how a project moves to a new one, and what is promised between versions. Read it before you pin or update the tool or the library.

The newest release: [github.com/joeblew999/charter/releases/latest](https://github.com/joeblew999/charter/releases/latest)

## Get the latest

```sh
go run github.com/joeblew999/charter/cmd/charter@latest help    # the tool, no install (needs Go)
go get github.com/joeblew999/charter/go@latest                  # the library, in a project
curl -fsSL https://github.com/joeblew999/charter/releases/latest/download/charter_darwin_arm64.tar.gz | tar xz charter   # the tool as a binary
```

Write `@latest` and `releases/latest` in anything you keep: a link with a version in it is out of date after the next release.

## What a release contains

One GitHub Release per version tag. No file name carries the version.

| File | What it is |
|---|---|
| `charter_linux_amd64.tar.gz`, `charter_linux_arm64.tar.gz`, `charter_darwin_amd64.tar.gz`, `charter_darwin_arm64.tar.gz` | The tool as a binary, with a `README.md` |
| `checksums.txt` | The SHA-256 of those archives |
| `notes-go-sdk-go.tar.gz`, `notes-go-sdk-typescript.tar.gz`, `notes-go-specs.tar.gz`, `notes-go-cli-linux-amd64` | The Go notes example's SDK sources, specs and CLI, generated fresh and checked |
| `notes-ts-sdk-go.tar.gz`, `notes-ts-sdk-typescript.tar.gz`, `notes-ts-specs.tar.gz`, `notes-ts-cli-linux-amd64` | The same for the TypeScript notes example |

A project uses the tool and the library. The library is not a file of the Release: Go fetches it from the repository at the release's tag. The example files show what your own project's `release` workflow attaches for your API ([Release](../guides/sdks.md#release)).

Not in a release: a Windows build of the tool (it manages process groups, which is Unix-only), SDK packages, the CLI for anything but Linux on amd64.

## The tags

| Tag | What it versions |
|---|---|
| `vX.Y.Z` | The release, and the tool: the module `github.com/joeblew999/charter`. The maintainer pushes this one |
| `go/vX.Y.Z` | The library: the module `github.com/joeblew999/charter/go`, with the Worker glue |
| `examples/notes-go/sdk/go/vX.Y.Z` | The Go SDK of the notes example |

Go finds a module in a subdirectory only under a tag that starts with the directory. The `release` workflow adds the last two on the commit of `vX.Y.Z` (`mise run release:tags`). A version with a hyphen (`v1.2.3-rc.1`) is a pre-release.

## How a release is cut

1. **Before the tag:** `mise run check` at the root, and `charter new` into an empty folder followed by that project's own `mise run check`. No workflow runs the second.
2. **Push the tag** `vX.Y.Z`. Never upload release files or push the module tags by hand.
3. **The `release` workflow** builds the tool with GoReleaser (`mise run charter:release`), tags the modules, and for each notes example runs `sdk:dist`, `sdk:dist:cli` and `release`.

On a pull request the same workflow is a dry run: the same builds, kept as workflow artifacts.

## How a project updates

A project made by `charter new` depends on a release in two places.

```sh
mise up --bump "go:github.com/joeblew999/charter/cmd/charter"         # the tool: the pin in mise.toml
go get -u github.com/joeblew999/charter/go && go mod tidy             # the library: the pin in go.mod
mise run workflows                                                    # the workflows, as the new tool writes them
mise run check                                                        # prove the project against both
```

| Comes with the update | Does not: it is yours |
|---|---|
| The tool's commands, and so what each task does | The tasks in `mise.toml` |
| TinyGo and its patches, which the tool pins | The contract, handlers and store |
| The Worker glue in `build/`, written from the library version in `go.mod` | `worker.mjs`, `platform_js.go`, `platform_other.go`, `./cmd/spec` |
| The workflow templates, the docs config, `docs/writing.md` | The test programs in `test/` |

A fix to a copied file reaches your project only if you copy it: compare with `examples/notes-go/` when a release's notes mention one.

## What is promised

**No compatibility.** The releases are `v0`: exported names, commands, flags, tasks and templates can change between two of them. The release notes are generated from the commits. Pin exact versions, update on purpose, run `mise run check` after.

| The checks guarantee, for a release's commit | They do not |
|---|---|
| The library passes its tests and vets for Wasm | That a new project's full `mise run check` passes: that is run by hand |
| Each example passes its local check: TinyGo build, live and MCP tests natively and under workerd | That anything works on Cloudflare: the live tests and the soak are run by hand |
| The SDKs in the release were generated from that commit's specs and passed their checks | That the tag is on a green commit: the workflow builds what is there |
| A project scaffolded by `charter new` builds under its own name (a test of the tool) | |

## Before the rename

The releases cut while the repository was called orpc-api have other module paths and another layout. `@latest` under `github.com/joeblew999/charter` resolves once a release is tagged from this layout. Until then, make a project from a checkout: `go run ./cmd/charter new -name <name> -into <dir>`.
