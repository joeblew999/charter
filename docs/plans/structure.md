---
title: Structure
nav_order: 1
parent: Plans
grand_parent: This repository
---
# Plan: one project shape, used everywhere

The repo grew as one example with tools around it, and three things are tangled: the reusable library lives inside the notes example's Go module, the examples are not in the shape a new project has (so `dev new` filters and renames this repo), and `mise.toml` is both this repo's tasks and the template. This plan untangles them. Backward compatibility is not kept.

## The target

```
go/         the Go library other repos import: humaworkers, transport, hub, d1, follow,
            humamcp, asyncapi, specfile, and the Worker glue (worker/*.mjs)
ts/         the TypeScript library, when there is one to publish (the AsyncAPI generator, follow)
examples/   notes-go, showcase-go, notes-ts, showcase-ts: each a complete project
dev/        the tool behind every task
docs/
```

A project, here and in any repo, has one shape. For Go:

```
mise.toml               the tool's pin and the tasks: setup, run, dev, build, spec, check, deploy, bench, sdk:gen ...
go.mod                  the project's module
main.go, platform_js.go, platform_other.go
api/                    the contract, the handlers, the store
cmd/spec/               writes the specs
worker.mjs              the Worker's entry: a few lines
cloudflare.config.ts, package.json
migrations/
fern/                   generators.yml and the generated specs
sdk/go/                 the generated Go client, committed
test/
```

- **Tasks have the same names in every project** (`check`, `deploy`, `bench`), and the dev tool works on the project it is run in. The repo's own `mise.toml` only runs each example's.
- **`dev new` copies `examples/notes-go`** and renames it. No filtering.
- **The Worker glue is not copied into a project.** It ships with the Go library, and the build writes it into `build/` from the library version the project uses, so the JavaScript and the Go that talk to each other always match.

## Steps

1. **The library on its own:** `go/` as a module, the glue shipped by the build. Done 2026-10-02: `go/` is the module `github.com/joeblew999/orpc-api/go` (the packages and `go/worker/*.mjs`), `api/go/` requires it, `dev wasm-build` writes the glue into `build/`, and each example has one entry file, `worker.mjs`.
2. **The examples as projects:** `examples/*`, each with its own `mise.toml`; the dev tool works on the current project; `dev new` copies. Not started.
3. **The docs, once, against the final shape.** Not started.
