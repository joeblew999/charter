# sdk/

Typed SDKs, a CLI and a docs site from OpenAPI specs, with **[Fern](https://buildwithfern.com)** (the `fern-api` npm package, running its generators locally in Docker). Fern is also what Cloudflare's own [Forge](https://github.com/cloudflare/forge) builds `cf` on. Forge itself isn't used here, only its spec of the Cloudflare API, in `sdk:cloudflare`.

`fern/` is a **standard Fern project**, so Fern's own docs apply as they are:

```
fern/
├── fern.config.json          organization + CLI version ("*" = the local CLI)
├── docs.yml                  the docs site: which APIs, titles (mise run sdk:docs)
└── apis/
    ├── petstore/             one folder per API
    │   ├── openapi.json      the spec
    │   └── generators.yml    one group per language, options under config:
    ├── modern/               SSE streaming (x-fern-streaming) + cursor pagination (x-fern-pagination)
    └── showcase/             every feature below, in one API
```

What Fern does through **standard options only**: the spec's OpenAPI features, `x-fern-*` extensions and `generators.yml`. All of it was verified in `showcase/` on 2026-09-29 (Go: build, vet and tests; TypeScript: typecheck).

| Feature | How to switch it on | What the Go SDK gets |
|---|---|---|
| OAuth client credentials | `auth-schemes:` in generators.yml plus a form-encoded token endpoint | `option.WithClientID/WithClientSecret`; token fetched and reused |
| Idempotency | `x-fern-idempotency-headers` plus `x-fern-idempotent: true` | `option.WithIdempotencyKey` |
| Retries | built in (per endpoint: `x-fern-retries`) | `option.WithMaxAttempts` |
| Cursor/offset pagination | `x-fern-pagination` | `*core.Page[...]`, auto-paging |
| SSE / streaming | `x-fern-streaming: { format: sse }` | `core.Stream[T]` |
| File upload | `multipart/form-data` body | typed `UploadFile(...)` |
| Webhooks | OpenAPI 3.1 `webhooks:` | typed payload structs |
| Webhook signatures | `x-fern-webhook-signature` (HMAC or asymmetric) | `WebhooksHelper.verifySignature(...)` (TypeScript), `webhooks_helper.go` (Go). Fern lists this as Enterprise; it generated locally here |
| WebSockets | an AsyncAPI spec beside the OpenAPI one (for `api/`, generated from the contract), plus TypeScript `generateWebSocketClients: true` | Go: message types only. TypeScript: a reconnecting `LiveNotesSocket` with `connect`, typed `sendSubscribe`, `on('message')` and `close` (needs the `ws` package on Node). Fern lists WS clients as Enterprise; it generated locally here |
| Audiences | `x-fern-audiences` on endpoints plus `audiences: [public]` on a group | one spec gives a full SDK and a public SDK (group `typescript-public` has no internal `uploadFile`) |
| Overlays | `overlays: overlays.yml` beside the spec (OpenAPI Overlay 1.0) | changes the SDK without editing the spec: `notes.listNotes` becomes `notes.list` |

## Does the TypeScript SDK work on Cloudflare Workers? Yes (verified 2026-09-29)

`sdk/harness/` is a small cf Worker project. It implements the showcase API itself (`/api/mock/*`), and `/api/sdk-test` runs the generated SDK against it inside workerd.

```sh
mise run sdk:harness:test                             # under cf dev
mise run sdk:harness:deploy                           # cf deploy: orpc-sdk-harness + orpc-sdk-harness-api
mise run sdk:harness:test --remote                    # on Cloudflare
```

All pass in both places:
- auto-pagination over 3 pages;
- OAuth client credentials (token fetched form-encoded, then reused);
- idempotent create (`Idempotency-Key` plus bearer token);
- an SSE stream of typed chunks;
- webhook HMAC verification (valid signature accepted, forged one rejected);
- the **WebSocket client**, both inside a Worker (on Cloudflare it connects to `orpc-sdk-harness-api`) and from Node over the network, with typed events and the bearer token received.

WebSocket auth in a Worker: the SDK sends the token as a handshake header, which Workers (like browsers) can't set, so from a Worker it never arrives. Fern's docs confirm headers are the only built-in way. The fix uses standard SDK calls: `client.auth.getToken(...)`, then `liveNotes.connect({ queryParams: { access_token } })`, and the server accepts either the header or `?access_token=`.

Two standard options make it Workers-ready:
- **`guardProcessEnvAccess: true`.** Workers have no `process`, and the OAuth code reads `process.env`.
- **`outputSourceFiles: false`** (group `typescript-dist`). Fern compiles the SDK to `.js` plus `.d.ts`, like an npm package. The raw `.ts` source clashes with Cloudflare's Worker types (`Headers`, `Response`); the compiled package typechecks cleanly.

One Cloudflare detail: a Worker calling another Worker on the same `workers.dev` zone needs the `global_fetch_strictly_public` compatibility flag. Without it you get error 1042, and a Worker can never call its own URL.

## A CLI for your API (Fern's CLI generator, Rust)

Group `cli` in `petstore/generators.yml`. Fern's docs call it early access with a `FERN_TOKEN`, but `mise run sdk:gen petstore cli` generated it locally without one (56 s, in Docker). The output is a Rust workspace with a cargo-dist config for 7 targets: macOS, Linux gnu/musl and Windows msvc.

Built and tried on 2026-09-29. The macOS build took 55 s (the first build compiles all dependencies) and gave an 11.7 MB arm64 binary. Against a local mock:
- **Commands per resource:** `petstore pets list-pets`, `create-pet --name rex`.
- **Output and requests:** `--format json|table|yaml|csv|jsonl|http`, `--query` (JMESPath), `--dry-run` (shows the request without sending it), `--base-url`, `--debug`.
- **For agents:** `--schema` gives the command surface as JSON, and `generate-skills` writes Claude-style `SKILL.md` files for the CLI (one shared file, one per resource).
- **Also included:** `auth` login, shell completions and a man page.

**The showcase CLI against the deployed Worker** (`showcase` group `cli`, 2026-09-29):
- These work: OAuth client credentials (token fetched automatically; `--debug` shows `authorization: [REDACTED]`), `notes list`, `notes list --page-all` (3 pages), `notes create --idempotency-key`, `chat` streaming SSE chunks live, and `files upload-file`.
- The OAuth token URL is taken from the spec's `servers` entry, and `--base-url` doesn't move it. So the spec's server is the real API (here, the deployed mock).
- No WebSockets: the CLI generator supports OpenAPI and GraphQL only, not AsyncAPI (Fern's docs; tried with AsyncAPI 3.0 and 2.6).
- A generator bug to avoid: a paginated method on the *root* client (an overlay renaming it with no `x-fern-sdk-group-name`) breaks the Rust build. Keep methods in a group.

Building is **heavy** the first time:
- `mise run sdk:cli:build -linux sdk/out/petstore/cli`: Linux, inside Docker (the container's architecture).
- `mise run sdk:cli:build sdk/out/petstore/cli`: this machine, native (needs Rust).
- All 7 platforms: cargo-dist on GitHub Actions, one native runner per OS, as Fern sets it up. A Linux container can't build macOS binaries (no Apple SDK) or the `windows-msvc` target.

## From an oRPC Worker (api/) to SDKs and a CLI (verified 2026-09-30)

`api/` is an oRPC 2.0 (`2.0.0-beta.40`) Worker, contract first (`api/src/contract.ts`: routes plus Zod 4 schemas). It's implemented on D1 and serves `/api/openapi.json` and `/api/asyncapi.json`. The chain:

```sh
mise run api:spec                  # contract -> sdk/fern/apis/api/{openapi,asyncapi}.json (offline; server = deployed URL)
mise run api:check                 # typecheck + unit tests + both specs match the contract (part of mise run check)
mise run sdk:gen api go            # + typescript, cli
mise run sdk:check sdk/out/api/go
mise run sdk:cli:build sdk/out/api/cli       # then: orpc-api notes list --page-all / notes watch
mise run api:live-test             # SSE + WebSocket, raw and through the SDK
mise run api:soak                  # the real-time matrix: every client x scenario (redeploys orpc-api)
```

- **Results:** the Go SDK (build, vet, tests) and the TypeScript SDK (typecheck) both pass. The CLI runs against the live Worker: `meta hello`, `notes create`, `notes list --page-all` across pages, and `notes watch --after`.
- **Both specs come from the contract; nothing is hand-written.**
  - OpenAPI: each route's `openapi({ operationId, tags, spec })` metadata adds the SDK group and method names, `x-fern-pagination`, `x-fern-streaming`, and the SSE note schema.
  - AsyncAPI: the WebSocket channel is a contract procedure (`notes.live`) marked with `asyncapi({ channel, address })` (`api/src/asyncapi.ts`, built on oRPC's public APIs). Its input becomes the channel's query parameters, and Fern's TypeScript SDK gets `liveNotes.connect({ after })`.
- **Make cursors strings in the contract:** with a numeric `next_cursor`, the CLI's `--page-all` stopped after page one.
- **OpenAPI version:** 2.0 defaults to 3.2.0, which `fern check` rejects, so both generators ask for 3.1.1 (`api/src/specs.ts`).
- **Where output goes:** here, `sdk/out/` (gitignored). For real use, SDKs ship as packages or repos (npm, a Go module repo, CLI releases). Fern's `output: location: github` can write to those repos.

### The same from a Go Worker (api-go/) (verified locally 2026-10-01)

`sdk/fern/apis/api-go/` is the same API with its specs written by the Go contract (`api-go/api/contract.go`, Huma) instead of the oRPC one. Its `generators.yml` has the same groups and names, so the same test programs run against either SDK.

```sh
mise run api-go:spec               # Go contract -> sdk/fern/apis/api-go/{openapi,asyncapi}.json
mise run sdk:gen api-go go         # + typescript, typescript-dist, cli
mise run sdk:check sdk/out/api-go/go
mise run sdk:cli:build sdk/out/api-go/cli
```

What Fern makes of Huma's spec differs in one way: Huma names its schemas (`components.schemas.Note`), so the SDKs get a shared `Note` type where the oRPC spec gives one type per response.

### Real-time on Cloudflare: the pattern to copy (.plans/realtime.md)

Verified live with `mise run api:soak`: 7 clients, a redeploy and a client drop, 50/50 notes each, no gaps or duplicates. The rules:

1. **One log, one cursor.** D1 is the source of truth, and the note id is the only position: list `cursor`, stream `after`, the SSE `id:`, and the `id` in every WebSocket message.
2. **The hub is disposable.** `NotesHub` (oRPC's `DurablePublisherObject`) only wakes followers. It's hibernatable, keeps no resume log, and may restart at any time.
3. **One `follow()`** (`api/src/follow.ts`, unit-tested). It subscribes, catches up from D1, then goes live, deduping by id. It resubscribes when the hub drops, and re-reads D1 when idle, in case the hub closed silently. Copy it unchanged: it only needs a `subscribe`/`since`/`latest` source.
4. **Transports are thin adapters** declared in the contract: SSE `notes.watch({ after, seconds })` and WebSocket `notes.live({ after })`.
5. **Streams are finite and never fail silently.** SSE ends after `seconds` with the terminator `[end-of-stream]`. If the hub stays down, SSE ends without the terminator and the WebSocket closes with 1011. No error events are sent, because generated clients read them as notes.
6. **Use Fern's options, not custom client code.** `x-fern-streaming: { format: sse, terminator, resumable: true }` is set in the contract, so the TypeScript and Go SDKs reconnect by themselves with `Last-Event-ID` after a clean end without the terminator.

**The client rule, everywhere: when the stream ends or fails, call again with `after` = the last note id.** The SDKs already reconnect after a clean drop. The loop covers the planned end and network resets, which the SDKs throw.

```ts
// TypeScript SDK (SSE)
let after: string | undefined;
for (;;) {
  try { for await (const note of await client.notes.watch({ after, seconds: 60 })) { handle(note); after = String(note.id); } }
  catch { await new Promise(r => setTimeout(r, 1000)); } // network reset: back off, then resume
}
// TypeScript SDK (WebSocket): on close, connect again with after
const socket = await client.liveNotes.connect({ after, reconnectAttempts: 0 });
```

```go
// Go SDK
for {
	stream, err := c.Notes.Watch(ctx, &orpcapi.WatchNotesRequest{After: after})
	if err == nil {
		for note, err := stream.Recv(); err == nil; note, err = stream.Recv() { handle(note); a := strconv.Itoa(note.ID); after = &a }
		stream.Close()
	}
	time.Sleep(time.Second)
}
```

```sh
# CLI: prints a stream's notes when it ends (json/jsonl), or live with --format raw
after=""
while :; do
  out=$(orpc-api notes watch ${after:+--after "$after"} --seconds 60 --format jsonl)
  [ -n "$out" ] && { echo "$out"; after=$(echo "$out" | tail -1 | jq -r .id); }
done
```

A browser's `EventSource` needs nothing: the SSE id is the note id, so its automatic `Last-Event-ID` is the same position.

**Known client gaps (Fern), all covered by the rules above:** the table of upstream issues, the workaround for each, and what to change when it's fixed is in [../api/README.md](../api/README.md#upstream-issues-workarounds-to-remove-when-theyre-fixed). `mise run upstream:status` shows which are fixed.

Everything runs through mise from the repo root:

```sh
mise run doctor                         # check the setup
mise run setup                          # npm packages, including Fern (fern-api) into sdk/node_modules
mise run sdk:list                       # APIs and their groups
mise run sdk:check-spec petstore        # fern check
mise run sdk:gen petstore go            # fern generate --local -> out/petstore/go
mise run sdk:check sdk/out/petstore/go   # Go: build + vet + tests on WireMock; TS: typecheck
mise run sdk:demo                       # Go + TypeScript for petstore, checked
mise run sdk:docs                       # API docs site for all APIs: http://localhost:3030 (fern docs dev)
mise run sdk:clean                      # remove out/, stop leftover containers
```

**To add an API,** copy `fern/apis/petstore/` to a new folder, replace `openapi.json`, and adjust `generators.yml`. That covers output paths, the Go module and each generator's options. The options for each language are documented at `buildwithfern.com/learn/sdks/generators/<lang>/configuration`. Here we use:

- `namespaceExport`, which names the TypeScript client (e.g. `PetstoreClient`);
- `module` and `packageName` for Go.

Good to know:

- **Versions:** the generators are pinned to the versions Cloudflare uses in Forge. Newer versions exist.
- **`sdk:cloudflare` is heavy.** It downloads Cloudflare's 26 MB spec and slices the chosen products into `fern/apis/cloudflare/`.
- **Licensing:** Fern's docs call local generation an Enterprise feature that needs a `FERN_TOKEN`. It has run here without one.
