---
title: Getting started
nav_order: 2
---

# Getting started

From nothing to an API on Cloudflare Workers, tested there, with a generated SDK. You need git and [mise](https://mise.jdx.dev), which installs charter and what it runs (Node, workerd; in Go, TinyGo); for an SDK, Docker; to deploy, a Cloudflare account and [fnox](https://github.com/jdx/fnox); for GitHub, `gh` signed in. In Go, also Go.

## 1. Make the project

```sh
mise use -g github:joeblew999/charter   # charter, the one command
charter new billing-api -from notes -subdomain <your workers.dev subdomain>
cd billing-api
mise install                            # what its mise.toml pins: charter, Node, workerd…
npm install
```

`billing-api` is the folder, the Worker and its database. `-from notes` starts from the notes API (sign-in, streams, rate limits); without it, from one route. `-go` makes it in Go ([Huma](https://huma.rocks)) instead of TypeScript. `charter.toml` says everything charter knows about it, and charter runs its steps. The project is a git repo of its own, kept by charter ([Any repo](any-repo.md)).

## 2. Check it, and run it

```sh
charter check    # every check that needs no Cloudflare account
charter dev      # the API on charter.toml's port; until Ctrl-C
```

## 3. Make it yours

The contract is `api/contract.ts` (in Go, `api/contract.go`). After a change to it, `charter spec`, then `charter check` ([Your API](api.md)).

## 4. An SDK

```sh
charter sdk      # the TypeScript SDK into sdk/out/typescript-dist (Docker)
charter sdk go   # any group of fern/generators.yml
```

## 5. Deploy, and test it there

Make the Worker's tokens first ([Auth](auth.md#bearer-tokens)), and put `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in fnox; then:

```sh
charter build
fnox exec -- charter deploy production
fnox exec -- charter live-test
```

Without `-subdomain`, put the URL the deploy printed into `charter.toml`'s `url`, then `charter spec` ([Deploy and CI](deploy.md)).

## 6. On GitHub

```sh
git add -A && git commit -m "billing-api"
gh repo create billing-api --private --source . --push   # its check job runs charter check
```

What to run after that, and when: `docs/charter/README.md`, in the project.

Checked on 2026-10-29 on macOS: steps 1 and 2 with each example in both languages, with this checkout's charter through mise and charter-lib packed as a release packs it (the `new` job). Steps 4 and 5 were run on this repo's projects, not on a project made this way.
