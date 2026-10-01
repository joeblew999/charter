---
title: Guides
nav_order: 3
has_children: true
---

# Guides: how to do one thing

Each guide is a task, start to finish, in a project made by `dev new` ([Getting started](getting-started.md)).

| Guide | You want to |
|---|---|
| [Define your API](guides/contract.md) | Add operations, inputs, outputs, validation and errors to the contract |
| [Real-time: SSE and WebSocket](guides/streaming.md) | Stream to clients without losing data across deploys and disconnects |
| [Auth, idempotency, uploads, webhooks](guides/fern-features.md) | Add what a production API needs, so the generated SDKs handle it |
| [Expose the API to AI agents (MCP)](guides/mcp.md) | Let an agent call your operations as tools |
| [Generate SDKs and a CLI](guides/sdks.md) | Give other developers a typed client in their language |
| [Deploy to Cloudflare](guides/deploy.md) | Put the Worker and its database live, and prove it works there |
| [Test locally and on Cloudflare](guides/testing.md) | Know a change is right before and after it ships |
| [CI and releases on GitHub](guides/ci-releases.md) | Check every push and publish versions |
| [A docs site for your repo](guides/docs-site.md) | Publish `docs/` with search, a sidebar and `llms.txt` |
| [The same in TypeScript (oRPC)](guides/typescript.md) | Build the API with oRPC instead of Go |
