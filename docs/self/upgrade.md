---
title: Moving from an older charter
nav_order: 11.5
---

# Moving from an older charter

charter is one command now, `charter`, taken from mise; a repo's settings and a project's are each one `charter.toml`. What to change depends on what the repo has. In every case, first:

```sh
mise use github:joeblew999/charter   # charter, pinned in the repo's mise.toml
```

Then `charter init` writes charter's files again, and `charter check` names whatever is still the old way. A file charter-ts or an older charter wrote (its first lines say so) is written over; a file of the repo's own never is.

## A repo on charter's Go tool before 0.20 (mise.toml pins 0.9 to 0.19)

| Then | Now |
|---|---|
| `description`, `topics`, `homepage`, `docs`, `renovate`, `workflow` at the top of `charter.toml` | The same under `[github]` (`workflow` is `workflows`); charter refuses the old keys, naming them |
| `projects` at the top of `charter.toml` | Gone: a job's projects are every folder with a `charter.toml` naming a Worker, or its own `projects` |
| The tool's mise tasks (`mise run repo`, `repo:check`, `repo:ci`…) | Delete them; `charter init` writes `.mise/tasks/charter.toml` (`mise run charter:<command>`) |
| Its workflows | Delete them; `charter init` writes charter's (`charter-ci.yml`, `charter-repo.yml`, one per job of `[ci.jobs]`) |

## A Go app on the library at `go/lib` or `go`

```sh
go get github.com/joeblew999/charter@latest && go mod tidy
```

| Then | Now |
|---|---|
| `github.com/joeblew999/charter/go/lib/<package>` | `github.com/joeblew999/charter/<package>` |
| `github.com/joeblew999/charter/go/<package>` | The same without `/go`; `humamcp` is `mcp`, `specfile` is `specs` |
| `go tool charter` (a `tool` line in `go.mod`) | `charter`, from mise: delete the `tool` line. The library's version and charter's are one, which `charter check` holds them to |

## A repo on charter-ts (npm scripts that run charter-ts)

Every step is charter's now, from `charter.toml`. In a project, write it from the two configs, as this repo's `examples/*-ts` were (each folder's history shows the change), then delete `charter.config.ts`, `cloudflare.config.ts`, `vite.config.ts` and `secrets.ts`.

| `charter.config.ts`, `cloudflare.config.ts` | `charter.toml` |
|---|---|
| `worker`, `name`; `compatibilityDate`, `compatibilityFlags` | `name`; `compatibility_date`, `compatibility_flags` |
| `url`, `port`, `specServer` | `url`, `port`, `spec_server` |
| `entrypoint` (`worker.ts`) | `[build] entry = "worker.ts"` |
| `bindings.d1()` | `[[d1]] binding = "DB"`, `migrations = "migrations"`, `database = "<worker>-db"` (the one cf made: a deploy finds it, and runs no migration again) |
| `bindings.durableObject` with `exports` | `[[durable_objects]] binding`, `class` |
| `rateLimits(spec, …)` | `spec = "fern/openapi.json"`: each limit of the contract, counting in the same counter as before |
| `bindings.text`; `secrets`, `optionalSecrets`; `localSecrets` | `[production] vars`; `[secrets] required`, `optional`; `[local] vars` |
| `sdks`, `sdkInWorker`, `db` | `[sdks] name`, `more`, `audiences`, `in_worker`; `db` |
| `test` (`run`, `env`, `serve`), `live` | `[test] run`, `env`, `serve`, `live`: commands, `{base}` the Worker's URL, `{port1}`… |
| `deployModes` (the same code as a second Worker) | `[production] also = ["<the other Worker>"]` |

In every repo on charter-ts:

| Then | Now |
|---|---|
| `package.json`: the scripts that run `charter-ts`, and `charter-ts`, `cf`, `vite` in `devDependencies` | Delete them (`charter check` names a script left) |
| `package.json`: `description`, `keywords`, `homepage` | `charter.toml`'s `[github]`: `description`, `topics`, `homepage` |
| charter-lib from `releases/download/ts-vX.Y.Z/` | From the release `vX.Y.Z`: `https://github.com/joeblew999/charter/releases/download/v0.20.0/charter-lib-0.20.0.tgz` |
| `ci.config.ts` | `charter.toml`'s `[ci.jobs.<name>]`: `run` as commands, `charter <command>` for `charter-ts <command>` |
| `tsconfig.json` extending `charter-ts/tsconfig.worker.json` | Its own options, `types: ["@cloudflare/workers-types"]` (a `devDependency`), and `.charter/env.d.ts` included: the Worker's `env`, typed from `charter.toml` |
| `npm run <command>` | `charter <command>`, or `mise run charter:<command>` |
