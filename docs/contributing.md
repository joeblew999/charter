---
title: How to help
nav_order: 6
has_children: true
---
# How to help

Report something, make it faster, or pick up an issue. [The rules](rules.md) are short and binding.

## Set up

```sh
git clone https://github.com/joeblew999/charter && cd charter
mise install && mise run setup    # tools, then every example's npm packages
mise run check                    # everything, locally: needs Docker, no Cloudflare account
```

Each example has the same task names. `mise run doctor` says what is missing.

## Report a bug

Use the GitHub forms (for a bug in Fern, TinyGo, workers-go, Huma or oRPC, check [Upstream issues](upstream.md) first). An agent prints the same form:

```sh
charter issue bug > body.md     # or: feature, upstream. Its first line is the gh command that files it
```

## Make it faster

A loop of about a minute and a half on a scratch Worker; it needs a Cloudflare account, and `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` for CPU figures.

```sh
cd examples/notes-go
mise run perf:try -- -name base      # a baseline, from the same hour
mise run perf:try -- -name myidea    # your change: built, deployed, benched, deleted
```

How to read it: [Performance](guides/performance.md). The target is the TypeScript Worker's cost (`mise run compare` at the root); what was tried: [Benchmarks](benchmarks.md). Issues: [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Pick up an issue

The plan is the [open issues](https://github.com/joeblew999/charter/issues) and their milestones; [`good first issue`](https://github.com/joeblew999/charter/labels/good%20first%20issue) says what done looks like. Before a pull request: `mise run check` passes, and `docs/` says what changed.

## Cut a release

1. `mise run check` at the root, and `charter new` into an empty folder followed by its `mise run check`.
2. Push the tag `vX.Y.Z`; never upload files or module tags by hand.
3. The `release` workflow builds the tool, tags `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z`, and attaches the notes examples' SDKs, specs and CLI.
