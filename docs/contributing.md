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

On GitHub, use the forms: a bug, a feature, or a bug in a project this one is built on. An agent cannot fill a web form, so the tool prints the same headings:

```sh
charter issue bug > body.md     # or: feature, upstream. Fill it in; its first line is the gh command that files it
```

A report needs the exact command, its full output, and what you expected. If the bug is in TinyGo, workers-go, Fern, Huma or oRPC, check [Upstream issues](upstream.md) first: those are fixed there.

## Make it faster

Performance work here is a loop of about a minute and a half, and it does not touch the deployed Workers: `mise run perf:try` in `examples/notes-go/` builds your change, deploys it to a scratch Worker of its own, benches it and deletes it. You need a Cloudflare account (`npx cf auth login` in the example) and, for CPU figures, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in your environment.

```sh
cd examples/notes-go
mise run perf:try -- -name myidea                    # build, deploy to a scratch Worker of its own, bench, delete
```

- **How to run and read an experiment:** [Try a change](benchmarks.md#try-a-change). Read each request, not the median.
- **The target:** the Go notes Worker costs what the TypeScript one does. `mise run compare` (at the repo root) benches both.
- **What has been found and tried** is on [Performance](benchmarks.md), **what is left** in [the plan](plans/next.md#make-the-go-worker-cheaper). Open performance issues are labelled [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Pick up an issue

Issues follow the plan ([What is next](plans/next.md)), under two milestones: "0.8: one project shape" and "1.0: used by other repos". [`good first issue`](https://github.com/joeblew999/charter/labels/good%20first%20issue) are small and say what done looks like. Before a pull request:

- **`mise run check` passes.**
- **A page in `docs/` says what changed,** if a reader needs to know ([Writing docs](writing.md)).
- **Nothing generated was edited by hand** ([what is generated](README.md#what-is-generated)).

## The rules

[The working rules](rules.md) are short and binding: mise drives everything, every task is one line, the contract is the source, workarounds name their upstream issue, and only verified results go into the findings.

The other pages for working here: [Upstream issues](upstream.md), [Findings](findings.md), [What is next](plans/next.md).
