---
title: Upstream issues
nav_order: 9
---

# Upstream issues

Every workaround in the code carries a tag `Upstream: <owner>/<repo>#<n> (when fixed: ...)`. `mise run upstream:status` finds the tags and shows each issue's state. When an issue closes, do what its tag says, then re-run `mise run check` and the soak for that server.

## Filed, and worked around here

| Issue | Problem | Workaround here | When it's fixed |
|---|---|---|---|
| [fern-api/fern#17936](https://github.com/fern-api/fern/issues/17936) | SSE terminator matched as a substring; the Go SDK applies `[DONE]` when none is declared | `END = "[end-of-stream]"` declared as the terminator, and note bodies reject it (`src/contract.ts`) | Drop the body rule |
| [fern-api/fern#17937](https://github.com/fern-api/fern/issues/17937) | `resumable` SDKs don't reconnect after a network reset, only after a clean end | Client rule: catch, then call again with `after` (`test/soak.mjs`, the client loops in `docs/sdk.md`) | SDKs survive resets themselves; keep the loop for planned ends |
| [fern-api/fern#17938](https://github.com/fern-api/fern/issues/17938) | SSE `event:` ignored, so `event: error` is decoded as a note | `watch` never sends error events; it ends without the terminator (`src/index.ts`). The spec hook replaces oRPC's envelope with the note schema (`src/contract.ts`) | Error events could say why a stream ended; oRPC's envelope could be used as is |
| [fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939) | CLI prints json/jsonl only when a stream ends; the Rust generator doesn't escape the terminator | The terminator is plain text (`src/contract.ts`); CLI latency is reported, not judged (`test/soak.mjs`) | Judge CLI latency like the SDKs' |
| [fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559) | Fern rejects OpenAPI 3.2.0 (oRPC 2.0's default) | `version: "3.1.1"` (`src/specs.ts`) | Use oRPC's 3.2.0 default |
| [middleapi/orpc#2115](https://github.com/middleapi/orpc/issues/2115) | oRPC generates no AsyncAPI | `src/asyncapi.ts` (offered upstream in the issue) | Switch to oRPC's generator and delete the file |
| [tinygo-org/tinygo#3599](https://github.com/tinygo-org/tinygo/issues/3599) | TinyGo has no `reflect.StructOf`; Huma's default config installs a hook that calls it | `humaworkers.Config` leaves the hook out (`api-go/humaworkers`) | Keep Huma's default hooks |
| [tinygo-org/tinygo#5798](https://github.com/tinygo-org/tinygo/issues/5798) | **Go timers stall on Cloudflare** (not under local workerd): after `setTimeout(d)` the production clock has moved by `d` rounded down to a millisecond, and TinyGo sleeps for fractions, so `time.Sleep`, timers and context deadlines hang about half the time (6 of 10 streams) | `api-go/worker/tinygo-clock.mjs` rounds every TinyGo sleep up to a whole millisecond (16 of 16 then end on time) | Delete the file and its import |
| [tinygo-org/tinygo#5799](https://github.com/tinygo-org/tinygo/issues/5799) | TinyGo's `http.ServeMux` doesn't match method patterns: `"GET /a"` is 404 (wildcards work), so Huma's `humago` adapter serves nothing | `api-go/humaworkers` matches routes itself | Huma's own adapter can do the routing |
| [tinygo-org/tinygo#3862](https://github.com/tinygo-org/tinygo/issues/3862) | TinyGo has no `reflect.Value.MethodByName`; Huma's typed multipart form (`huma.MultipartFormFiles[T]`) calls it | The upload takes the plain `multipart.Form` and declares its schema on the operation (`api-go/showcase/contract.go`) | Use `huma.MultipartFormFiles[T]` |
| [syumai/workers-go#97](https://github.com/syumai/workers-go/issues/97) | workers-go can't answer a WebSocket upgrade | Go answers with a stream of lines; `api-go/worker/websocket.mjs` sends each as a frame, and gives each frame from the client to Go as a `POST` | Answer the upgrade in Go and delete the adapter |
| [syumai/workers-go#220](https://github.com/syumai/workers-go/issues/220) | A Durable Object class can't be written in Go | The hub is JavaScript (`api-go/worker/hub.mjs`) | The hub can be Go |

The oRPC rows are tagged in `api/src/`; the Go Worker carries the same tags for the Fern issues (`api-go/api/`), and the showcase contract (`sdk/harness/src/contract.ts`) the one for its SSE stream, so `upstream:status` lists every place.

## Found, not filed yet

- **Fern's Go generator writes a test that doesn't compile** when the OAuth token endpoint is form-encoded and its request schema is a named one (a `$ref`): `auth/oauth_wire_test/oauth_wire_test.go: undefined: showcase.Request` (fern-go-sdk 1.64.0; `go vet` and `go test` of the generated SDK fail, the SDK itself builds). With the same schema written inline it names the type `GetTokenRequest` and passes. Huma names every body schema, so the Go showcase writes this one into the operation (`api-go/showcase/contract.go`, the token operation). When fixed: drop that `RequestBody`.
- **`cf dev` drops a WebSocket upgrade that the Worker refuses.** Go answers 401 to an upgrade without a token; natively the client sees the 401, under `cf dev` the connection just closes. Nothing works around it: `test/showcase-test.mjs` accepts either.

## Not an issue, a setting

**Huma needs a bigger stack than TinyGo's Wasm default.** With the default 64 KB stack the first Huma request fails with `memory access out of bounds`. `-stack-size=256kb` works (128 KB also did). `mise run api-go:build` passes it.

**Huma's adapter writes uploads over 8 KB to a temporary file, and a Worker has no disk.** A multipart upload then fails with `open /tmp/multipart-...: file does not exist`. `humaworkers` sets `humago.MultipartMaxMemory` to 32 MB, so uploads stay in memory.

## Useful to the workers-go maintainers

- **A typed client for the whole Cloudflare API.** `mise run sdk:cloudflare` slices products (D1, KV, R2, Workers, ...) out of Cloudflare's own OpenAPI spec, and Fern generates a Go or TypeScript SDK from it ([sdk.md](sdk.md)).
- **A TypeScript Worker and a Go Worker serving the same API, with the same tests and measured side by side** ([benchmarks.md](benchmarks.md)): a ready place to try a workers-go or TinyGo change and see what it does to CPU time per request.
