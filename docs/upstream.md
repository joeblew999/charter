---
title: Upstream issues
nav_order: 2
parent: How to help
---
# Upstream issues: every workaround, and the issue it waits for

Every gap in TinyGo, workers-go, Huma, oRPC and Fern that charter works around: what the workaround is, where it lives, and what to do when the gap closes. Read it when you meet an `Upstream:` tag in the code, when an issue closes, or before you file a new one.

```sh
mise run upstream:status    # every Upstream: tag in the code, with its issue's state (needs gh)
```

Every workaround carries a tag `Upstream: <owner>/<repo>#<n> (when fixed: ...)`. When an issue closes, do what its tag says, then run `mise run check` and the soak. States below are those of 2026-10-02.

## Worked around here

| Issue | State | Problem | Workaround | When it is fixed |
|---|---|---|---|---|
| [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800) | open, ours | **TinyGo collects garbage at every pause of a workers-go program:** a full collection runs whenever the scheduler is idle and 32 objects with finalizers were made, and workers-go makes one per JavaScript value | The build patches a copy of TinyGo's runtime (`finalizerGCThreshold = 0`) and starts with an 8 MB heap (`cmd/charter/wasm.go`) | Drop the patch; keep the starting heap |
| [tinygo-org/tinygo#5801](https://github.com/tinygo-org/tinygo/issues/5801) | closed, ours: fixed on TinyGo's dev branch | **TinyGo 0.42.0 allocates a stack for every goroutine and every call from JavaScript,** and only a full collection gets one back | The build patches TinyGo's scheduler in the same copy: a finished goroutine's stack is kept for the next. The patch leaves itself out with a TinyGo that has the fix | Delete the stack patches when the tool pins a TinyGo release with the fix |
| [tinygo-org/tinygo#5798](https://github.com/tinygo-org/tinygo/issues/5798) | open, ours | **Go timers stall on Cloudflare** (not under local workerd): the production clock moves in whole milliseconds and TinyGo sleeps for fractions, so `time.Sleep`, timers and context deadlines hang about half the time | `go/worker/tinygo-clock.mjs` rounds every sleep up to a whole millisecond | Delete the file and its import |
| [tinygo-org/net#56](https://github.com/tinygo-org/net/pull/56) | open pull request (our tinygo-org/tinygo#5799 was closed as its duplicate) | TinyGo's `http.ServeMux` does not match method patterns: `"GET /a"` is 404, so Huma's `humago` adapter serves nothing | `humaworkers` routes itself (`go/humaworkers/`) | When it is in a TinyGo release: Huma's own adapter can route |
| [tinygo-org/tinygo#3599](https://github.com/tinygo-org/tinygo/issues/3599) | open | No `reflect.StructOf`; Huma's default config installs a hook that calls it | `humaworkers.Config` leaves the hook out | Keep Huma's default hooks |
| [tinygo-org/tinygo#3862](https://github.com/tinygo-org/tinygo/issues/3862) | open | No `reflect.Value.MethodByName`; Huma's typed multipart form calls it | The upload takes the plain `multipart.Form` and declares its schema (`examples/showcase-go/api/contract.go`) | Use `huma.MultipartFormFiles[T]` |
| [syumai/workers-go#97](https://github.com/syumai/workers-go/issues/97) | open | workers-go cannot answer a WebSocket upgrade | Go answers with a stream of lines; `go/worker/websocket.mjs` sends each as a frame, and gives each frame from the client to Go as a `POST` | Answer the upgrade in Go and delete the adapter |
| [syumai/workers-go#220](https://github.com/syumai/workers-go/issues/220) | open | A Durable Object class cannot be written in Go | The hub is JavaScript (`go/worker/hub.mjs`) | The hub can be Go |
| [fern-api/fern#17936](https://github.com/fern-api/fern/issues/17936) | open, ours | The SSE terminator is matched as a substring; the Go SDK applies `[DONE]` when none is declared | `END = "[end-of-stream]"` is declared as the terminator, and note bodies reject it (both notes contracts) | Drop the body rule |
| [fern-api/fern#17937](https://github.com/fern-api/fern/issues/17937) | open, ours | `resumable` SDKs do not reconnect after a network reset, only after a clean end | The client rule: catch, then call again with `after` ([the loops](guides/streaming.md#the-client-rule)) | Keep the loop for planned ends only |
| [fern-api/fern#17938](https://github.com/fern-api/fern/issues/17938) | open, ours | The SSE `event:` field is ignored, so `event: error` is decoded as an item | `watch` never sends error events: it ends without the terminator. The oRPC contracts replace oRPC's event envelope with the item's schema | Error events could say why a stream ended |
| [fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939) | open, ours | The generated CLI prints json and jsonl only when a stream ends; the Rust generator does not escape the terminator | The terminator is plain text; the soak reports the CLI's latency and does not judge it | Judge the CLI's latency like the SDKs' |
| [fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559) | open | Fern rejects OpenAPI 3.2.0, oRPC 2.0's default | `version: "3.1.1"` (`examples/notes-ts/src/specs.ts`) | Use oRPC's default |
| [middleapi/orpc#2115](https://github.com/middleapi/orpc/issues/2115) | closed by us: oRPC wants it as a community package first | oRPC generates no AsyncAPI | `examples/notes-ts/src/asyncapi.ts` is that generator | Publish it as a package ([plan](plans/next.md)) |

## Found, not filed

- **Fern's Go generator writes a test that does not compile** when the OAuth token endpoint is form-encoded and its request schema is a named one: `undefined: showcase.Request` (fern-go-sdk 1.64.0). With the schema written inline it passes. Huma names every body schema, so the Go showcase writes this one into the operation (`examples/showcase-go/api/contract.go`). When fixed: drop that `RequestBody`.
- **Huma documents `application/octet-stream` for a `RawBody` beside a typed `Body`, and Fern's Go generator then takes an `io.Reader`** (Huma 2.39.1, fern-go-sdk 1.64.0). `humaworkers` takes the binary content out of every operation that also has another content type (`jsonOnly` in `go/humaworkers/humaworkers.go`). When fixed: delete `jsonOnly`.
- **Fern ignores an AsyncAPI server's `pathname`.** The default WebSocket URL of every generated client lacks it. Nothing works around it in the specs: the oRPC showcase's tests pass `baseUrl`.
- **What oRPC's generators cannot say:** a form-encoded request body, `security` on one operation, document-level settings, OpenAPI `webhooks`. Each is added in code ([the table](guides/fern-features.md#from-an-orpc-contract)).
- **`cf dev` drops a WebSocket upgrade that the Worker refuses.** Natively the client sees the 401; under `cf dev` the connection just closes. The showcase test accepts either.

## Not an issue, a setting

- **Huma needs a bigger stack than TinyGo's Wasm default.** The first request fails with `memory access out of bounds`; 64 KB failed too. The build passes 128 KB.
- **Huma's adapter writes uploads over 8 KB to a temporary file, and a Worker has no disk.** `humaworkers` raises the limit to 32 MB, so uploads stay in memory.

## Told to the maintainers

- **What made a Go Worker cost what a TypeScript one does** was told to workers-go's maintainer in [syumai/workers-go#240](https://github.com/syumai/workers-go/issues/240), closed by us: nothing in workers-go needs to change. The runtime-reusing entry was offered as a small pull request.
- **A place to try a TinyGo or workers-go change:** the two notes examples serve the same API with the same tests, measured side by side by `mise run compare` ([Benchmarks](benchmarks.md)).
