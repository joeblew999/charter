---
title: Plans
nav_order: 12
parent: This repository
has_children: true
---
# Next

What is not built yet, in order. What exists is on each part's page ([../README.md](../README.md) is the index), and what was proven is in [../findings.md](../findings.md). The numbers are kept as they were, because other pages refer to them.

000. **The showcase, what is left** ([../sdk.md](../sdk.md#the-orpc-showcase), [../showcase-go.md](../showcase-go.md)):
   - **Tell oRPC what its generators can't say:** a form-encoded request body, `security` per operation, document-level settings from the contract, OpenAPI `webhooks`. The list and what we do instead is in sdk.md. Nothing is filed.
   - **Tell Fern** that its generators ignore an AsyncAPI server's `pathname`. Not filed ([../upstream.md](../upstream.md#found-not-filed)).
   - **Receive the webhook from the deployed Go showcase.** It needs a receiver the Worker can reach (`SHOWCASE_WEBHOOK_URL` at deploy), and the Worker's secrets set instead of their defaults.
   - **Run the Go SDK against the Go showcase,** and build its Fern CLI.
   - **File the Fern Go generator bug** (a test that doesn't compile for a named form-encoded token request; [../upstream.md](../upstream.md#found-not-filed)), and offer Huma the form format (`humaworkers.WithForm`).
   - **Check tokens in the oRPC showcase too,** so both servers refuse the same requests and `--open` can go from the test.00. **Promised on middleapi/orpc#2115 (2026-10-01):** publish `api/src/asyncapi.ts` as a community package (`asyncapi()` + `AsyncAPIGenerator`, API-compatible with `openapi()` / `OpenAPIGenerator`), then link it on the issue. The `send` operations and the OpenAPI `webhooks` writer in `api/src/specs.ts` belong in the package. The maintainer prefers community packages first. An MCP generator for oRPC, built the same way, was offered too. See [asyncapi.md](asyncapi.md).
0. **The Go Worker (api-go/), what is left:**
   - **Make it cheaper still:** a read went from 40 to 70 ms of CPU to 1 to 3 ms, and the first request in a new isolate from 54 to 68 ms to about 10. What is left is in [performance.md](performance.md).
   - **Upstream:** the timer and `ServeMux` findings are filed (tinygo-org/tinygo#5798, #5799) and tracked in [../upstream.md](../upstream.md). The timer one matters to every workers-go project: tell syumai/workers-go, which ships the file, once TinyGo answers.
   - **A native database:** `api.SQLStore` is `database/sql`, so a SQLite driver in `api-go/platform_other.go` gives the native build persistence.
   - **MCP:** authorization for the endpoint, a run with a model behind the client, and the oRPC side. The list is in [mcp.md](mcp.md).
   - **Go client and server on separate Workers:** [microservices.md](microservices.md), including a try of [humaclient](https://github.com/danielgtaylor/humaclient) beside Fern.
1. **The real-time plan's remaining steps:** [realtime.md](realtime.md). Mostly following the upstream issues.
2. **The AsyncAPI generator upstream:** [asyncapi.md](asyncapi.md) and middleapi/orpc#2115. If they want it, port it as `@orpc/asyncapi`; if they ship their own, switch to it.
3. **Ship the generated output as packages.** A release attaches the SDK sources and the Linux CLIs to the GitHub Release ([../dev.md](../dev.md#cutting-a-release)); nothing is published as a package.
   - TypeScript SDK → npm. Go SDK → its own module repo. CLI → a repo with cargo-dist releases (7 targets).
   - Fern's `output: location: github` per group, so `sdk:gen` opens a PR in each SDK repo. Not tried.
4. **Settle Fern's licensing.** Its docs call local generation, WebSocket clients, webhook signatures and the CLI generator Enterprise or early access. All of it ran here without a `FERN_TOKEN`.
5. **Apply it to our real projects:**
   - TypeScript: the five steps in [../api.md](../api.md#copying-it-into-a-typescript-orpc-project).
   - Go (workers-go): `dev new` ([../dev.md](../dev.md#a-new-project-dev-new)).
6. **Worker-to-Worker through service bindings:** pass `env.X.fetch` as the SDK's `fetch` (no public URL). Not tested.
7. **CI and releases, what is left** ([../dev.md](../dev.md#github-workflows)):
   - **Run `api-deploy` from GitHub.** It needs the secrets `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` (`mise run cloudflare:secrets`). The deploys recorded in findings were made from a machine; whether the workflow has run was not checked.
   - **The Fern CLI on macOS and Windows:** cargo-dist on GitHub Actions.
   - **The SDKs as packages** (item 3).
   - **Pin mise itself in CI.**8. **Move to oRPC 2.0.0 final** when it ships (the repo is on `2.0.0-beta.40`).
9. **Flue** (issue #1): message channels and agents on Cloudflare with oRPC/Fern. Start small.
