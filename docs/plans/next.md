# Next

Where this repo stands (2026-10-01) and what comes next, in order. What's proven is in docs/findings.md.

## Where we are

- **api/:** an oRPC 2.0.0-beta.40 Worker on D1, contract first.
  - Both specs are generated.
  - `follow()` serves SSE and WebSockets.
  - `mise run api:soak` is green for 7 clients × (planned end, hub restart, client drop, 20 min idle).
- **sdk/:** Fern generates Go and TypeScript SDKs, a Rust CLI and docs from the specs. The TypeScript SDK runs inside Workers (`sdk/harness`).
  - **The showcase is contract first** (2026-10-01): one oRPC contract (`sdk/harness/src/contract.ts`) with every Fern feature we use: OAuth, idempotency, pagination, SSE, file upload, webhooks with signatures, a WebSocket the client also sends on, audiences. Its specs are generated, the harness Worker serves it, and the hand-written `asyncapi.yml` is gone.
- **api-go/:** the same API as a Go Worker: Huma on workers-go, built with TinyGo (added 2026-10-01).
  - The contract is Go (`api-go/api/contract.go`), and both specs are generated from it into `sdk/fern/apis/api-go/`.
  - Fern generates the Go and TypeScript SDKs and the CLI from them; a test keeps the SDK surface equal to the oRPC contract's.
  - Deployed as `orpc-api-go` and verified on Cloudflare: live test 5/5, soak 7/7 through a redeploy, a client drop and 20 minutes idle.
- **Upstream:** six issues are tracked (Fern #17936–#17939 and #9559, oRPC #2115); see `mise run upstream:status`.

## Next

000. **The showcase, what is left** ([../sdk.md](../sdk.md#the-showcase-is-contract-first-orpc-verified-2026-10-01)):
   - **Deploy the harness** (`mise run sdk:harness:deploy`, then `mise run sdk:harness:test -remote`). The deployed one is still the plain mock; the oRPC one has only run under `cf dev`.
   - **Tell oRPC what its generators can't say:** a form-encoded request body, `security` per operation, document-level settings from the contract, OpenAPI `webhooks`. The list and what we do instead is in sdk.md. Nothing is filed yet.
   - **Tell Fern** that its generators ignore an AsyncAPI server's `pathname`. Not filed yet.
   - **The same contract in Go** (`sdk/fern/apis/showcase-go/`, in progress on another branch): keep its names equal to this one's. `NoteEvent` gained an optional `auth` here.
00. **Promised on middleapi/orpc#2115 (2026-10-01):** publish `api/src/asyncapi.ts` as a community package (`asyncapi()` + `AsyncAPIGenerator`, API-compatible with `openapi()` / `OpenAPIGenerator`), then link it on the issue. It now also writes `send` operations (what the client sends), and `api/src/specs.ts` writes OpenAPI `webhooks`: both belong in the package. The maintainer prefers community packages first. An MCP generator for oRPC, built the same way, was offered too.
0. **The Go Worker (api-go/), what is left:**
   - **Make it cheaper:** 40 to 70 ms of CPU per request today. The plan is [performance.md](performance.md).
   - **Upstream:** the timer and `ServeMux` findings are filed (tinygo-org/tinygo#5798, #5799) and tracked in [../upstream.md](../upstream.md). The timer one matters to every workers-go project: tell syumai/workers-go, which ships the file, once TinyGo answers.
   - **A native database:** `api.SQLStore` is `database/sql`, so a SQLite driver in `platform_other.go` gives the native build persistence.
   - **MCP from the same contract** is built (`api-go/humamcp`, `/api/mcp`) and passes on Cloudflare. Left, in [mcp.md](mcp.md): authorization for the endpoint, a run with a model behind the client, and the oRPC side (`api/src/mcp.ts`, built like `asyncapi.ts`).
   - **Go client and server on separate Workers:** [microservices.md](microservices.md), including a try of [humaclient](https://github.com/danielgtaylor/humaclient) beside Fern.
1. **The real-time plan's remaining steps:** see [realtime.md](realtime.md). Document a client loop per client (done in docs/sdk.md) and follow the upstream issues.
2. **The AsyncAPI generator upstream:** see [asyncapi.md](asyncapi.md) and middleapi/orpc#2115. If they want it, port it as `@orpc/asyncapi`; if they ship their own, switch to it.
3. **Ship the generated output.** Today it's only in `sdk/out/`, which is gitignored.
   - TypeScript SDK → npm. Go SDK → its own module repo. CLI → a repo with cargo-dist releases (7 targets).
   - Fern's `output: location: github` per group, so `sdk:gen` opens a PR in each SDK repo.
4. **Settle Fern's licensing.** Its docs call local generation, WebSocket clients, webhook signatures and the CLI generator Enterprise or early access. All of it ran here without a `FERN_TOKEN`.
5. **Apply it to our real projects** (README.md, "Using it in another project"):
   - TypeScript: a contract with `openapi()`/`asyncapi()` metadata, the copied `follow.ts`/`asyncapi.ts`/`specs.ts`, a Fern folder and the tasks.
   - Go (workers-go): a Huma contract, the copied `humaworkers`/`asyncapi`/`follow` packages and `worker/`, a Fern folder and the tasks.
6. **Worker-to-Worker through service bindings:** pass `env.X.fetch` as the SDK's `fetch` (no public URL). Not tested yet.
7. **CI and releases** (docs/dev.md, "GitHub workflows"): six workflows, written from templates in `dev/workflows/`, that only call mise tasks.
   - Checks on every push to main and pull request: `api-check`, `sdk-check`, `dev-check`.
   - `api-deploy` is by hand. It needs the secrets `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`; they aren't set, so it has never run.
   - A version tag releases the `dev` binaries, the SDK sources, the specs and the Linux CLIs, and tags the Go modules (`api-go/vX.Y.Z`, `dev/vX.Y.Z`). No version has been cut yet.
   - Still to do: cargo-dist for the CLI on macOS and Windows; the SDKs as packages (item 3); pin mise itself in CI.
8. **Move to oRPC 2.0.0 final** when it ships (we're on the beta).
9. **Flue** (issue #1): message channels and agents on Cloudflare with oRPC/Fern. Start small.
