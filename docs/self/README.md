---
title: Home
nav_order: 1
permalink: /
---

# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on Cloudflare Workers, in TypeScript ([oRPC](https://orpc.dev)) or Go ([Huma](https://huma.rocks)).

## Why

An API is more than its handlers. It needs input checked, an OpenAPI spec, SDKs for its callers, tools for AI agents, sign-in, rate limits, streams that survive a deploy, tests on the real runtime, CI, and docs. Each is usually set up by hand, differently in every repo, and each drifts from the others as the API changes.

With charter you write one thing, **the contract**: the operations, their inputs and outputs, who may call them. Everything else follows from it and is checked against it:

- **The specs** (OpenAPI and AsyncAPI), written from the contract, and a check that fails when they are stale.
- **SDKs** in any language [Fern](https://buildwithfern.com) supports, and a CLI.
- **MCP tools:** every operation is also a tool an agent can call, with the same checks and the same sign-in.
- **The same answers everywhere:** every API serves its specs, signs callers in and refuses the same way ([The standard API](standard.md)), and a test built into charter checks it on every run.
- **Go at TypeScript's cost:** a Go API runs on Workers, built with TinyGo, and costs about what a TypeScript one does ([Benchmarks](benchmarks.md)).
- **One way to work:** every step is an npm script, the same on your machine and on GitHub, so a check that passes here passes there.

## Why you can trust it

This repo holds the same APIs written twice, once in TypeScript and once in Go. A program sends both the same requests, the right ones and every kind of wrong one, and fails if any answer differs. The conformance projects use every feature of the generated SDKs and test them against the API, on Cloudflare. All of it runs on every change ([How to help](contributing.md#the-ci)).

## Start

| You want | Go to |
|---|---|
| A new API | [Getting started](start.md) |
| charter's docs, rules and checks in a repo you have | [Any repo](any-repo.md) |
| To help | [How to help](contributing.md) |

## Every page

| Page | What it is |
|---|---|
| [Getting started](start.md) | From nothing to a deployed API with an SDK |
| [Your API](api.md) | The contract, handlers and storage; MCP; `charter.config.ts` |
| [Streaming](streaming.md) | Streams that lose nothing across deploys |
| [SDKs](sdks.md) | Generating, proving and publishing them; Fern's features |
| [Auth](auth.md) | Who may call: tokens, Cloudflare Access, OpenID Connect; rate limits |
| [Server-rendered pages](pages.md) | Pages from the same Worker, live over its streams |
| [Deploy and CI](deploy.md) | Cloudflare, secrets, the checks on GitHub |
| [Repos that use each other](repos.md) | Who pins what, breaking changes, Renovate |
| [Any repo](any-repo.md) | What a repo with no API takes |
| [The standard API](standard.md) | What every API serves, and how it refuses |
| [Libraries](libraries.md) | The Go packages and the TypeScript library |
| [How to help](contributing.md) | Set up, the CI, the tests, a release |
| [Rules](rules.md) | This repo's own rules |
| [Upstream issues](upstream.md) | What the code works around, and when it can stop |
| [Benchmarks](benchmarks.md) | What was measured, where and when |

What to run, and when: [What to do](../charter/README.md). For an agent: [llms.txt](https://joeblew999.github.io/charter/llms.txt) lists every page as Markdown.
