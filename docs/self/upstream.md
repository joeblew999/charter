---
title: Upstream issues
nav_order: 16
---
# Upstream issues: every workaround, and the issue it waits for

`charter upstream` shows the state of each issue the code tags (`Upstream:`). A row without a link is not filed. When an issue closes, do what its row says, then `charter check` and the soak.

| Issue | Problem | Workaround | When fixed |
|---|---|---|---|
| [workers-sdk#16199](https://github.com/cloudflare/workers-sdk/issues/16199) | Under `cf dev`, a request answered before its body was read makes the next request on the connection fail (`fetch failed`, a 500 from the dev server): workerd up to 1.20261013.1 closes the connection, and miniflare does not retry | `charter-lib/serve` reads a body the answer did not (`ts/src/serve.ts`) | Drop that |
| [tinygo#5800](https://github.com/tinygo-org/tinygo/issues/5800) | The collector runs at every pause of a workers-go program. TinyGo 0.44 moved the threshold (`gc_finalizer_common.go`) and made the trigger grow with the finalizers, but still collects at a pause once it is reached | A runtime patch and an 8 MB heap (`internal/cli/build`) | Drop the patch |
| [tinygo#5798](https://github.com/tinygo-org/tinygo/issues/5798) | Go timers stall on Cloudflare. Fixed in TinyGo 0.44 ([#5818](https://github.com/tinygo-org/tinygo/pull/5818)), but a Worker runs workers-go's own copy of `wasm_exec.js`, which v0.36.0 has without the fix | `worker/tinygo-clock.mjs` | Delete it once workers-go ships TinyGo 0.44's `wasm_exec.js` |
| [tinygo#5684](https://github.com/tinygo-org/tinygo/issues/5684), [#5888](https://github.com/tinygo-org/tinygo/issues/5888), [#5889](https://github.com/tinygo-org/tinygo/issues/5889), [net#106](https://github.com/tinygo-org/net/issues/106), [net#95](https://github.com/tinygo-org/net/issues/95) | The official Go MCP SDK does not build with TinyGo 0.43.0: `hash/maphash` with Go 1.27, `reflect.VisibleFields`, `os.OpenRoot`, two `net/http` APIs and `net.Dialer.Control` | `charter/mcp` serves MCP itself | When all are closed, try the official SDK (`mcp/mcp.go`) |
| [tinygo-org/net#56](https://github.com/tinygo-org/net/pull/56) | `http.ServeMux` has no method patterns | `humaworkers` routes itself | Use Huma's adapter |
| [tinygo#3599](https://github.com/tinygo-org/tinygo/issues/3599) | No `reflect.StructOf` | `humaworkers.Config` drops the Huma hook | Keep Huma's defaults |
| [tinygo#3862](https://github.com/tinygo-org/tinygo/issues/3862) | No `reflect.Value.MethodByName` | Uploads take `multipart.Form` | Use `huma.MultipartFormFiles[T]` |
| [workers-go#97](https://github.com/syumai/workers-go/issues/97) | No WebSocket upgrade from Go | `worker/websocket.mjs` | Upgrade in Go |
| [workers-go#220](https://github.com/syumai/workers-go/issues/220) | No Durable Object in Go | `worker/hub.mjs` | Write the hub in Go |
| [fern#17936](https://github.com/fern-api/fern/issues/17936) | The SSE terminator matches as a substring | `[end-of-stream]`, refused in items | Drop the item rule |
| [fern#17937](https://github.com/fern-api/fern/issues/17937) | SDKs do not reconnect after a network reset | [The client rule](streaming.md#the-client-rule) | Loop for planned ends only |
| [fern#17938](https://github.com/fern-api/fern/issues/17938) | `event: error` is read as an item | No SSE error events | Send them |
| [fern#17939](https://github.com/fern-api/fern/issues/17939) | The CLI prints `json` streams at the end; the terminator is not escaped | A plain-text terminator | Any terminator text would do |
| [fern#17775](https://github.com/fern-api/fern/issues/17775) | The TypeScript SDK sends only one of two header schemes (Access's two) | The tests pass both as `headers` | Drop the `headers` |
| [fern#9559](https://github.com/fern-api/fern/issues/9559) | OpenAPI 3.2.0 is rejected | `version: "3.1.0"` in `ts/src/specs.ts` | Use oRPC's default |
| [fern#18316](https://github.com/fern-api/fern/issues/18316) | An enum `grant_type` in the OAuth token request: the Go SDK's token client passes a `*string` where the enum is expected, and does not compile | `grant_type` is a string; the conformance servers refuse any other grant (400) | Make it an enum |
| [fern#18200](https://github.com/fern-api/fern/issues/18200) | A named form-encoded OAuth request schema breaks the Go SDK's test | The schema is inline (`examples/conformance-go/api/contract.go`) | Drop that `RequestBody` |
| [huma#1132](https://github.com/danielgtaylor/huma/issues/1132) | A `RawBody` beside a `Body` documents a binary request body the operation refuses (415), and Fern's Go SDK takes it: an `io.Reader` | `jsonOnly` in `humaworkers/humaworkers.go` | Delete `jsonOnly` |
| [typescript-sdk#3032](https://github.com/modelcontextprotocol/typescript-sdk/issues/3032) | MCP's TypeScript SDK takes a schema whose root is a `$ref` (a named zod schema) for no object: on a 2025 connection the result is wrapped as `{ result }` | `objectRoot` in `ts/src/mcp.ts` inlines the root's `$ref` | Drop that inlining; `objectRoot`'s other two jobs stay |
| [typescript-sdk#3033](https://github.com/modelcontextprotocol/typescript-sdk/issues/3033) | MCP's TypeScript SDK client refuses authorization-server metadata without `authorization_endpoint`, which RFC 8414 does not ask of a server with only client credentials, so `ClientCredentialsProvider` cannot sign in to one | None on the server: the conformance MCP test follows the chain by hand (`examples/conformance-ts/test/mcp.ts`) | Sign in with `ClientCredentialsProvider` there |
| [fern#17997](https://github.com/fern-api/fern/issues/17997) | With `endpoint-security`, the TypeScript SDK's WebSocket client uses `_metadata` it never declares, and the SDK is not written | `auth: any` in every `fern/generators.yml` (`internal/fern` writes it) | Route credentials per operation |
| [fern#17998](https://github.com/fern-api/fern/issues/17998) | An `auth` setting in `fern/generators.yml` marks every AsyncAPI channel as needing credentials | A TypeScript client with none passes `auth: false` (`examples/notes-ts/test/sdk.ts`, `examples/notes-go/test/soak.ts`) | Drop `auth: false` |
| [fern#17996](https://github.com/fern-api/fern/issues/17996) | The CLI sends only the first credential it has (`AuthStrategy::Any`) | None: the CLI cannot pass Access | The CLI passes Access |
| [fern#18313](https://github.com/fern-api/fern/issues/18313) | A generated TypeScript SDK's `Headers` does not typecheck with `@types/node` 26 | Every project stays on `@types/node` 22 (its `package.json`; `internal/fern` proves the SDK with it) | Move to `@types/node` 26 |
