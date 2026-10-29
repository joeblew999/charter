---
title: Home
nav_order: 1
permalink: /
---

# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on Cloudflare Workers, in TypeScript ([oRPC](https://orpc.dev)) or Go ([Huma](https://huma.rocks)).

## Why

An API needs much more than its handlers: input checked, a spec, SDKs, tools for AI agents, sign-in, rate limits, streams that survive a deploy, tests on the real runtime, CI and docs. Each is usually set up by hand, differently in every repo, and drifts as the API changes.

With charter you write one thing, **the contract**, and the rest follows from it and is checked against it:

- **The specs** (OpenAPI, AsyncAPI), and a check that fails when they are stale.
- **SDKs** in any language [Fern](https://buildwithfern.com) supports, and a CLI.
- **MCP tools:** every operation that answers once is a tool, with the same checks and sign-in.
- **One standard:** every API serves, signs callers in and refuses the same way ([The standard API](standard.md)), tested on every run.
- **Go at TypeScript's cost** for a warm request, built with TinyGo ([Benchmarks](benchmarks.md)).
- **One way to work:** every command is an npm script, the same here and on GitHub.

The same APIs are kept here in TypeScript and in Go, and a program holds them to the same answers on every change.

## Every page

| Page | What it is |
|---|---|
| [Getting started](start.md) | A new API, deployed, with an SDK |
| [Your API](api.md) | The contract, handlers and storage; the database's typed code; MCP; `charter.toml` |
| [Streaming](streaming.md) | Streams that lose nothing across deploys |
| [SDKs](sdks.md) | Generating, proving and publishing them; Fern's features |
| [Auth](auth.md) | Who may call, and how they prove it |
| [Rate limits](ratelimits.md) | How often each caller may call |
| [Server-rendered pages](pages.md) | Pages from the same Worker, live over its streams |
| [Deploy and CI](deploy.md) | Cloudflare, secrets, the checks on GitHub |
| [Repos that use each other](repos.md) | Who pins what, breaking changes, Renovate |
| [Any repo](any-repo.md) | charter's docs, rules and checks in a repo you have |
| [Moving from an older charter](upgrade.md) | A repo on charter-ts, on charter's Go tool before 0.20, or on the library at `go/lib` or `go` |
| [The standard API](standard.md) | What every API serves, and how it refuses |
| [Libraries](libraries.md) | The Go packages and the TypeScript library |
| [How to help](contributing.md) | Set up, the CI, the tests, a release |
| [Rules](rules.md) | This repo's own rules |
| [Upstream issues](upstream.md) | What the code works around |
| [Benchmarks](benchmarks.md) | What was measured, where and when |

What to run, and when: [What to do](../charter/README.md). For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page.
