---
title: Repos that use each other
nav_order: 6
parent: Guides
---

# Repos that use each other

A repo pins what it takes from another: a program from a release in `mise.toml`, a Go SDK in `go.mod`, task folders by include. Those pins are the only record. charter reads them to find who uses what, to catch a breaking change before a release, and to carry a release to every repo that pins it. Nothing is declared a second time.

## Find who uses what

```sh
charter catalog                          # Markdown: the owner's charter repos and who pins each
charter catalog -json > catalog.json     # the same for programs (spec-diff -catalog reads it)
charter catalog -page > docs/catalog.md  # a docs page
```

It reads the owner's public repos (`-owner`, default the token's login), not forks or archived ones, through the GitHub API with `GITHUB_TOKEN`, `GH_TOKEN` or gh's token. The charter repos are those with the topic `charter` (`charter repo` sets it), charter itself, and every repo that pins one of them. In this repo: `mise run catalog`.

| A repo pins | In | As |
|---|---|---|
| A program from a release | `mise.toml` `[tools]` | `"github:owner/repo"`, `"ubi:owner/repo"`, `"go:github.com/owner/repo/..."` |
| Task folders | `mise.toml` `[task_config] includes` | `git::https://github.com/owner/repo.git//<folder>?ref=vX.Y.Z` |
| A Go module | `go.mod` | a `require` not marked `// indirect` |
| An npm package | `package.json` | `github:owner/repo#ref`, or a URL of the repo's release |

What a repo publishes: its latest release (a file named for an OS is a program), `fern/` specs, Go modules named for the repo, npm packages, `tasks/<name>/` folders, and `API_URL` from its `mise.toml`. Each pin says whether it is at the latest release.

Limits: the default branch only; up to 60 files with pins per repo; `node_modules/`, `vendor/`, `testdata/`, `dist/` and `build/` are skipped.

## Catch a breaking change

```sh
mise run spec:diff                                  # the working tree against the newest version tag
mise run spec:diff -- -from vX.Y.Z -to vX.Y.Z       # two releases
mise run spec:diff -- -tag vX.Y.Z -catalog catalog.json
```

It compares `fern/openapi.json` and `fern/asyncapi.json` and marks each breaking change with `!`. A consumer sends parameters and request bodies, and receives responses and the messages of a `receive` operation: what it sends may accept more, and what it receives may hold less, but not the reverse.

| Breaking | Not breaking |
|---|---|
| An operation, channel, message, parameter, success response or field removed or moved | One added, if not required in what the consumer sends |
| A generated SDK's method renamed (Fern's group and method, or `operationId`) | |
| A parameter, field or request body made required; a received field made optional | The reverse |
| A type changed, or narrowed in what the consumer sends | A type widened there (`integer` to `integer` or `null`) |
| An enum value removed; an enum added to a sent value | An enum value added |
| Security tightened: a caller let in before is not now (a scope or scheme added, an open operation closed) | Security loosened |

It fails on a breaking change unless `-tag` (or the workflow's tag) is a major release after `-from`: a higher major number, or in v0 a higher minor one. With `-catalog`, it names the repos that pin this one's programs, modules or packages. Not compared: `oneOf`, `anyOf`, `allOf`, limits such as `maxLength`, and AsyncAPI security.

## Propagate a release

`charter repo` writes `renovate.json`, which extends charter's preset at the tool's release (`cmd/charter/renovate/charter.json`):

| What | Renovate does |
|---|---|
| `mise.toml` tools, `go.mod`, `package.json` | Opens a pull request when a pinned repo releases (managers `mise`, `gomod`, `npm`) |
| `[task_config] includes` | The same for each `?ref=` |
| charter's tool, task folders and Go library | One pull request for all three |
| A minor or patch update | Merges itself once the consumer's checks (`mise run check`) pass |
| A major update | Waits for a person |
| `.github/workflows/` | Nothing: `mise run workflows` writes them |

`renovate = false` in `charter.toml` leaves `renovate.json` to the repo. Limit: the release URL of `@charter/ts` in `package.json` is not updated (its version is written twice).

### One Renovate run for every repo

Renovate runs from one workflow in one repo of the owner, every day and when a pinned repo releases:

```yaml
# .github/workflows/renovate.yml
name: renovate
on:
  schedule:
    - cron: "0 5 * * *"
  repository_dispatch:
    types: [release]
  workflow_dispatch:
jobs:
  renovate:
    runs-on: ubuntu-24.04
    steps:
      - uses: renovatebot/github-action@vX.Y.Z   # the current release, pinned
        with:
          token: ...   # the secret RENOVATE_TOKEN, in GitHub's expression syntax
        env:
          RENOVATE_AUTODISCOVER: "true"
          RENOVATE_AUTODISCOVER_FILTER: "<owner>/*"
          RENOVATE_ONBOARDING: "false"
          RENOVATE_REQUIRE_CONFIG: "required"   # only repos with a renovate.json
```

`RENOVATE_TOKEN` is a fine-grained token for the owner's repos with write access to contents, pull requests and workflows. A producer's release wakes the run with a token that can write to the running repo:

```sh
gh api repos/<owner>/<renovate repo>/dispatches -f event_type=release -f "client_payload[repo]=<owner>/<repo>"
```
