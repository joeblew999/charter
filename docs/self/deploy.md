---
title: Deploy and CI
nav_order: 9
---

# Deploy and CI

How to put a project on Cloudflare Workers, test it there, and run the same from GitHub. Local checks cannot see everything Cloudflare's runtime does, so every deploy is followed by the live test.

## Deploy

In the project's folder (in this repo, `go tool charter` in place of `charter`), with `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` in fnox:

```sh
charter build                            # the Worker into build/: Go with TinyGo, TypeScript bundled
fnox exec -- charter deploy production   # the Worker with its secrets, then its migrations
fnox exec -- charter live-test           # REMOTE: the standard test, then the project's (the notes ones write notes)
```

- **`charter.toml`** declares the Worker, its database (`[[d1]]`, `database` its name on Cloudflare) and, with a stream, the hub (`[[durable_objects]]`). A second deploy keeps the database. The same code may deploy as more Workers (`[production] also`: `examples/conformance-ts` does), each with a database and counters of its own.
- **Migrations** are `migrations/*.sql`, applied in name order, each once, recorded in the database's `d1_migrations`: `charter deploy` applies them on Cloudflare, `charter dev` locally. Never edit one that was applied.
- **The URL is `url` in `charter.toml`:** the specs name it and the live test calls it. After changing it, `charter spec`.
- **The logs:** `fnox exec -- charter tail` streams the deployed Worker's.
- **The size:** `charter check` fails when the Worker, gzipped as a deploy uploads it, is over 3 MB, what Workers Free allows.

## Secrets

`[secrets]` `required` in `charter.toml` must be set for a deploy; `optional` go with it when the environment has them. `charter deploy production` takes them from the environment: `fnox exec --` here, the repo's GitHub secrets in CI. They reach Cloudflare without being printed or kept in a file of the repo; a secret a deploy doesn't set stays as it was. Making them, and who may call: [Auth](auth.md).

## CI on GitHub

A repo's own jobs are its `charter.toml`'s `[ci.jobs]`; a project starts with one, `check`, which runs `charter check` on every push and pull request ([Any repo](any-repo.md#checks-on-github)). A job that deploys needs the Cloudflare credentials and the Worker's secrets as the repo's GitHub secrets, and names them in its `secrets`:

```sh
fnox exec -- sh -c 'gh secret set CLOUDFLARE_API_TOKEN --body "$CLOUDFLARE_API_TOKEN"'
fnox exec -- sh -c 'gh secret set CLOUDFLARE_ACCOUNT_ID --body "$CLOUDFLARE_ACCOUNT_ID"'
fnox exec -- sh -c 'gh secret set WRITE_TOKEN --body "$WRITE_TOKEN" && gh secret set READ_TOKEN --body "$READ_TOKEN"'
```

This repo has one, `deploy`, run by hand: `go tool charter ci deploy examples/notes-go -remote`. It has not run on GitHub; its three steps were run here by hand, on all eight projects on 2026-10-29.

This repo's projects deploy to production only. `charter.toml` also takes `[staging]` and `[preview]`, each with its own Worker, and a custom domain (every key: `go doc github.com/joeblew999/charter/project`).
