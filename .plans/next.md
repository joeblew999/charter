# Next

Where this repo stands (2026-09-30) and what comes next, in order. What's proven is in FINDINGS.md.

## Where we are

- **api/:** an oRPC 2.0.0-beta.40 Worker on D1, contract first.
  - Both specs are generated.
  - `follow()` serves SSE and WebSockets.
  - `mise run api:soak` is green for 7 clients × (planned end, hub restart, client drop, 20 min idle).
- **sdk/:** Fern generates Go and TypeScript SDKs, a Rust CLI and docs from the specs. The TypeScript SDK runs inside Workers (`sdk/harness`).
- **Upstream:** six issues are tracked (Fern #17936–#17939 and #9559, oRPC #2115); see `mise run upstream:status`.

## Next

1. **The real-time plan's remaining steps:** see [realtime.md](realtime.md). Document a client loop per client (done in sdk/README.md) and follow the upstream issues.
2. **The AsyncAPI generator upstream:** see [asyncapi.md](asyncapi.md) and middleapi/orpc#2115. If they want it, port it as `@orpc/asyncapi`; if they ship their own, switch to it.
3. **Ship the generated output.** Today it's only in `sdk/out/`, which is gitignored.
   - TypeScript SDK → npm. Go SDK → its own module repo. CLI → a repo with cargo-dist releases (7 targets).
   - Fern's `output: location: github` per group, so `sdk:gen` opens a PR in each SDK repo.
4. **Settle Fern's licensing.** Its docs call local generation, WebSocket clients, webhook signatures and the CLI generator Enterprise or early access. All of it ran here without a `FERN_TOKEN`.
5. **Apply it to our real projects:** a contract with `openapi()`/`asyncapi()` metadata, the copied `follow.ts`/`asyncapi.ts`/`specs.ts`, a Fern folder and the tasks (README.md, "Using it in another project").
6. **Worker-to-Worker through service bindings:** pass `env.X.fetch` as the SDK's `fetch` (no public URL). Not tested yet.
7. **CI:** `mise run check` runs on every push and pull request (`.github/workflows/check.yml`, Ubuntu with Docker; green since 2026-09-30). Still to do: cargo-dist for CLI releases.
8. **Move to oRPC 2.0.0 final** when it ships (we're on the beta).
9. **Flue** (issue #1): message channels and agents on Cloudflare with oRPC/Fern. Start small.
