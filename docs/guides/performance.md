---
title: Measure and improve performance
nav_order: 9
parent: Guides
---

# Measure and improve performance

How to see what each request of your API costs on Cloudflare, and how to try a change in about 70 seconds without touching the deployed Worker. The numbers for the notes example, and what made a Go Worker cost what a TypeScript one does, are in [Benchmarks](../benchmarks.md). To work on charter's own cost: [How to help](../contributing.md#make-it-faster).

## What you need

A deployed Worker and its `API_URL` ([Deploy](deploy.md)). For CPU figures: `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment, and observability enabled in `cloudflare.config.ts` (the example has it). CPU time is Cloudflare's own figure, what Workers bills and limits. It takes up to two minutes to arrive.

## The tasks

| Task | What it does | Touches |
|---|---|---|
| `mise run bench` | Times every GET operation of the deployed Worker once it is warm: wall time, CPU median and p99, and the CPU of each request | Read-only |
| `mise run bench -- -write` | The same with the operations that change data | Writes test data |
| `mise run perf` | Deploys, then benches the new isolate from its first request: 8 at once, then each operation. About two minutes | Redeploys your Worker |
| `mise run perf:try -- -name <experiment>` | Builds, deploys to a scratch Worker with a database and hub of its own, benches it, deletes it. About 70 seconds | Nothing of yours |
| `mise run perf:clean` | Deletes scratch Workers and databases a run left | Scratch only |

`bench` reads the operations from the spec the server gives at `/api/openapi.json` and their inputs from the spec's examples, so it measures your operations, not the notes. It skips streams, and operations without an example for a required input.

## Read each request, not the median

```
GET /api/hello, 8 at once     3 6n 5w 74n 13w 110n 83n 101n
GET /api/hello                2 2 2 3 2 3 2 2 2 3 3 2 4 2 3 6 3 2 2 2 1 3 2
GET /api/notes                18 5 4 4 5 6 7 4 5 5 5 4 8w 5 5 4 4 5 4 6 4 4 7
```

Milliseconds of CPU per request, in the order sent (a `mise run perf` of the notes example, 2026-10-02).

No letter: served by a Go runtime that had served before. `w`: by one started ahead, while the Worker's module loaded. `n`: by one the request had to start. `?`: Cloudflare has no figure yet. A median hides what this shows: the request that starts a runtime, the first use of an operation, the first request in a new isolate.

## Try a change

```sh
mise run perf:try -- -name base                         # a baseline, from the same hour
mise run perf:try -- -name heap4 -build '-heap 4'       # the same code with other build flags
mise run perf:try -- -name myidea -keep                 # leave the scratch Worker; perf:clean removes it later
mise run perf:try -- -name tinygodev -prebuilt          # deploy the build/ that is there, made by another TinyGo
```

- **The bench it runs** is `-each -burst 8 -write`. Other bench flags go after a second `--` (`charter perf -name x -- <bench flags>`).
- **`-name`** is lower-case letters, digits and hyphens, at most 22. The scratch Worker is `<worker>-perf-<name>`.
- **Several can run at once:** each name is its own Worker and database.
- **It needs** `cloudflare.config.ts` to name the Worker after the mode when the mode starts with `perf-`. The example's does.
- **`-build`** takes the flags of `charter wasm-build`: `-heap`, `-stack`, `-opt`, `-plain` ([The charter command](../reference/charter.md#wasm-build)).

## What moves the cost

In the order that mattered for the notes example:

1. **Cross between Go and JavaScript as few times as possible.** Every value that crosses costs, and so does every promise Go waits for. Read rows with the library's `d1` package, not `database/sql`; publish to the hub once.
2. **Warm what a new isolate needs.** `go.warm({ paths: [...] })` in `worker.mjs` has each waiting runtime answer those GET paths during start-up. Only paths whose handlers touch no binding.
3. **Start more runtimes ahead if bursts hit new isolates:** `go.warm({ runtimes: 4, paths })`. Each holds 8 MB and adds to the isolate's start-up time. The default is 2, the most kept waiting is 4.
4. **Do less before `main` serves.** Package initialisers run each time a runtime starts.

Then prove it is still right: `mise run check`, and `mise run soak` for anything near a stream.

## The flags

Every flag of the bench, the experiment and the build: [The charter command](../reference/charter.md#bench). Pass them after `--`: `mise run bench -- -write -n 30`. For any server, `charter bench <url>`: a local one gives wall time only.
