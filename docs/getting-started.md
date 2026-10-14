---
title: Getting started
nav_order: 2
---

# Getting started: from `charter-ts new` to a deployed API with an SDK

One tutorial. You end with a TypeScript API ([oRPC](https://orpc.dev)) on Cloudflare Workers, tested there, with a generated SDK. You need Node 26 and git, and from step 4 Docker and a Cloudflare account. In Go: [Your API](guides/replace-the-example.md).

## 1. Make the project

```sh
npx --package https://github.com/joeblew999/charter/releases/latest/download/charter-ts.tgz charter-ts new billing-api --from notes --subdomain you
cd billing-api && git init
npm install
```

`--from notes` starts from the notes API (auth, streams, rate limits); without it, one route. `billing-api` is the folder, the Worker and its database; `--subdomain` is your workers.dev subdomain.

## 2. Check it, here

```sh
npm run check    # typecheck, the specs match the contract, the tests under workerd: no Cloudflare account
npm run dev      # the API on charter.config.ts's port; the first time, npm run migrate:local in another shell
```

## 3. Make it yours

The contract is `src/contract.ts`. After a change: `npm run spec`, then `npm run check`. More: [Your API](guides/replace-the-example.md), [TypeScript](guides/typescript.md).

## 4. An SDK

```sh
npm run sdk          # the TypeScript SDK into sdk/out/typescript-dist (Docker)
npm run sdk -- go    # any group of fern/generators.yml
```

## 5. Deploy, and test it there

Make the Worker's tokens first ([Auth](guides/auth.md#bearer-tokens)), then:

```sh
npx cf auth login
fnox exec -- npm run deploy
fnox exec -- npm run live-test
```

If you gave no `--subdomain`, put the URL the deploy printed into `charter.config.ts`'s `url`, then `npm run spec`. More: [Deploy and CI](guides/deploy.md).

## 6. On GitHub

```sh
gh repo create billing-api --private --source . --push   # its check workflow runs npm run check
```

Checked on 2026-10-14 on macOS: steps 1 and 2 from the published package, with each template. Steps 4 to 6 were run on this repo's projects, not on a project made this way.

Next: a stream of your own ([Streaming](guides/streaming.md)), OAuth or webhooks ([SDKs](guides/sdks.md#fern-features)), who may call ([Auth](guides/auth.md)).
