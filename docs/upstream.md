---
title: Upstream issues
nav_order: 10
parent: This repository
---
# Upstream issues

Every upstream gap this repo works around, what the workaround is, and what to do when the gap closes. Read it when you meet an `Upstream:` tag in the code, when an issue closes, or before filing a new one.

```sh
mise run upstream:status      # every Upstream: tag in the code, with its issue's state (needs gh)
```

Every workaround in the code carries a tag `Upstream: <owner>/<repo>#<n> (when fixed: ...)`. When an issue closes, do what its tag says, then re-run `mise run check` and the soak for that server.

## Filed, and worked around here

| Issue | Problem | Workaround here | When it's fixed |
|---|---|---|---|
| [fern-api/fern#17936](https://github.com/fern-api/fern/issues/17936) | SSE terminator matched as a substring; the Go SDK applies `[DONE]` when none is declared | `END = "[end-of-stream]"` declared as the terminator, and note bodies reject it (`api/ts/src/contract.ts`, `api/go/api/contract.go`) | Drop the body rule |
| [fern-api/fern#17937](https://github.com/fern-api/fern/issues/17937) | `resumable` SDKs don't reconnect after a network reset, only after a clean end | Client rule: catch, then call again with `after` (`test/soak.mjs`, the client loops in [realtime.md](realtime.md#what-a-client-does)) | SDKs survive resets themselves; keep the loop for planned ends |
| [fern-api/fern#17938](https://github.com/fern-api/fern/issues/17938) | SSE `event:` ignored, so `event: error` is decoded as a note | `watch` never sends error events; it ends without the terminator (`api/ts/src/index.ts`, `api/go/api/handlers.go`). The spec hook replaces oRPC's envelope with the note schema (`api/ts/src/contract.ts`, and the oRPC showcase's `sdk/harness/src/contract.ts`) | Error events could say why a stream ended; oRPC's envelope could be used as is |
| [fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939) | The Fern CLI prints json/jsonl only when a stream ends; the Rust generator doesn't escape the terminator | The terminator is plain text (`api/ts/src/contract.ts`, `api/go/api/contract.go`); CLI latency is reported, not judged (`test/soak.mjs`) | Judge CLI latency like the SDKs' |
| [fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559) | Fern rejects OpenAPI 3.2.0 (oRPC 2.0's default) | `version: "3.1.1"` (`api/ts/src/specs.ts`) | Use oRPC's 3.2.0 default |
| [middleapi/orpc#2115](https://github.com/middleapi/orpc/issues/2115) | oRPC generates no AsyncAPI | `api/ts/src/asyncapi.ts` (offered upstream in the issue) | Switch to oRPC's generator and delete the file |
| [tinygo-org/tinygo#3599](https://github.com/tinygo-org/tinygo/issues/3599) | TinyGo has no `reflect.StructOf`; Huma's default config installs a hook that calls it | `humaworkers.Config` leaves the hook out (`api/go/humaworkers/`) | Keep Huma's default hooks |
| [tinygo-org/tinygo#5798](https://github.com/tinygo-org/tinygo/issues/5798) | **Go timers stall on Cloudflare** (not under local workerd): after `setTimeout(d)` the production clock has moved by `d` rounded down to a millisecond, and TinyGo sleeps for fractions, so `time.Sleep`, timers and context deadlines hang about half the time (6 of 10 streams) | `api/go/worker/tinygo-clock.mjs` rounds every TinyGo sleep up to a whole millisecond (16 of 16 then end on time) | Delete the file and its import |
| [tinygo-org/tinygo#5799](https://github.com/tinygo-org/tinygo/issues/5799) | TinyGo's `http.ServeMux` doesn't match method patterns: `"GET /a"` is 404 (wildcards work), so Huma's `humago` adapter serves nothing | `api/go/humaworkers/` matches routes itself | Huma's own adapter can do the routing |
| [tinygo-org/tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800) | **TinyGo collects garbage at every pause of a workers-go program:** a full collection runs whenever the scheduler is idle and 32 objects with finalizers were made, and workers-go makes one per JavaScript value. A read cost 40 to 70 ms of CPU on Cloudflare | `dev wasm-build` builds against a copy of TinyGo's runtime with `finalizerGCThreshold = 0`, and with an 8 MB starting heap (`dev/wasm.go`). TinyGo itself is not changed. | Drop the patch and build with TinyGo as it is; keep the starting heap |
| [tinygo-org/tinygo#5801](https://github.com/tinygo-org/tinygo/issues/5801) | **TinyGo allocates a stack for every goroutine and every call from JavaScript, and rarely frees one.** A hello on workers-go allocated 3.4 MB with 256 KB stacks; after 9 of them in one runtime 23 MB was still in use although the collector had run 14 times | `dev wasm-build` patches TinyGo's scheduler in the same copy of the runtime (`src/internal/task/task_asyncify.go`, about 20 lines): a finished goroutine's stack is kept for the next one. `api/go/worker/go.mjs` still drops a runtime before its collector would run | Drop the stack patches; a runtime could then be reused without limit |
| [tinygo-org/tinygo#3862](https://github.com/tinygo-org/tinygo/issues/3862) | TinyGo has no `reflect.Value.MethodByName`; Huma's typed multipart form (`huma.MultipartFormFiles[T]`) calls it | The upload takes the plain `multipart.Form` and declares its schema on the operation (`api/go/showcase/contract.go`) | Use `huma.MultipartFormFiles[T]` |
| [syumai/workers-go#97](https://github.com/syumai/workers-go/issues/97) | workers-go can't answer a WebSocket upgrade | Go answers with a stream of lines; `api/go/worker/websocket.mjs` sends each as a frame, and gives each frame from the client to Go as a `POST` | Answer the upgrade in Go and delete the adapter |
| [syumai/workers-go#220](https://github.com/syumai/workers-go/issues/220) | A Durable Object class can't be written in Go | The hub is JavaScript (`api/go/worker/hub.mjs`) | The hub can be Go |

The numbers in the timer row were measured on the deployed Go Worker on 2026-10-01 ([findings.md](findings.md)). Each row's tag is in the files it names, so `mise run upstream:status` lists every place.

## Found, not filed

- **Fern's Go generator writes a test that doesn't compile** when the OAuth token endpoint is form-encoded and its request schema is a named one (a `$ref`): `auth/oauth_wire_test/oauth_wire_test.go: undefined: showcase.Request` (fern-go-sdk 1.64.0; `go vet` and `go test` of the generated SDK fail, the SDK itself builds). With the same schema written inline it names the type `GetTokenRequest` and passes. Huma names every body schema, so the Go showcase writes this one into the operation (`api/go/showcase/contract.go`, the token operation). When fixed: drop that `RequestBody`.
- **Huma documents `application/octet-stream` for a `RawBody` that sits beside a typed `Body`, and Fern's Go generator then takes an `io.Reader`.** An input with `Body T` and `RawBody []byte` keeps the request exactly as posted next to the validated one. Huma 2.39.1 adds `application/octet-stream` (a binary string) to the operation's request body next to `application/json` (`setRequestBodyFromRawBody` in its `huma.go`), though the operation only reads JSON. Fern's Go generator (fern-go-sdk 1.64.0) picks the binary one: `Report(ctx, id string, request io.Reader)`. `humaworkers` takes the binary content out of every operation it registers that also has another content type (`jsonOnly` in `api/go/humaworkers/humaworkers.go`), so the spec says JSON only and the method takes the typed request. No Huma issue covers it (searched 2026-10-01). When fixed: delete `jsonOnly`.
- **Fern ignores an AsyncAPI server's `pathname`.** With `host` and `pathname: /api/mock`, the TypeScript, Go and Rust output all have `wss://<host>` as the default WebSocket URL. Nothing works around it in the specs: the tests of the oRPC showcase pass `baseUrl` ([sdk.md](sdk.md#the-orpc-showcase)).
- **What oRPC's generators can't say:** a form-encoded request body, `security` on one operation, document-level settings in the contract, OpenAPI `webhooks`. Each is added in code; the table is in [sdk.md](sdk.md#the-orpc-showcase).
- **`cf dev` drops a WebSocket upgrade that the Worker refuses.** Go answers 401 to an upgrade without a token; natively the client sees the 401, under `cf dev` the connection just closes. Nothing works around it: `test/showcase-test.mjs` accepts either.

## Not an issue, a setting

**Huma needs a bigger stack than TinyGo's Wasm default.** With the default stack the first Huma request fails with `memory access out of bounds`; 64 KB failed too. `-stack-size=128kb` works, and 96 KB ran the examples. `mise run api:go:build` and `mise run showcase:go:build` pass it.

**Huma's adapter writes uploads over 8 KB to a temporary file, and a Worker has no disk.** A multipart upload then fails with `open /tmp/multipart-...: file does not exist`. `humaworkers` sets `humago.MultipartMaxMemory` to 32 MB, so uploads stay in memory.

## Useful to the workers-go maintainers

- **The cost of finalizers, with a fix a project can apply today.** Told to the maintainer in [syumai/workers-go#240](https://github.com/syumai/workers-go/issues/240): what the collector does per request, the measurements, and the two build settings. Nothing in this repo waits on it.
- **A typed client for the whole Cloudflare API.** `mise run sdk:cloudflare` slices products (D1, KV, R2, Workers, ...) out of Cloudflare's own OpenAPI spec, and Fern generates a Go or TypeScript SDK from it ([sdk.md](sdk.md#adding-an-api)).
- **A TypeScript Worker and a Go Worker serving the same API, with the same tests and measured side by side** ([benchmarks.md](benchmarks.md)): a ready place to try a workers-go or TinyGo change and see what it does to CPU time per request.
