---
title: Getting started
nav_order: 2
---

# Getting started

From nothing to an API on Cloudflare Workers, tested there, with a generated SDK. You need Node 26 and git; from step 4, Docker and a Cloudflare account.

## 1. Make the project

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts new billing-api --from notes --subdomain <your workers.dev subdomain>
cd billing-api
npm install
```

`billing-api` is the folder, the Worker and its database. `--from notes` starts from the notes API (sign-in, streams, rate limits); without it, from one route. The project is a git repo of its own, with charter's docs, rules and checks ([Any repo](any-repo.md)).

`--go` makes it in Go ([Huma](https://huma.rocks), built with TinyGo) instead of TypeScript; it needs Go and [mise](https://mise.jdx.dev), which fetches TinyGo. Every step below is the same in both.

## 2. Check it, and run it

```sh
npm run check    # every check that needs no Cloudflare account
npm run dev      # the API on charter.config.ts's port; until Ctrl-C
```

## 3. Make it yours

The contract is `api/contract.ts`. After a change to it, `npm run spec`, then `npm run check` ([Your API](api.md)).

## 4. An SDK

```sh
npm run sdk          # the TypeScript SDK into sdk/out/typescript-dist (Docker)
npm run sdk -- go    # any group of fern/generators.yml
```

## 5. Deploy, and test it there

Make the Worker's tokens first ([Auth](auth.md#bearer-tokens)), then:

```sh
npx cf auth login
fnox exec -- npm run deploy
fnox exec -- npm run live-test
```

Without `--subdomain`, put the URL the deploy printed into `charter.config.ts`'s `url`, then `npm run spec` ([Deploy and CI](deploy.md)).

## 6. On GitHub

```sh
gh repo create billing-api --private --source . --push   # its check runs npm run check
```

## The scripts

The same in every project, Go or TypeScript:

| Script | What it does |
|---|---|
| `npm run dev` | The API under workerd, as on Cloudflare, its local database migrated; in Go, `-- --native` runs it with `go run .` |
| `npm run check` | Every check that needs no Cloudflare account, and how long each took: the types, the specs against the contract, Fern's check, the unit tests, then the API on your machine with the standard test and the project's tests against it. `-- --fresh` does again what charter's cache skipped |
| `npm run spec` | Both specs from the contract into `fern/` |
| `npm run sdk` | An SDK with Fern (Docker); `-- --prove`: every client, proven in its language; `-- --publish`, in Go: the Go SDK into `sdk/go` |
| `fnox exec -- npm run deploy` | REMOTE: the Worker with its secrets, then its migrations |
| `fnox exec -- npm run live-test` | REMOTE: the standard test and the project's tests against the deployed Worker |

Checked on 2026-10-15 on macOS: steps 1 and 2 with each template, in both languages, from the packed package (the `new` job; a Go project took this repo's Go library, as the release that tags it is not out yet). Steps 4 to 6 were run on this repo's projects, not on a project made this way.
