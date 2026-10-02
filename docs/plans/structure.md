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

The layout, and the one shape every project has, are on the home page ([what is what](../README.md#what-is-what)) and in [Configuration](../reference/config.md#the-files-of-a-project).

## Done

- **The library is a module of its own,** and the Worker glue ships with it: the build writes it into `build/`, so no project keeps a copy that can fall behind.
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
