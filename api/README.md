# api/: an oRPC API with real-time, on Cloudflare

The TypeScript half of the repo (the Go half is [../api-go/](../api-go/README.md); the [root README](../README.md#what-is-what) says what is what). A notes API: an oRPC 2.0 (`2.0.0-beta.40`) Worker on D1, contract first. From the one contract, oRPC serves plain REST, SSE and a WebSocket, and generates the OpenAPI and AsyncAPI specs that Fern turns into SDKs, a CLI and docs (see [../sdk/README.md](../sdk/README.md)).

## Layout

| File | What it is |
|---|---|
| `src/contract.ts` | The API: every route, its input/output (Zod 4), and what Fern needs (`openapi({...})`, `asyncapi({...})`) |
| `src/index.ts` | The Worker: the contract implemented on D1, the SSE and WebSocket transports, `/api/openapi.json` and `/api/asyncapi.json` |
| `src/follow.ts` | `follow()`: the one gap-free real-time feed that both transports use |
| `src/hub.ts` | `NotesHub`: oRPC's `DurablePublisherObject`, a hibernatable live fan-out |
| `src/asyncapi.ts` | The AsyncAPI generator (oRPC has none yet) |
| `src/specs.ts` | Both specs from the contract, shared by `spec.ts` and the Worker |
| `spec.ts` | Writes `sdk/fern/apis/api/{openapi,asyncapi}.json` offline |
| `test/` | Unit tests for `follow()` |

The tests it must pass are in [../test/](../test/README.md), and its D1 schema is `../migrations/`; both are shared with the Go Worker.

## Tasks (from the repo root)

```sh
mise run api:dev               # local dev server on PORT (first time, in another shell: mise run api:migrate:local)
mise run api:deploy            # deploy orpc-api, then apply D1 migrations
mise run api:check             # typecheck + unit tests + both committed specs match the contract
mise run api:spec              # regenerate the specs after changing the contract
mise run api:live-test         # quick live check
mise run api:soak              # the real-time matrix (redeploys orpc-api); --idle 20 for the long-idle case
mise run upstream:status       # which upstream issues below are fixed
```

## Real-time

The design is in [../.plans/realtime.md](../.plans/realtime.md), and the pattern to copy (the rules plus a client loop for each SDK) is in [../sdk/README.md](../sdk/README.md#real-time-on-cloudflare-the-pattern-to-copy-plansrealtimemd). In short:
- D1 is the log, and the note id is the only position (`after`).
- The hub only wakes followers.
- `follow()` hides hub restarts.
- Streams are finite, and clients resume with `after`.

Verified live with `mise run api:soak`: 7 clients, a redeploy and a client drop, every note exactly once.

## Upstream issues (workarounds to remove when they're fixed)

Each workaround in the code carries a tag `Upstream: <repo>#<n> (when fixed: ...)`. `mise run upstream:status` finds the tags and shows each issue's state. When an issue closes, do what its tag says, then re-run `mise run api:check` and `mise run api:soak`.

| Issue | Problem | Workaround here | When it's fixed |
|---|---|---|---|
| [fern-api/fern#17936](https://github.com/fern-api/fern/issues/17936) | SSE terminator matched as a substring; the Go SDK applies `[DONE]` when none is declared | `END = "[end-of-stream]"` declared as the terminator, and note bodies reject it (`src/contract.ts`) | Drop the body rule |
| [fern-api/fern#17937](https://github.com/fern-api/fern/issues/17937) | `resumable` SDKs don't reconnect after a network reset, only after a clean end | Client rule: catch, then call again with `after` (`test/soak.mjs`, the client loops in `sdk/README.md`) | SDKs survive resets themselves; keep the loop for planned ends |
| [fern-api/fern#17938](https://github.com/fern-api/fern/issues/17938) | SSE `event:` ignored, so `event: error` is decoded as a note | `watch` never sends error events; it ends without the terminator (`src/index.ts`). The spec hook replaces oRPC's envelope with the note schema (`src/contract.ts`) | Error events could say why a stream ended; oRPC's envelope could be used as is |
| [fern-api/fern#17939](https://github.com/fern-api/fern/issues/17939) | CLI prints json/jsonl only when a stream ends; the Rust generator doesn't escape the terminator | The terminator is plain text (`src/contract.ts`); CLI latency is reported, not judged (`test/soak.mjs`) | Judge CLI latency like the SDKs' |
| [fern-api/fern#9559](https://github.com/fern-api/fern/issues/9559) | Fern rejects OpenAPI 3.2.0 (oRPC 2.0's default) | `version: "3.1.1"` (`src/specs.ts`) | Use oRPC's 3.2.0 default |
| [middleapi/orpc#2115](https://github.com/middleapi/orpc/issues/2115) | oRPC generates no AsyncAPI | `src/asyncapi.ts` (offered upstream in the issue) | Switch to oRPC's generator and delete the file |

The Go Worker carries the same tags for the Fern issues (`api-go/api/contract.go`, `api-go/api/handlers.go`), so `upstream:status` lists both places.
