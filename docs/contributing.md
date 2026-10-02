---
title: How to help
nav_order: 6
has_children: true
---
# How to help

Three ways in: report something, make it faster, or pick up an issue. Everything runs through `mise`, the same on your machine and in CI.

## Set up

```sh
git clone https://github.com/joeblew999/charter && cd charter
mise install && mise run setup    # tools, then every example's npm packages
mise run check                    # everything, locally: needs Docker, no Cloudflare account
```

The repo is a library (`go/`), a tool (`cmd/charter/`) and four example projects (`examples/`). Each example is a complete project with its own `mise.toml`; `cd` into one and the same task names work there (`check`, `dev`, `deploy`, `bench`). `mise run doctor` says what is missing.

## Report a bug or ask for a feature

```sh
gh issue create --repo joeblew999/charter --title "[bug] ..." --body-file body.md
```

A report needs the exact command, its full output, and what you expected. If the bug is in TinyGo, workers-go, Fern, Huma or oRPC, check [Upstream issues](upstream.md) first: those are fixed there.

## Make it faster

Performance work here is a loop of about 70 seconds, and it does not touch the deployed Workers. You need a Cloudflare account (`./node_modules/.bin/cf auth login` in the example) and, for CPU figures, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in your environment.

```sh
cd examples/notes-go
mise run perf:try -- -name myidea                    # build, deploy to a scratch Worker of its own, bench, delete
mise run perf:try -- -name heap4 -build '-heap 4'    # the same with other build flags
```

It prints the CPU time Cloudflare recorded for every request, in the order sent, and whether a request was served by a reused Go runtime (no letter), one started ahead (`w`) or one it had to start (`n`):

```
GET /api/hello      0 0 0 0 0 1 0 0 0 0 1 0 0
POST /api/notes     2 2 3 2 2 2 2 4 2 2 3 3 2
```

- **Read each request, not the median.** Every finding so far came from a pattern in that line: every second request costing double, one in three starting a runtime, the first request in a new isolate.
- **Compare against a baseline from the same hour.** Run it once before your change.
- **`mise run compare`** (at the repo root) runs the same bench against the TypeScript notes Worker and the Go one: the target is that they cost the same.
- **Several people can run experiments at once:** each `-name` is its own Worker and database. `mise run perf:clean` deletes any that were left.
- **Then prove it is still correct:** `mise run check`, and for anything that touches streams, the soak (`mise run soak`).

The flags are in [Measure and improve performance](guides/performance.md). What has been found is in [Benchmarks](benchmarks.md), what is left in [the performance plan](plans/performance.md). Open performance issues are labelled [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Pick up an issue

Issues follow the plan ([What is next](plans/next.md)), under two milestones: "0.8: one project shape" and "1.0: used by other repos". [`good first issue`](https://github.com/joeblew999/charter/labels/good%20first%20issue) are small and say what done looks like. Before a pull request:

- **`mise run check` passes.**
- **A page in `docs/` says what changed,** if a reader needs to know ([Writing docs](writing.md)).
- **Nothing generated was edited by hand** ([what is generated](README.md#what-is-generated)).

## The rules

[The working rules](rules.md) are short and binding: mise drives everything, every task is one line, the contract is the source, workarounds name their upstream issue, and only verified results go into the findings.

## The pages for working here

| Page | What it is |
|---|---|
| [Rules](rules.md) | The working rules |
| [Upstream issues](upstream.md) | Every workaround, and the issue it waits for |
| [Benchmarks](benchmarks.md) | What a request costs, and what made it cheaper |
| [Findings](findings.md) | The log of verified results, newest last |
| [What is next](plans/next.md) | What is not built yet |
| [Performance plan](plans/performance.md) | What is left to make cheaper |
| [The restructure](plans/structure.md) | One project shape: what was done |
