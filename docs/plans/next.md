---
title: What is next
nav_order: 5
parent: How to help
---
# What is next: what is not built yet

The plan, in the order of its two milestones. Each item is an issue on GitHub: the issue says what done looks like, this page says why and what is known. What exists is on the other pages; what was proven is in [Findings](../findings.md).

## 0.8: one project shape

| Issue | What | State |
|---|---|---|
| [#17](https://github.com/joeblew999/charter/issues/17) | One project shape: the library, four example projects, the tool | [The restructure](structure.md) |
| [#18](https://github.com/joeblew999/charter/issues/18) | Docs: half the pages, one fact in one place | These pages |
| [#19](https://github.com/joeblew999/charter/issues/19) | The tool writes a project's collaboration files: issue forms, labels, contact links | Not started |

## 1.0: used by other repos

| Issue | What | What is known |
|---|---|---|
| [#20](https://github.com/joeblew999/charter/issues/20) | Move the projects already using it to charter | They were made from earlier releases, with the old layout ([how a project updates](../reference/releases.md#how-a-project-updates)) |
| [#21](https://github.com/joeblew999/charter/issues/21) | Drop the TinyGo patches as upstream fixes land | [Upstream issues](../upstream.md): one fix is on TinyGo's dev branch, one issue is open |
| [#22](https://github.com/joeblew999/charter/issues/22) | Cold start: the first use of each operation in a new isolate | [Performance plan](performance.md) |
| [#23](https://github.com/joeblew999/charter/issues/23) | Publish the AsyncAPI generator for oRPC as a package | Below |
| [#24](https://github.com/joeblew999/charter/issues/24) | Releases: SDKs as packages, the CLI for macOS and Windows | Below |
| [#25](https://github.com/joeblew999/charter/issues/25) | A soak test that works on any project's contract | Today `test/soak.mjs` knows the notes routes |
| [#26](https://github.com/joeblew999/charter/issues/26) | The native build with a real database | `api.SQLStore` is `database/sql`: a SQLite driver in `platform_other.go` gives it persistence |
| [#27](https://github.com/joeblew999/charter/issues/27) | MCP: authorization, and a run with a model behind the client | Below |
| [#28](https://github.com/joeblew999/charter/issues/28) | Go client and server on separate Workers | Below |
| [#29](https://github.com/joeblew999/charter/issues/29) | Fern: file what we found, and settle its licensing | Below |
| [#30](https://github.com/joeblew999/charter/issues/30) | Integrate with gsx | Not looked at |
| [#1](https://github.com/joeblew999/charter/issues/1) | Flue: message channels and agents on Cloudflare | Start small |

## The AsyncAPI generator, as a package

Built: `examples/notes-ts/src/asyncapi.ts` ([how it works](../guides/typescript.md#the-asyncapi-generator)). Left: publish it as a community package, as promised on middleapi/orpc#2115 (`asyncapi()` and `AsyncAPIGenerator`, API-compatible with `openapi()` and `OpenAPIGenerator`), with the `send` side and the OpenAPI `webhooks` writer. If oRPC wants it as `@orpc/asyncapi`, port it; if oRPC ships its own, switch to it. Open: which AsyncAPI versions Fern accepts (3.0.0 works); query parameters and a send side on one channel.

## SDKs as packages

A release attaches SDK sources and the Linux CLI to the GitHub Release; nothing is published as a package. Fern's `output: location: github` per group would open a pull request in a repo per SDK (not tried). The CLI for all platforms needs cargo-dist on GitHub Actions, one native runner per OS.

## MCP

Built for the Go server ([MCP](../guides/mcp.md)). Left:

1. **Connect a real host** (Claude, an IDE) to a deployed Worker and watch a model use the tools.
2. **Authorization:** MCP's own scheme. Needed before the endpoint fronts anything private.
3. **A `Host` allowlist** for a native server reachable from a browser.
4. **The oRPC side:** a tool per procedure that is not an event iterator, schemas from Zod. oRPC has no official MCP package.
5. **A plain Huma API as input:** `humamcp` needs a `*humaworkers.API` today.

## Go client and server on separate Workers

An idea; nothing is built. Small Go Workers that talk to each other: one serves a Huma contract, another is its client. It needs a Go client that runs inside a Worker (Fern's Go SDK under TinyGo, or [humaclient](https://github.com/danielgtaylor/humaclient)), service bindings instead of public URLs, streams between Workers (`follow` again), and a second example with a soak across the pair. Open: how a Worker authenticates another.

## Fern

- **File what was found and not filed** ([the list](../upstream.md#found-not-filed)), tell oRPC what its generators cannot say, and offer Huma the form format (`humaworkers.WithForm`).
- **Settle the licence.** Fern's docs call local generation, WebSocket clients, webhook signatures and the CLI generator Enterprise or early access, needing a `FERN_TOKEN`. All of it ran here without one.

## Smaller things

- **The showcase:** receive the webhook from the deployed Go showcase; run the Go SDK against it; check tokens in the oRPC showcase too.
- **Real-time:** a long-idle soak against the TypeScript Worker.
- **CI:** run the `deploy` workflow from GitHub (not checked so far); pin mise itself in CI.
- **oRPC 2.0.0 final,** when it ships.
