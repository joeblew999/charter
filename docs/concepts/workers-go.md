---
title: Huma on Cloudflare Workers
nav_order: 2
parent: Concepts
---

# Huma on Cloudflare Workers: what is different, and what it costs

How a [Huma](https://huma.rocks) API runs on Cloudflare Workers, what is different from a Go server you have written before, and what a request costs. Read it before you choose Go for a Worker, and before you write code that assumes a normal Go process.

## The three-minute version

- **Your API is ordinary Go:** Huma operations and handlers on `net/http` types. The same code runs natively (`mise run run`).
- **On Cloudflare it is Wasm,** built by [TinyGo](https://tinygo.org) and connected to the Worker by [workers-go](https://github.com/syumai/workers-go).
- **A Go runtime is not a server process.** It serves one request at a time, for some dozens of requests, and is then replaced. State goes in a binding.
- **It costs what a TypeScript Worker costs** per request, measured side by side ([Benchmarks](../benchmarks.md)). TinyGo and workers-go as they come cost far more: the library and the build here are the difference.
- **A few things are JavaScript,** because Go cannot do them there. They ship with the library; you do not edit them.
- **TinyGo is not the standard compiler.** Some libraries do not build. `mise run check` finds out.

## How it runs

```mermaid
flowchart LR
    R["Request"] --> J["worker.mjs and the glue<br/>(JavaScript)"]
    J --> G["A Go runtime<br/>(TinyGo Wasm)"]
    G --> H["Your handler<br/>(Huma)"]
    H --> D["D1: the database"]
    H --> U["The hub<br/>(a Durable Object)"]
```

A Worker runs JavaScript and WebAssembly, not native programs. The glue receives each request and hands it to a Go runtime: one that is waiting, or a new one.

## A Go runtime is not a server process

A normal Go server starts once and serves for days. Here two requests at the same moment are in two runtimes, and Cloudflare drops them all when it drops the isolate.

- **Do not count on memory between requests.** A package variable, a cache, a counter: the next request may be in a runtime where it is empty.
- **Do not count on memory being empty either.** The next request may be in the same runtime. Never keep one caller's data in a package variable.
- **State goes in a binding.** What must last goes in D1; what must be shared live goes through a Durable Object. The notes example does both.
- **No background work.** A goroutine must not outlive its request: no ticker, no queue worker.
- **Start-up is paid more often.** What a program does before `main` serves, it does each time a runtime starts. So `humaworkers` registers an operation only when a request first matches it.
- **A stream is one long request.** It keeps its runtime while it is open. A WebSocket's feed and each message the client sends run in runtimes of their own.

A runtime is dropped before Go's collector would have to run in it: the Go side says when its heap has no room for another request (`transport.Serve`), and the glue starts a new one for the request after.

## What Go cannot do there

The JavaScript is the library's (`go/worker/`). The build writes it into your project's `build/` from the library version in `go.mod`, so it always matches the Go it talks to. Your project has one entry file of a few lines, `worker.mjs`.

| Go cannot | What does it instead | File in `go/worker/` |
|---|---|---|
| Be the Worker's entry | The glue hands each request to a Go runtime, keeps runtimes between requests, and starts two while the module loads | `go.mjs` |
| Answer a WebSocket upgrade | Go answers with a stream of lines; the adapter sends each as a frame. What the client sends reaches Go as a `POST` | `websocket.mjs` |
| Be a Durable Object class | The hub is a small class with hibernating WebSockets. It stores nothing | `hub.mjs` |
| Rely on its timers | On Cloudflare, not under local workerd, Go timers stalled about half the time. The fix rounds every sleep up to a whole millisecond | `tinygo-clock.mjs` |

Which paths are WebSockets, what they take and what they send all stay in the Go contract. The JavaScript only carries bytes. The protocol is in [Go packages](../reference/packages.md#transport).

## What TinyGo lacks that you will meet

`go test` runs with standard Go and cannot see these. `mise run check` also runs the Wasm under workerd for that reason.

| Gap | What you see | What the library or the build does |
|---|---|---|
| The default stack is too small for Huma | `memory access out of bounds` on the first request | The build passes `-stack-size=128kb` |
| No `reflect.StructOf` | A panic from a hook in Huma's default config | `humaworkers.Config` leaves that hook out |
| `http.ServeMux` does not match method patterns (`"GET /path"`) | Every route is 404 with Huma's own adapter | `humaworkers` matches routes itself |
| No `reflect.Value.MethodByName` | Huma's typed multipart form panics | Take the plain `multipart.Form` and declare its schema |
| No disk | An upload over 8 KB fails | `humaworkers` keeps uploads in memory, up to 32 MB |
| No MCP SDK builds with TinyGo | | `humamcp` implements the protocol by hand |

Expect the same of other libraries that lean on `reflect`, the file system or parts of `net`. Find out early: `mise run build`, `mise run test:workerd`. The issues are in [Upstream issues](../upstream.md).

## What a request costs, and why

On Cloudflare a Go Worker built this way costs what the TypeScript one does; the first request in a new isolate costs more. The figures, and what each step below was worth, are in [Benchmarks](../benchmarks.md). What closed the gap, in order:

1. **The collector no longer runs at every pause.** A patch to a copy of TinyGo's runtime, and an 8 MB starting heap ([tinygo-org/tinygo#5800](../upstream.md)).
2. **Go runtimes are reused between requests,** and two are started while the Worker's module loads (`go/worker/go.mjs`, `transport.Run`).
3. **Goroutine stacks are reused.** A second patch; TinyGo's dev branch has the fix.
4. **A request crosses into Go in one call and out in one call** (`transport`).
5. **D1 rows cross as one JSON string** (`d1`).
6. **The hub is published to by RPC,** not by a fetch.

The rule behind the last three: every value that crosses between Go and JavaScript costs. TinyGo itself is not rebuilt: the tool pins the TinyGo it was tested with, and patches a copy of its runtime source at build time ([wasm-build](../reference/charter.md#wasm-build)).

What still costs:

- **A new isolate.** Cloudflare starts one after a deploy, when a Worker has been idle, and when traffic spreads. Its first requests cost more, and requests that arrive together beyond the waiting runtimes start their own.
- **Workers Free allows 10 ms of CPU per request.** Ordinary requests are well inside it; the first in a new isolate are at it. Free is enough to try; plan on Workers Paid for production.
- **A stream costs CPU for as long as it is open.**

Measure your own: [Measure and improve performance](../guides/performance.md).

## The same code runs natively

The handlers do not know they are on Cloudflare. They get the database and the hub through one value (`api.Env`), filled in by `platform_js.go` (the bindings) or `platform_other.go` (memory).

| | `mise run run` | `mise run dev` |
|---|---|---|
| What runs | Standard Go, one process | The Wasm under workerd |
| Storage | Memory: gone when it stops | A local D1 |
| Shows TinyGo's gaps | No | Yes |

## When it fits

| A good fit | Not a good fit |
|---|---|
| Your team writes Go and wants one language for the API, its types and its tests | You must stay within Workers Free for certain |
| The API is request and response, plus streams, with its state in D1 or a Durable Object | You depend on Go libraries TinyGo cannot build, or on in-process state: caches, pools, background goroutines |
| You want the same code to run off Cloudflare too | You cannot accept workarounds in the path: a few JavaScript files and open upstream issues, each small and tracked |

For TypeScript, the same design exists on oRPC: [The TypeScript (oRPC) version](../guides/typescript.md).
