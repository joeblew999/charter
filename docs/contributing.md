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
charter issue bug > body.md     # or: feature, upstream, plan. Its first line is the gh command that files it
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

1. `charter new` into an empty folder, then its `mise run check`.
2. At the root, `mise run release -- vX.Y.Z -dry-run`, then `mise run release -- vX.Y.Z`: `check` (Docker), the notes examples' SDKs, specs and CLI for every OS (`dist`, all six only on a Mac), the tag, then the TypeScript library's package (`charter-ts-X.Y.Z.tgz`), the tool (GoReleaser), the examples' files and `SHA256SUMS` onto the Release (`release:publish`), and the tags `go/vX.Y.Z` and `examples/notes-go/sdk/go/vX.Y.Z` (`release:tags`). Never upload files or module tags by hand.
3. The `check` and `release` workflows then run on the tag: a failure there is fixed in a new release.

### Fast track: the tool only

A repo that uses charter installs the tool (`github:joeblew999/charter` under `[tools]`) and includes `tasks/` by tag. When a change is to the tool or to `tasks/`, the tool's binaries are all such a repo is waiting for, and they do not need Docker or the workflows:

```sh
mise run charter:check                         # the tool's own checks: about a minute
git tag vX.Y.Z && git push origin vX.Y.Z       # on main, pushed
mise run charter:release -- -tag vX.Y.Z        # GoReleaser: the tool for every OS, onto the Release
```

`charter:release` makes the GitHub Release if it is missing and attaches the archives and `checksums.txt`, with the token `gh` holds. It needs `-tag`: without it, it builds a snapshot into `dist/` and publishes nothing. A minute or two, and `mise up --bump github:joeblew999/charter` in the other repo takes it.

The workflows still run on the tag and finish the release: the TypeScript package, the examples' files, `SHA256SUMS`, the module tags. They find the tool there and only build it, as a check. Nothing in the fast track waits for them.

Not checked this way: `mise run check` (the Go and TypeScript libraries, the examples under workerd). Use the full release in step 2 when one of those changed.
