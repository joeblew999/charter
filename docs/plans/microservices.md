---
title: Go microservices
nav_order: 5
parent: Plans
grand_parent: This repository
---
# Plan: Go client and server on separate Workers

An idea, saved for later. Nothing here is built.

## The idea

Make it very easy to build small Go Workers that talk to each other on Cloudflare: one Worker is the server (a Huma contract, as `api/go/`), another is a client of it, each small and deployed on its own. They already have what microservices usually lack:

- **A typed contract between them**, generated from Go.
- **Real-time sync that doesn't lose data** (`follow`, the hub, resume with `after`), so a client Worker can stay in step with a server Worker's data.
- **The tooling:** specs, drift checks, the same tests, one-line tasks, the GitHub workflows.

And anyone outside Cloudflare uses the Fern-generated SDKs and CLI to reach the same Workers, with no extra work.

## What it needs

1. **A Go client that runs inside a Worker.** Two candidates, to try both:
   - Fern's generated Go SDK under TinyGo on workers-go (its HTTP transport must be workers-go's `fetch`).
   - [humaclient](https://github.com/danielgtaylor/humaclient): a Go client generated from the Huma API itself, with no Docker and no spec file in between.
2. **Service bindings instead of public URLs.** Worker-to-Worker calls through a binding skip the public internet and Cloudflare's error 1042. The client's transport takes the binding's `fetch` (the TypeScript side of this is item 6 in [next.md](next.md), also untested).
3. **Streams between Workers.** A client Worker following a server Worker's feed is `follow` again, with the server's SSE or WebSocket as the live source. A stream held between two Workers counts against both.
4. **A second example** beside `api/go/`: a small Worker that consumes the notes API and exposes something of its own, with a soak test across the pair.
5. **Cost.** Every hop is a request to a Go Worker: 1 to 3 ms of CPU for a read ([../benchmarks.md](../benchmarks.md)). Service bindings avoid the network, not that.

## Open questions

- Does Fern's Go SDK compile and run under TinyGo? (The TypeScript SDK inside a Worker is proven: [../sdk.md](../sdk.md).)
- How does a Worker authenticate another Worker, and does that belong in the contract (`security` in OpenAPI)?
- One repo per service with `dev workflows -into .`, or several services in one repo?
