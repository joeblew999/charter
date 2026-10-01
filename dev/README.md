# dev/: the tool the tasks run

Every task in `../mise.toml` is one line. Anything that needs more than one line is a command of this Go program (standard library only), so the logic is real code that can be read, tested and reused, not shell inside TOML.

```sh
go run ./dev help            # every command
mise run <task>              # what you normally type: each task is one line that calls a tool or a dev command
```

| Command | What it does | Task that calls it |
|---|---|---|
| `with-server` | Starts a server, waits until a URL answers, runs commands against it, stops it | `api-go:test:native`, `api-go:test:workerd` |
| `migrate-local` | Applies `migrations/*.sql` to a running dev server's local D1, each once (cf can't) | `api:migrate:local`, `api-go:migrate:local` |
| `migrate` | Finds the Worker's D1 database and applies pending migrations | `api:migrate`, `api-go:migrate`, the deploy tasks |
| `size` | Fails if a file is over a gzipped size (the Wasm limit) | `api-go:build` |
| `sdk-gen`, `sdk-check`, `sdk-ready`, `sdk-list`, `sdk-clean` | Fern: generate an SDK, prove it works, make what the tests need | `sdk:*` |
| `cli-build` | Builds the Rust CLI that Fern generates, natively or for Linux in Docker | `sdk:cli:build` |
| `harness-test`, `harness-deploy` | Fern's TypeScript SDK inside a Worker (`sdk/harness`) | `sdk:harness:*` |
| `upstream` | Lists every `Upstream: owner/repo#n` tag in the code with the issue's state | `upstream:status` |
| `doctor` | Checks the tools and installs the tasks need | `doctor` |
| `cloudflare-spec` | Slices Cloudflare products out of Forge's spec as a Fern API | `sdk:cloudflare` |

## Adding a task

1. If it's one command, write the one-line task in `mise.toml`.
2. If it isn't, add a command here (register it in an `init`, as the others do) and write a one-line task that calls it.

## Using it from another repo

The module path is real, so another repo can run it without copying it:

```sh
go run github.com/joeblew999/orpc-api/dev@latest help
```

It expects this repo's layout (`api/`, `api-go/`, `sdk/`, `migrations/`, a `go.work` at the root).
