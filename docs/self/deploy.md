---
title: Deploy and CI
nav_order: 9
---

# Deploy and CI

How to put a project on Cloudflare Workers, test it there, and run the same from GitHub. Local checks cannot see everything Cloudflare's runtime does, so every deploy is followed by the live test.

## Deploy

In the project's folder (or with `-w <project>` from this repo's root):

```sh
npx cf auth login                # once per machine; or set CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID
fnox exec -- npm run deploy      # a Go Worker built first; the Worker with its secrets, then its migrations
fnox exec -- npm run live-test   # REMOTE: the standard test, then the project's tests (the notes ones write notes)
```

- **`cloudflare.config.ts`** declares the Worker, its database `<name>-db` (binding `DB`) and, with a stream, the hub. A second deploy keeps the database. A project may deploy the same code as a second Worker (`deployModes`: `examples/conformance-ts` does).
- **Migrations** are `migrations/*.sql`, applied in name order, each once: `npm run deploy` applies them on Cloudflare, `npm run dev` locally. Never edit one that was applied.
- **The URL is `url` in `charter.config.ts`** (`API_URL` overrides it): the specs name it and the live test calls it. After changing it, `npm run spec`.
- **A Go repo without Node** deploys with `charter deploy` and reads the Worker's logs with `charter tail` ([Libraries](libraries.md#go-githubcomjoeblew999charter)).
- **The size:** `npm run check` fails when the Worker, gzipped as a deploy uploads it (and in Go, its Wasm), is over `bundleLimit` in `charter.config.ts`: 3 MB by default, what Workers Free allows; Workers Paid allows 10 MB.

## Secrets

`secrets` in `charter.config.ts` must be set for a deploy; `optionalSecrets` go with it when the environment has them. `npm run deploy` takes them from the environment: `fnox exec --` here, the repo's GitHub secrets in CI. They reach cf in a file only you can read, removed after. Making them, and who may call: [Auth](auth.md).

## CI on GitHub

A repo's own jobs are its `ci.config.ts`; a project starts with one, `check`, which runs `npm run check` on every push and pull request ([Any repo](any-repo.md#checks-on-github)). A job that deploys needs the Cloudflare credentials and the Worker's secrets as the repo's GitHub secrets, and names them in its `secrets`:

```sh
fnox exec -- sh -c 'gh secret set CLOUDFLARE_API_TOKEN --body "$CLOUDFLARE_API_TOKEN"'
fnox exec -- sh -c 'gh secret set CLOUDFLARE_ACCOUNT_ID --body "$CLOUDFLARE_ACCOUNT_ID"'
fnox exec -- sh -c 'gh secret set WRITE_TOKEN --body "$WRITE_TOKEN" && gh secret set READ_TOKEN --body "$READ_TOKEN"'
```

This repo has one, `deploy`, run by hand: `npm run ci -- deploy examples/notes-go --remote`. It has not yet run on GitHub; it was run here, with every project, on 2026-10-15.

Limits: one Worker and database per deploy mode, no staging, no custom domain.
