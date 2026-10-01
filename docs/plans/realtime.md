---
title: Real-time: what is left
nav_order: 1
parent: Plans
grand_parent: This repository
---
# Plan: the real-time system (SSE + WebSockets on Cloudflare)

Tracked in issue #2. The design this plan led to is built on both Workers and is described in [../realtime.md](../realtime.md): the guarantee, the five rules that code comments cite by number, the feed, the client loops and the test matrix. This page keeps only what is left.

## Left

- **Follow the upstream issues.** The Fern gaps found on the way are filed and tracked in [../upstream.md](../upstream.md). Two things on the plan's list for Fern have no issue there: the CLI exits 0 after an SSE error event, and the SDKs don't expose event ids. The servers send no error events, and resume uses `after`, so neither blocks anything.
- **The AsyncAPI generator upstream:** [asyncapi.md](asyncapi.md).
- **A long-idle run against the oRPC Worker** (`mise run api:soak --idle 20`) has no entry in [../findings.md](../findings.md); the Go Worker's has.
- **If the hub's subscriber sockets become a limit:** one hub socket per Worker isolate, or a hub per topic. Not needed so far.
