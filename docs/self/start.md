---
title: Getting started
nav_order: 2
---

# Getting started

From nothing to an API on Cloudflare Workers, tested there, with a generated SDK. You need Node 26 and git; for an SDK, Docker; to deploy, a Cloudflare account and [fnox](https://github.com/jdx/fnox); for GitHub, `gh` signed in. In Go, also Go and [mise](https://mise.jdx.dev), which fetches TinyGo.

## 1. Make the project

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.9.1/charter-ts-0.9.1.tgz charter-ts new billing-api --from notes --subdomain <your workers.dev subdomain>
cd billing-api
npm install
```

`billing-api` is the folder, the Worker and its database. `--from notes` starts from the notes API (sign-in, streams, rate limits); without it, from one route. `--go` makes it in Go ([Huma](https://huma.rocks)) instead of TypeScript. The project is a git repo of its own, kept by charter ([Any repo](any-repo.md)).

## 2. Check it, and run it

```sh
npm run check    # every check that needs no Cloudflare account
npm run dev      # the API on charter.config.ts's port; until Ctrl-C
```

## 3. Make it yours

The contract is `api/contract.ts` (in Go, `api/contract.go`). After a change to it, `npm run spec`, then `npm run check` ([Your API](api.md)).

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
git add -A && git commit -m "billing-api"
gh repo create billing-api --private --source . --push   # its check runs npm run check
```

What to run after that, and when: `docs/charter/README.md`, in the project.

Checked on 2026-10-15 on macOS: steps 1 and 2 with each example in both languages, from the packed package (the `new` job). Steps 4 to 6 were run on this repo's projects, not on a project made this way.
