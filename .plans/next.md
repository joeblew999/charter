# Next

Where this repo stands (2026-10-01) and what comes next, in order. What's proven is in FINDINGS.md.

## Where we are

- **api/:** an oRPC 2.0.0-beta.40 Worker on D1, contract first.
  - Both specs are generated.
  - `follow()` serves SSE and WebSockets.
  - `mise run api:soak` is green for 7 clients × (planned end, hub restart, client drop, 20 min idle).
- **sdk/:** Fern generates Go and TypeScript SDKs, a Rust CLI and docs from the specs. The TypeScript SDK runs inside Workers (`sdk/harness`).
- **api-go/:** the same API as a Go Worker: Huma on workers-go, built with TinyGo (added 2026-10-01).
  - The contract is Go (`api-go/api/contract.go`), and both specs are generated from it into `sdk/fern/apis/api-go/`.
  - Fern generates the Go and TypeScript SDKs and the CLI from them; a test keeps the SDK surface equal to the oRPC contract's.
  - Verified locally (native, and Wasm under workerd): live test 3/3, soak 7/7 without the redeploy. Not deployed yet.
- **Upstream:** six issues are tracked (Fern #17936–#17939 and #9559, oRPC #2115); see `mise run upstream:status`.

## Next

0. **Finish the Go Worker (api-go/):**
   - **Deploy it** (`mise run api-go:deploy`: a new Worker `orpc-api-go` and D1 database; needs a go-ahead), then run `api-go:live-test` and `api-go:soak` (the hub-restart scenario only exists deployed) and measure CPU time per request. Put the results in FINDINGS.md.
   - **Upstream issues for the three Huma/TinyGo workarounds** (`api-go/README.md`): `reflect.StructOf` in TinyGo, Huma's default hook that needs it, and TinyGo's `ServeMux` patterns; plus WebSocket upgrades in workers-go. Search for existing issues first, then tag the code (`Upstream: ...`). Filing is outward-facing: needs a go-ahead. A Huma fork is not needed so far.
   - **A native database:** `api.SQLStore` is `database/sql`, so a SQLite driver in `platform_other.go` gives the native build persistence.
   - **Try [humaclient](https://github.com/danielgtaylor/humaclient)** beside Fern: a Go client generated from the Huma API itself, with no Docker.
   - **An MCP server from the same contract:** Huma's operations and schemas are enough to list tools and validate their input, as with oRPC.
   - **CI:** `api-go:check` is in `mise run check`; confirm TinyGo and binaryen install on the Ubuntu runner.
1. **The real-time plan's remaining steps:** see [realtime.md](realtime.md). Document a client loop per client (done in sdk/README.md) and follow the upstream issues.
2. **The AsyncAPI generator upstream:** see [asyncapi.md](asyncapi.md) and middleapi/orpc#2115. If they want it, port it as `@orpc/asyncapi`; if they ship their own, switch to it.
3. **Ship the generated output.** Today it's only in `sdk/out/`, which is gitignored.
   - TypeScript SDK → npm. Go SDK → its own module repo. CLI → a repo with cargo-dist releases (7 targets).
   - Fern's `output: location: github` per group, so `sdk:gen` opens a PR in each SDK repo.
4. **Settle Fern's licensing.** Its docs call local generation, WebSocket clients, webhook signatures and the CLI generator Enterprise or early access. All of it ran here without a `FERN_TOKEN`.
5. **Apply it to our real projects** (README.md, "Using it in another project"):
   - TypeScript: a contract with `openapi()`/`asyncapi()` metadata, the copied `follow.ts`/`asyncapi.ts`/`specs.ts`, a Fern folder and the tasks.
   - Go (workers-go): a Huma contract, the copied `humaworkers`/`asyncapi`/`follow` packages and `worker/`, a Fern folder and the tasks.
6. **Worker-to-Worker through service bindings:** pass `env.X.fetch` as the SDK's `fetch` (no public URL). Not tested yet.
7. **CI:** `mise run check` runs on every push and pull request (`.github/workflows/check.yml`, Ubuntu with Docker; green since 2026-09-30). Still to do: cargo-dist for CLI releases.
8. **Move to oRPC 2.0.0 final** when it ships (we're on the beta).
9. **Flue** (issue #1): message channels and agents on Cloudflare with oRPC/Fern. Start small.
