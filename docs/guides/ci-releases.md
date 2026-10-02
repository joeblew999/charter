---
title: CI and releases on GitHub
nav_order: 8
parent: Guides
---

# CI and releases on GitHub

This page gets your project checked on every push and pull request, and its SDKs published with a version tag. Read it once the project is a repository on GitHub.

The workflows were generated on 2026-10-01 in a project made by `charter new -name billing-api`, and read. They were not run on GitHub for this page: no tag was pushed and no release made. What a release holds was built locally (`mise run sdk:dist`, then `mise run release` as a dry run).

## Write the workflows

```sh
mise run workflows     # writes .github/workflows/
```

```
wrote .github/workflows/check.yml
wrote .github/workflows/deploy.yml
wrote .github/workflows/sdk-check.yml
wrote .github/workflows/release.yml
.github/workflows: 4 workflows, 4 written
```

Commit them. A project gets these four:

| Workflow | Runs on | What it runs | Secrets |
|---|---|---|---|
| `check` | A push to `main`, every pull request, by hand | `mise run setup`, then `mise run check`: lint, Go tests, spec drift, the Wasm build, and the live and MCP tests natively and under workerd | none |
| `sdk-check` | A push to `main`, every pull request, by hand | Two jobs, one per SDK (Go, TypeScript): `mise run sdk:gen <group>`, then `mise run sdk:check <group>`. A third, `published`: `mise run sdk:publish:check`, which fails when the committed Go SDK in `sdk/go` is stale ([Giving the Go SDK to another repo](sdks.md#giving-the-go-sdk-to-another-repo)) | none |
| `deploy` | By hand only | `cloudflare:token`, `setup`, `deploy`, `live-test` ([Deploy from GitHub](deploy.md#deploy-from-github)) | `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` |
| `release` | A version tag. As a dry run: by hand, and on pull requests that change `fern/`, `sdk/`, `mise.toml`, `go.mod` or the workflow itself | Two jobs. `sdk`: `mise run sdk:dist`, `mise run release`, then `mise run release:tags`. `cli`: `mise run sdk:dist:cli`, then `mise run release` | none: it uses the token GitHub gives the workflow |

Start one by hand with `gh workflow run check.yml`.

## Workflows only call mise tasks

Every step that does work is `mise run` and a task. The rest is checking out the code, installing the tools with mise, and keeping build output as a workflow artifact. So:

- **A failing step is one line you can run on your machine.** `check` failed? Run `mise run check`.
- **To change what CI does, change the task** in `mise.toml`. The workflow follows.
- **The tools are the same versions as yours:** GitHub installs them from `mise.toml`, as `mise install` does.
- **Each workflow runs on a pinned runner (`ubuntu-24.04`) with pinned versions of the actions.**

## Keep the workflows in step: `charter:check`

Do not edit the files in `.github/workflows/`. They are copies of templates that are compiled into the tool the tasks run (`charter`).

```sh
mise run charter:check       # part of mise run check
```

```
.github/workflows: 4 workflows match their templates
```

It fails, naming the files, when a workflow differs from its template. That happens when you edited one, or when a newer release of the tool (`mise up`) has newer templates. Either way:

```sh
mise run workflows   # writes them again
```

- **`charter:check` runs on your machine, in `mise run check`.** No generated workflow runs it on GitHub.
- **To run something of your own on GitHub,** add a workflow file of your own beside these. `charter:check` only looks at the files it wrote.

## The two secrets

Only `deploy` needs secrets: `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. Its first step fails, naming them, when one is missing.

```sh
mise run cloudflare:secrets    # REMOTE, once per repo: copies both from fnox into the repo's GitHub secrets
```

The values are never printed. Where they come from, and how to set them without fnox: [Deploy from GitHub](deploy.md#deploy-from-github). This task was not run for this page.

## Cut a release

On the commit to release, once its checks are green:

```sh
git tag vX.Y.Z && git push origin vX.Y.Z
```

The tag must be a semantic version: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release. Any other tag that starts with `v` starts the workflow and fails it.

The `release` workflow then builds everything again from that commit and attaches it to the tag's GitHub Release. It creates the Release if there is none, with notes GitHub generates from the commits. A tag with a hyphen becomes a pre-release.

The same workflow then runs `mise run release:tags`, which gives the project's Go modules their versions. A Go module in a subfolder is only found under a tag that starts with the folder, so, when the Go SDK has been published into `sdk/go`, it adds `sdk/go/vX.Y.Z` on the commit of `vX.Y.Z` (the project's own module is at the root: `vX.Y.Z` is its version). Outside a tag's workflow run the task is a dry run that prints the tags. This step has not run on GitHub in a project yet.

A release does not deploy. Deploying is `deploy`, or `mise run deploy`.

## What a release contains

| File | What is in it |
|---|---|
| `billing-api-sdk-go.tar.gz` | The Go SDK's source, generated fresh and checked with `sdk:check` |
| `billing-api-sdk-typescript.tar.gz` | The TypeScript SDK's source, the same way |
| `billing-api-specs.tar.gz` | `openapi.json` and `asyncapi.json` |
| `billing-api-cli-linux-amd64` | The CLI, built for Linux on amd64. The binary calls itself by the project's name (`billing-api`) |

The first three, built here:

```sh
mise run sdk:dist    # needs Docker: generates, checks and archives into dist/
mise run release     # not on a version tag: lists dist/ and publishes nothing
```

```
archived: dist/billing-api-sdk-go.tar.gz (53 files)
archived: dist/billing-api-sdk-typescript.tar.gz (81 files)
archived: dist/billing-api-specs.tar.gz (2 files)
dry run (not on a version tag): 3 files built, nothing published
```

`dist/` ignores itself, so nothing in it is committed. The CLI for the release is built by `mise run sdk:dist:cli`, which was not run for this page.

A pull request that changes `sdk/` or `mise.toml` runs the same build as a dry run and keeps the files as workflow artifacts, so a broken release shows before the tag.

## What a project does not have

A project made by `charter new` uses the `charter` tool but has no `cmd/charter/` folder. That decides what it gets:

- **No jobs for the tool or the Go library.** The same four workflows are written into the charter repo with those jobs added, and with a job per example.
- **No GoReleaser step.** GoReleaser builds the `charter` tool for charter's own releases. Your release has no binaries of the tool, and your `mise.toml` has no `charter:release` task.
- **No packages.** Nothing is published to npm, the Go SDK is a module in the project's own repository (`sdk/go`) and not in one of its own, and there is no CLI for macOS or Windows ([Hand the SDKs to others](sdks.md#hand-the-sdks-to-others)).
- **No deploy on push.** `deploy` runs by hand.
