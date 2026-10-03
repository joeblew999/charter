---
title: Deploy and CI
nav_order: 4
parent: Guides
---

# Deploy and CI

How to put a project on Cloudflare Workers, test it there, and run the same from GitHub. Local checks cannot see everything Cloudflare's runtime does, so every deploy is followed by the live test.

## Deploy

```sh
npx cf auth login     # once per machine; or set CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID
mise run deploy       # builds the Wasm, deploys the Worker with its secrets, its D1 database and the hub, applies migrations
mise run live-test    # REMOTE, writes test notes: a few seconds after every deploy
```

- **`cloudflare.config.ts`** declares the Worker, the database `<name>-db` (binding `DB`), the hub (`HUB`) and `APP_NAME`. A second deploy keeps the database.
- **Migrations** are `migrations/*.sql`, applied in name order, each once: `mise run migrate` on Cloudflare, `mise run migrate:local` for `mise run dev`. Never edit one that was applied.
- **The URL is `API_URL`** in `mise.toml`: the specs name it and the remote tests call it. After changing it, `mise run spec`. `mise.local.toml` overrides it on one machine; do not run `mise run spec` then.
- **The build fails over 3,000,000 bytes gzipped,** the Workers Free limit.

## Secrets

The Worker's secrets are named in `mise.toml`: `WORKER_SECRETS` must be set, `WORKER_OPTIONAL_SECRETS` are set when the environment has them. Every `mise run deploy` sets them, from fnox or, in CI, from the repo's GitHub secrets. Making them, and who may call: [Auth](auth.md).

## Test

| Task | Where | What it proves |
|---|---|---|
| `mise run check` | Local | Lint, Go tests, fresh specs, then the live and MCP tests natively (`test:native`) and as Wasm under workerd (`test:workerd`) |
| `mise run live-test` | Cloudflare | The same programs, and the TypeScript SDK, on the deployed Worker |
| `mise run soak` | Cloudflare, redeploys | No stream loses, repeats or reorders an item ([Streaming](streaming.md)) |

Add a Go test in `api/` with the helpers in `api/api_test.go` (`server(t)`, `do(t, method, url, body)`). Add a Node program that takes a URL and exits 1 on a failure to `test:native`, `test:workerd` and `live-test` in `mise.toml`.

## CI on GitHub

`charter new` writes the workflows; every step is a mise task, so a failing step runs the same on your machine. Never edit them: `mise run workflows` writes them again.

| Workflow | When | Runs |
|---|---|---|
| `check` | Every push to main, version tag and pull request | `setup`, `check`, on Linux, Windows and macOS |
| `sdk-check` | The same | `sdk:gen`, `sdk:check`, `sdk:publish:check` |
| `deploy` | `gh workflow run deploy.yml` | `deploy`, then `live-test` |
| `release` | A version tag (`mise run release` pushes it) | Builds it all again, adds what the Release lacks ([Release](release.md)) |

```sh
mise run cloudflare:secrets    # once: the deploy workflow's two secrets, from fnox (or: gh secret set)
```

Limits: one Worker and database, no staging, no custom domain. The deploy workflow has not yet run from GitHub.

## Keep the repo in shape

`charter.toml` at the repo's root says what the repo is; `charter new` writes one.

```toml
description = "billing-api: a contract-first API on Cloudflare Workers"
topics = ["api", "cloudflare-workers"]
# homepage = "https://..."     default: the GitHub Pages URL
# docs = "docs"                the docs folder: "docs" or "."
# projects = ["api"]           folders that are projects, in a repo that holds several
# renovate = false             no renovate.json (default: charter's Renovate preset)
```

```sh
mise run repo          # REMOTE: brings the repo in line, printing ok or changed per item
mise run repo:check    # REMOTE, read-only: fails on drift, for CI
```

It runs at the repo's root, from any folder in it, and keeps: the docs site's config, `docs/writing.md` and the [generated pages](#generated-pages); the issue forms and `labels.tsv`; the workflows, for a repo that is a project or lists `projects`; the GitHub labels, removing GitHub's unused defaults; the description, homepage and topics, with `charter` added, which is how `charter catalog` finds the repo; `renovate.json` ([Repos that use each other](repos.md#propagate-a-release)); GitHub Pages for the docs folder. Running it again changes nothing. A project in a subfolder keeps its own tasks.

### Generated pages

A page made from the code (every command and flag, the MCP tools, the API's routes, the last test run) is committed like any other, so GitHub Pages renders it with no build step:

1. Give the repo a command that prints the page as Markdown, from its `# Title` on.
2. List it in `_generated.toml` in `docs/` (the underscore keeps it off the site):

   ```toml
   [[generated]]
   page = "reference/commands.md"              # below docs/
   run = "go run ./cmd/billing docs-commands"  # at the repo's root; no shell, "double quotes" hold an argument with spaces
   ```

3. `mise run docs:setup` writes it with charter's front matter: the title from its first heading, its section page as parent (`reference/x.md`: `reference.md`), after the written pages. Commit it.
4. Link it from its section's page, and add it to the home page's "What is generated" table, written by `mise run docs:setup`.

`mise run docs:check` fails while a committed page differs from what its command prints now, as `spec:check` does for the specs. `docs:lint` checks a generated page's front matter, links and curly braces, not the tasks, paths and versions it names: those are fixed in the code.
