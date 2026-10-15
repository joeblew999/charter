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
mise install    # the tools: node, go, gh, fnox
npm ci          # every package; builds charter-ts and charter-lib
npm run check   # the check and projects jobs: the tools, the libraries, every project (Docker for the conformance projects)
npm run ci      # every job GitHub runs, the clients in every language among them (Docker)
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

charter-ts is `charter-ts/`: its commands are in `charter-ts/src/steps.ts`, the kit it writes in `charter-ts/kit/`. `npm run check -w charter-ts` runs its tests (`charter-ts/test/`): `adopt`, `new` and `docs:lint`, run in throwaway folders. Try a change on this repo too (`npm run adopt`, `npm run docs:check`), and on a throwaway repo with the package you pack:

```sh
npm pack -w charter-ts --pack-destination /tmp/try
cd <a throwaway repo>
npx --package /tmp/try/charter-ts-X.Y.Z.tgz charter-ts adopt
```

## The CI

`ci.config.ts` at the root says the jobs and when GitHub runs them, typed. `npx charter-ts ci:write` writes a workflow per job from it (`.github/workflows/self-<job>.yml`: `self-` names this repo's own workflows, `charter-` those charter writes into every repo), and `npm run check` fails while one differs. Each workflow installs the tools with mise, the packages with npm, and runs `npx charter-ts ci <job>`: the command that runs the same job here. One job in one project: `npx charter-ts ci clients go/examples-notes`.

Everything charter-ts makes says how big it is (`made build/app.wasm (the Worker, TinyGo): 3.28 MB, 1.20 MB gzipped (limit 3.00 MB)`), the Rust CLI and the release's packages too, so what grows shows where it is made. Everything says how long it took, so what is slow shows where it runs: each program charter-ts runs that takes a second or more (`took 19.8 s: fern generate --local --group typescript-dist ...`), a server's start (`took 2.6 s: cf answering at ...`), every command and every step of a job, and each job; `check` sums its parts, and `clients` each language's generation and proof. An SDK that is up to date says so instead of being made again.

| Job | When | What |
|---|---|---|
| `check` | Push, pull request, tag; Linux and macOS | charter-ts and charter-lib with their tests, the Go library, the docs, the workflows |
| `projects` | The same; one runner per project | Every project's check |
| `clients` | The same; one runner per conformance project | Every client its `fern/generators.yml` declares, generated and proven in its language. The conformance projects carry every Fern feature, so this proves the generators; an example's specs get Fern's check in its own `check` |
| `deploy` | By hand: `npx charter-ts ci:remote deploy go/examples-notes` | A project's deploy, then its live tests |

## Cut a release

```sh
npm run release -- X.Y.Z --dry-run   # every check, and the package into dist/
npm run release -- X.Y.Z             # the version, the tag ts-vX.Y.Z, the GitHub Release with the package
```

It runs `npm run check` (the `check` and `projects` jobs) before it tags, and never waits for GitHub. The Release carries `charter-ts-X.Y.Z.tgz`, which a repo pins, and the release puts that version into every command the docs show (npx keeps what it fetched for a URL, so a URL without a version would serve an old one).
