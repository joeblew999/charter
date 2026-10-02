---
title: Releases
nav_order: 5
parent: Reference
---

# Releases: what one contains, and how a project updates

What a release of charter is, its files and tags, how a project moves to a new one, and what is promised between versions. Read it before you pin or update the tool or the library.

The newest release: [github.com/joeblew999/charter/releases/latest](https://github.com/joeblew999/charter/releases/latest)

## What a release contains

One GitHub Release per version tag. No file name carries the version.

| File | What it is |
|---|---|
| `charter_linux_amd64.tar.gz`, `charter_linux_arm64.tar.gz`, `charter_darwin_amd64.tar.gz`, `charter_darwin_arm64.tar.gz` | The tool as a binary, with a `README.md` |
| `checksums.txt` | The SHA-256 of those archives |
| `notes-go-sdk-go.tar.gz`, `notes-go-sdk-typescript.tar.gz`, `notes-go-specs.tar.gz`, `notes-go-cli-linux-amd64` | The Go notes example's SDK sources, specs and CLI, generated fresh and checked |
| `notes-ts-sdk-go.tar.gz`, `notes-ts-sdk-typescript.tar.gz`, `notes-ts-specs.tar.gz`, `notes-ts-cli-linux-amd64` | The same for the TypeScript notes example |

- **A project uses the tool and the library.** The library is not a file of the Release: Go fetches it from the repository at the release's tag.
- **The example files** are what your own project's `release` workflow attaches for your API ([Release](../guides/sdks.md#release)).
- **Not in a release:** a Windows build of the tool, SDK packages, the CLI for anything but Linux on amd64.
- **Write `@latest` and `releases/latest`** in anything you keep: a version in a link is out of date after the next release.

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

The checks guarantee, for a release's commit: the library passes its tests; each example passes its local check (the TinyGo build, the live and MCP tests natively and under workerd); the SDKs in the release were generated from that commit's specs and passed their checks; a project scaffolded by `charter new` builds under its own name.

They do not guarantee: that a new project's full `mise run check` passes, or that anything works on Cloudflare (both are run by hand), or that the tag is on a green commit.

## Before the rename

The releases up to v0.7.0 were cut while the repository was called orpc-api: they have other module paths and another layout. From v0.8.0 the paths are the ones on this page, and `@latest` under `github.com/joeblew999/charter` resolves to them.

### Moving a project made before v0.8.0

Such a project has an `api-go/` folder, copies of the Worker glue in `api-go/worker/`, tasks named `api-go:*`, and imports `github.com/joeblew999/orpc-api/api-go/...`. Too much changed to edit it in place. Make a new project and move what is yours into it:

1. **Make the new project** beside the old one, with the old one's name, module and subdomain:

   ```sh
   go run github.com/joeblew999/charter/cmd/charter@latest new -name <name> -module <module> -subdomain <yours> -into <new dir>
   ```

2. **Move your API:** the old `api-go/api/` (contract, handlers, store, tests) replaces the new `api/`. In it, change the imports from `github.com/joeblew999/orpc-api/api-go/<package>` to `github.com/joeblew999/charter/go/<package>`, and your own from `<module>/api-go/api` to `<module>/api`.
3. **Move your data and bindings:** `migrations/`, and what you added to `platform_js.go`, `platform_other.go` and `cloudflare.config.ts`. Keep the new `worker.mjs` and `main.go`: the glue you had in `api-go/worker/` is now written into `build/` by the build.
4. **A feed of your own type** uses the library: `hub.DurableObject[T]("HUB", "<name>")` and `hub.Memory[T]`, in place of a copied hub. The Durable Object class is exported as `Hub`; if your deployed Worker declares it under another name, keep that name in `worker.mjs` (`export { Hub as <YourName> }`) and in `cloudflare.config.ts`.
5. **The store on Cloudflare** is a `D1Store` on the library's `d1` package (`d1.Query[T]`), as in the new `api/store_js.go`: copy its shape for your tables.
6. **Regenerate and check:** `mise install && mise run setup && mise run spec && mise run check`.
7. **Deploy and measure:** `mise run deploy`, `mise run live-test`, `mise run bench`. The Worker keeps its name, so it replaces the old deployment and keeps its database.

Then copy over `docs/`, `.github/` settings of your own, and the git history if you want it (move the new files into the old repository instead of the other way round).
