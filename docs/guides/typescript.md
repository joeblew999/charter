---
title: TypeScript
nav_order: 8
parent: Guides
---

# TypeScript: charter-ts

A TypeScript API on Cloudflare Workers from an oRPC contract, with the specs, SDKs and tests that follow from it. Every step is an npm script that runs `charter-ts <step>`, and `charter.config.ts` says what the steps need. You need Node 26 and npm, plus Docker for the SDKs; no npm account.

## Start a project

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts new my-api --subdomain <your workers.dev subdomain>
cd my-api
npm install
npm run check
```

`--from notes` starts from the notes API (auth, streams, rate limits) instead of one route. The project comes with `.github/workflows/self-check.yml`, which runs `npm run check` on GitHub. Its `package.json` installs charter-ts from that release's own URL, with its version in it: another release is another URL, and Renovate opens that pull request.

## The steps

A project's `package.json` has the scripts a person runs:

| Script | What it does |
|---|---|
| `npm run dev` | The API under workerd on the config's port, its local database migrated; until Ctrl-C |
| `npm run check` | Every local check, in order, no Cloudflare account, and how long each part took: typecheck (the Worker, the config and the tests), the specs match the contract, Fern's check, `test/*.test.ts`, the config's tests under workerd. Fern's check, and in Go the TinyGo build, done before for the same sources come from charter's cache (`~/Library/Caches/charter` on a Mac); `npm run check -- --fresh` does everything again |
| `npm run spec` | Write `fern/openapi.json` and `fern/asyncapi.json` from the contract, after a change to it |
| `npm run sdk -- <group>` | A group of `fern/generators.yml` with Fern (Docker) into `sdk/out/<group>`; by default the TypeScript SDK |
| `fnox exec -- npm run deploy` | REMOTE: the Worker with its secrets, then its migrations |
| `fnox exec -- npm run live-test` | REMOTE: the config's live tests against the deployed Worker |

Every other step `check` is made of runs alone too: `npx charter-ts lint`, `spec:check`, `test:workerd`, `migrate:local`. `npx charter-ts` lists them all.

## charter.config.ts

```ts
import { defineConfig } from "charter-ts/config";
import { specsOf } from "charter-lib/serve";
import { contract, document } from "./src/contract.ts";

export default defineConfig({
	worker: "my-api",
	url: process.env.API_URL ?? "https://my-api.me.workers.dev",
	port: 5178,
	specs: specsOf(contract, document),
	test: { run: ({ origin, node }) => node("test/live-test.ts", origin) },
	live: ({ url, node }) => node("test/live-test.ts", url),
});
```

Every field is documented in [`Config`](https://github.com/joeblew999/charter/blob/main/charter-ts/src/config.ts): secrets, throwaway local secrets, more environment for the tests, more deploy modes, the SDK copied into a Worker, and the most the Worker may be as a deploy uploads it (`bundleLimit`: 3 MB gzipped, what Workers Free allows, by default). A test run gets the server's port and three more free ones (`extras`) for what the tests serve, and `serve` names programs that run beside the server for its whole run, such as a webhook's receiver: the conformance projects serve a token issuer on one port and a receiver on another (`ts/conformance/charter.config.ts`).

## The library

What the Worker imports is `charter-lib` (`ts/lib/`), in `dependencies`; `charter-ts`, the tool, is in `devDependencies`. Both come from the same release, at the same version. The library is built with the web's globals and no Node types, so nothing in it can need Node: it runs on Workers.

| Import | What it is |
|---|---|
| `"charter-lib/serve"` | `serve(router, { document, trusted, limits })`: the API the standard way, every part below wired ([The standard API](../reference/standard.md)); `guard`, the middleware that allows and limits each call; `specsOf(contract, document)` for `charter.config.ts`'s `specs` |
| `"charter-lib/specs"` | Both specs from the contract, for `npm run spec` and for the Worker's `/api/openapi.json` and `/api/asyncapi.json` |
| `"charter-lib/asyncapi"` | `asyncapi({...})` on a procedure: a WebSocket channel in the AsyncAPI spec |
| `"charter-lib/auth"` | Bearer tokens, Cloudflare Access, an OpenID Connect issuer and tokens the API issued itself (`issued`, for an OAuth endpoint of its own), with scopes; each refusal's challenge, and `wellKnown`: what a client reads to find out how to get in ([Auth](auth.md)) |
| `"charter-lib/ratelimit"` | Limits per caller on Workers Rate Limiting bindings |
| `"charter-lib/follow"` | `follow()`: the gap-free feed behind the streams ([Streaming](streaming.md)) |
| `"charter-lib/mcp"` | `mcpHandler(router, { info })`: every procedure that answers once as an MCP tool at `/api/mcp`, on the official MCP SDK ([Agents: MCP](replace-the-example.md#agents-mcp)) |

From the tool, for `charter.config.ts` and scripts of your own:

| Import | What it is |
|---|---|
| `"charter-ts/config"` | `defineConfig` |
| `"charter-ts/cloudflare"` | `rateLimits(spec, worker, bindings.rateLimit)`: a Rate Limiting binding per limit the spec declares, for `cloudflare.config.ts` |
| `"charter-ts/scripts"` | What the steps are made of: `run`, `withServer`, `migrateLocal`, `deploy`, `sdk` |

## In this repo

The root `package.json` is the npm workspace of `charter-ts/` (the tool), `ts/lib/` (the library) and the projects in `ts/` and `go/`: `ts/examples-start` and `ts/examples-notes`, which `new` copies, and `ts/conformance`, every Fern feature with the SDK tested inside a Worker.

```sh
npm ci                               # everything; builds charter-ts
npm run check                        # charter-ts's tests, every project's check (Docker), the docs
npm run ci                           # every job GitHub runs, the clients in every language among them
npm run release -- X.Y.Z --dry-run   # the check, and the package into dist/
npm run release -- X.Y.Z             # the version, the tag ts-vX.Y.Z, the GitHub Release with the package
```

GitHub runs the same jobs (`ci.config.ts`, [The CI](../contributing.md#the-ci)). Checked on 2026-10-14 on macOS: a project from each template, made by `new` from the packed package outside the repo, passed `npm run check`.
