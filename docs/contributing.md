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
mise install    # node and fnox
npm ci          # charter-ts and every TypeScript project; builds charter-ts
npm run check   # everything, locally: needs Docker, no Cloudflare account
```

Every project has the same steps (its npm scripts). The Go side (`go/`) keeps its mise tasks for now.

## Report a bug

Use the GitHub forms (for a bug in Fern, TinyGo, workers-go, Huma or oRPC, check [Upstream issues](upstream.md) first). An agent prints the same form:

```sh
charter issue bug > body.md     # or: feature, upstream, plan. Its first line is the gh command that files it
```

## Make it faster

The target is the TypeScript Worker's cost per request; what was tried and measured is in [Benchmarks](benchmarks.md). Issues: [`perf`](https://github.com/joeblew999/charter/labels/perf).

## Pick up an issue

The plan is the [open issues](https://github.com/joeblew999/charter/issues) and their milestones; [`good first issue`](https://github.com/joeblew999/charter/labels/good%20first%20issue) says what done looks like. Before a pull request: `npm run check` passes, and `docs/` says what changed.

## A change to charter

charter-ts is `charter-ts/`: its commands are in `charter-ts/src/steps.ts`, the kit it writes in `charter-ts/kit/`. Try a change on this repo (`npm run adopt`, `npm run docs:check`) and on a throwaway repo, with the package you pack:

```sh
npm pack -w charter-ts --pack-destination /tmp/try
cd <a throwaway repo>
npx --package /tmp/try/charter-ts-X.Y.Z.tgz charter-ts adopt
```

## Cut a release

```sh
npm run release -- X.Y.Z --dry-run   # every check, and the package into dist/
npm run release -- X.Y.Z             # the version, the tag ts-vX.Y.Z, the GitHub Release with the package
```

It runs `npm run check` before it tags, and never waits for GitHub. The Release carries `charter-ts-X.Y.Z.tgz`, which a repo pins, and `charter-ts.tgz`, which `releases/latest/download/charter-ts.tgz` serves.
