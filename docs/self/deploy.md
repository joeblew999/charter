---
title: Deploy and CI
nav_order: 8
---

# Deploy and CI

How to put a project on Cloudflare Workers, test it there, and run the same from GitHub. Local checks cannot see everything Cloudflare's runtime does, so every deploy is followed by the live test.

## Deploy

In the project's folder (or with `-w <project>` from this repo's root):

```sh
npx cf auth login                # once per machine; or set CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID
fnox exec -- npm run deploy      # a Go Worker built first; the Worker with its secrets, then its migrations
fnox exec -- npm run live-test   # REMOTE, writes test notes: a few seconds after every deploy
```

- **`cloudflare.config.ts`** declares the Worker, its database `<name>-db` (binding `DB`), the hub and `APP_NAME`. A second deploy keeps the database.
- **Migrations** are `migrations/*.sql`, applied in name order, each once: `npm run deploy` applies them on Cloudflare, `npm run dev` locally. Never edit one that was applied.
- **The URL is `url` in `charter.config.ts`** (`API_URL` overrides it): the specs name it and the live test calls it. After changing it, `npm run spec`.
- **A Go build fails over 3,000,000 bytes gzipped,** the Workers Free limit.

## Secrets

`secrets` in `charter.config.ts` must be set for a deploy; `optionalSecrets` go with it when the environment has them. `npm run deploy` takes them from the environment: `fnox exec --` here, the repo's GitHub secrets in CI. They reach cf in a file only you can read, removed after. Making them, and who may call: [Auth](auth.md).

## Test

| Step | Where | What it proves |
|---|---|---|
| `npm run check` | Local | Go: gofmt and vet, Go tests, fresh specs, the tests natively and as Wasm under workerd. TypeScript: typecheck, fresh specs, the tests under workerd |
| `npm run live-test` | Cloudflare | The config's live tests on the deployed Worker |
| `fnox exec -- node test/soak.ts <url>` in `go/examples-notes` | Cloudflare, redeploys | No stream loses, repeats or reorders an item ([Streaming](streaming.md)) |

A test is a Node program that takes the server's URL and exits 1 on a failure, run from `test`, `native` or `live` in `charter.config.ts`. In Go, add a Go test in `api/` with the helpers in `api/api_test.go` (`server(t)`, `do(t, method, url, body)`).

## CI on GitHub

In this repo the workflows are written from `ci.config.ts`, and each runs the command that runs the same job on your machine ([The CI](contributing.md#the-ci)): `self-deploy.yml`, by hand (`npm run ci -- deploy go/examples-notes --remote`), runs a project's deploy, then its live tests. Every repo charter keeps does the same ([Any repo](any-repo.md#checks-on-github)).

A project `charter-ts new` makes is kept the same way: its `self-check.yml` runs `npm run check`. The deploy workflow's secrets:

```sh
fnox exec -- sh -c 'gh secret set CLOUDFLARE_API_TOKEN --body "$CLOUDFLARE_API_TOKEN"'
fnox exec -- sh -c 'gh secret set CLOUDFLARE_ACCOUNT_ID --body "$CLOUDFLARE_ACCOUNT_ID"'
```

Limits: one Worker and database per project, no staging, no custom domain. The deploy workflow has not yet run from GitHub.

## Keep the repo in shape

The repo's description, topics and homepage on GitHub are `package.json`'s `description`, `keywords` and `homepage` ([Any repo](any-repo.md#settings)).

```sh
npm run repo          # REMOTE: brings the repo in line, printing ok or changed per item
npm run repo -- --check  # REMOTE, read-only: fails on drift (the repo job runs it)
```

It keeps the labels (charter's `charter-labels.tsv`), removing GitHub's unused defaults; the description, homepage and topics, with `charter` added; and GitHub Pages for the docs folder. It writes no file: `npm run adopt` writes charter's files. Running it again changes nothing.
