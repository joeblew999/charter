# api/: an oRPC API with real-time, on Cloudflare

The TypeScript half of the repo (the Go half is [api-go.md](api-go.md); [README.md](README.md#what-is-what) says what is what). A notes API: an oRPC 2.0 (`2.0.0-beta.40`) Worker on D1, contract first. From the one contract, oRPC serves plain REST, SSE and a WebSocket, and generates the OpenAPI and AsyncAPI specs that Fern turns into SDKs, a CLI and docs (see [sdk.md](sdk.md)).

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

The tests it must pass are in [testing.md](testing.md), and its D1 schema is `../migrations/`; both are shared with the Go Worker.

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

The design is in [plans/realtime.md](plans/realtime.md), and the pattern to copy (the rules plus a client loop for each SDK) is in [sdk.md](sdk.md#real-time-on-cloudflare-the-pattern-to-copy). In short:
- D1 is the log, and the note id is the only position (`after`).
- The hub only wakes followers.
- `follow()` hides hub restarts.
- Streams are finite, and clients resume with `after`.

Verified live with `mise run api:soak`: 7 clients, a redeploy and a client drop, every note exactly once.

## Upstream issues

Every workaround in the code names its upstream issue. The table, and what to do when one closes, is in [upstream.md](upstream.md).
