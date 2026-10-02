---
title: The restructure
nav_order: 7
parent: How to help
---
# The restructure: one project shape, used everywhere

A record of what the repository was turned into on 2026-10-02 ([#17](https://github.com/joeblew999/charter/issues/17)), and what is left of it. Backward compatibility was not kept.

## Why

The repo grew as one example with tools around it, and three things were tangled: the library lived inside the notes example's Go module, the examples were not in the shape a new project has (so the scaffold filtered and renamed the whole repo), and one `mise.toml` was both the repo's tasks and the template.

## What it is now

```
go.mod, cmd/charter/     the tool: the command charter
go/                      the library: its packages, and the Worker glue (worker/*.mjs)
examples/notes-go/       the Go notes API: a complete project
examples/showcase-go/    every Fern feature, in Go
examples/notes-ts/       the notes API in TypeScript, on oRPC
examples/showcase-ts/    every Fern feature in TypeScript, and the SDK inside a Worker
docs/
mise.toml                the repo's own tasks
```

A project, here and in any repo, has one shape:

```
mise.toml               the tool's pin and the tasks
go.mod, main.go, platform_js.go, platform_other.go
api/                    the contract, the handlers, the store
cmd/spec/               writes the specs
worker.mjs              the Worker's entry: a few lines
cloudflare.config.ts, package.json
migrations/
fern/                   generators.yml and the generated specs
sdk/go/                 the generated Go client, committed
test/
```

## Done

- **The library is a module of its own:** `github.com/joeblew999/charter/go`.
- **The Worker glue ships with the library.** The build writes it into `build/` from the library version a project uses, so no project keeps a copy that can fall behind.
- **Each example is a complete project** with its own `mise.toml`, `package.json` and `fern/` folder.
- **Task names are the same in every project,** with no prefix. The root `mise.toml` only runs each example's (`charter each`).
- **The tool works on the project it is run in,** and is called `charter`: the repo's root module.
- **`charter new` copies `examples/notes-go/`** and renames it. Nothing is filtered.
- **One set of workflow templates** serves a repo that is one project and a repo that holds several.
- **The repository is called charter.** Workers are `charter-notes-go`, `charter-notes-ts`, `charter-showcase-go`, `charter-showcase-ts`; the SDKs are `Notes` and `Showcase`.
- **The docs were rewritten once,** against this shape ([#18](https://github.com/joeblew999/charter/issues/18)).

## Left

- **A release from this layout.** Until one is tagged, `@latest` does not resolve to it ([Releases](../reference/releases.md#before-the-rename)).
- **`ts/`:** a TypeScript library, when there is one to publish (the AsyncAPI generator, the feed). Today the showcase imports them from `examples/notes-ts/`.
- **The projects made from earlier releases** move to this shape by hand ([#20](https://github.com/joeblew999/charter/issues/20)).
