---
title: Performance
nav_order: 5
parent: Guides
---

# Performance: measure your API, try a change

How to see what each request of your API costs on Cloudflare, and how to try a change in about a minute and a half without touching your deployed Worker. CPU time is Cloudflare's figure, what Workers bills and limits. The numbers for the examples: [Benchmarks](../benchmarks.md).

You need a deployed Worker, `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in the environment, and observability on in `cloudflare.config.ts` (a new project has it).

## Measure

```sh
mise run bench              # every GET operation, warm: CPU median, p99 and each request. About half a minute
mise run bench -- -write    # also the operations that change data
mise run perf               # REDEPLOYS: the new isolate from its first request
```

`bench` takes the operations and their inputs from the spec's examples, so it measures your API ([every flag](../reference/charter.md#bench)).

**Read each request, not the median.** Each number is one request's CPU in ms, in the order sent, marked by the Go runtime that served it: none for a reused one, `w` for one started ahead, `n` for one the request had to start.

```
GET /api/hello, 8 at once     3 6n 5w 74n 13w 110n 83n 101n
```

## Try a change

```sh
mise run perf:try -- -name base                     # a baseline, from the same hour
mise run perf:try -- -name myidea                   # your change: built, deployed to a scratch Worker, benched, deleted
mise run perf:try -- -name heap4 -build '-heap 4'   # other build flags (those of charter wasm-build)
mise run perf:clean                                 # deletes scratch Workers a run left
```

Each name is its own Worker and database, so several people can run at once. Then prove it is still correct: `mise run check`, and `mise run soak` near a stream.

## What lowers the cost

- **Cross between Go and JavaScript less:** read rows with the library's `d1` package, publish to the hub once.
- **Warm a new isolate:** list GET paths whose handlers touch no binding in `go.warm({ paths })` in `worker.mjs`.
- **Do less in package initialisers:** they run each time a Go runtime starts.
