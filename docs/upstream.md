---
title: Upstream issues
nav_order: 3
parent: How to help
---
# Upstream issues: every workaround, and the issue it waits for

`mise run upstream:status` shows each issue's state. When one closes, do what the row says, then `mise run check` and the soak.

| Issue | Problem | Workaround | When fixed |
|---|---|---|---|
| [tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800) | The collector runs at every pause of a workers-go program | A runtime patch and an 8 MB heap (`cmd/charter/wasm.go`) | Drop the patch |
| [tinygo#5801](https://github.com/tinygo-org/tinygo/issues/5801) (fixed on dev) | Goroutine stacks are rarely freed | A patch reuses them | Drop it with a TinyGo release that has the fix |
| [tinygo#5798](https://github.com/tinygo-org/tinygo/issues/5798) | Go timers stall on Cloudflare | `go/worker/tinygo-clock.mjs` | Delete it |
| [tinygo-org/net#56](https://github.com/tinygo-org/net/pull/56) | `http.ServeMux` has no method patterns | `humaworkers` routes itself | Use Huma's adapter |
| [tinygo#3599](https://github.com/tinygo-org/tinygo/issues/3599) | No `reflect.StructOf` | `humaworkers.Config` drops the Huma hook | Keep Huma's defaults |
| [tinygo#3862](https://github.com/tinygo-org/tinygo/issues/3862) | No `reflect.Value.MethodByName` | Uploads take `multipart.Form` | Use `huma.MultipartFormFiles[T]` |
| [workers-go#97](https://github.com/syumai/workers-go/issues/97) | No WebSocket upgrade from Go | `go/worker/websocket.mjs` | Upgrade in Go |
| [workers-go#220](https://github.com/syumai/workers-go/issues/220) | No Durable Object in Go | `go/worker/hub.mjs` | Write the hub in Go |
| [fern#17936](https://github.com/fern-api/fern/issues/17936) | The SSE terminator matches as a substring | `[end-of-stream]`, refused in items | Drop the item rule |
| [fern#17937](https://github.com/fern-api/fern/issues/17937) | SDKs do not reconnect after a network reset | [The client rule](guides/streaming.md#the-client-rule) | Loop for planned ends only |
| [fern#17938](https://github.com/fern-api/fern/issues/17938) | `event: error` is read as an item | No SSE error events | Send them |
| [fern#17939](https://github.com/fern-api/fern/issues/17939) | The CLI prints `json` streams at the end; the terminator is not escaped | A plain-text terminator | |
| [fern#17775](https://github.com/fern-api/fern/issues/17775) | The TypeScript SDK sends only one of two header schemes (Access's two) | The tests pass both as `headers` | Drop the `headers` |
| [fern#9559](https://github.com/fern-api/fern/issues/9559) | OpenAPI 3.2.0 is rejected | `version: "3.1.1"` in `ts/src/specs.ts` | Use oRPC's default |
| [orpc#2115](https://github.com/middleapi/orpc/issues/2115) | oRPC generates no AsyncAPI | `ts/src/asyncapi.ts` | Publish it ([#23](https://github.com/joeblew999/charter/issues/23)) |
| Fern, not filed | A named form-encoded OAuth request schema breaks the Go SDK's test | The schema is inline (`conformance/showcase-go/api/contract.go`) | Drop that `RequestBody` |
| Huma, not filed | A `RawBody` beside a `Body` gives Fern's Go SDK an `io.Reader` | `jsonOnly` in `go/humaworkers/humaworkers.go` | Delete `jsonOnly` |
| Fern, not filed | An AsyncAPI server's `pathname` is ignored | Tests pass `baseUrl` | |
| `cf dev`, not filed | A refused WebSocket upgrade is dropped | The showcase test accepts either | |
| Fern, not filed | With `endpoint-security`, the TypeScript SDK's WebSocket client uses `_metadata` it never declares, and the SDK is not written | `auth: any` in the notes examples' `fern/generators.yml` | Route credentials per operation |
| Fern, not filed | An `auth` setting in `fern/generators.yml` marks every AsyncAPI channel as needing credentials | A TypeScript client with none passes `auth: false` (`test/sdk-live-test.mjs`, `test/soak.mjs`) | Drop `auth: false` |
| Fern, not filed | The CLI sends only the first credential it has (`AuthStrategy::Any`) | None: the CLI cannot pass Access | |
