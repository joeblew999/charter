---
title: Any repo
nav_order: 11
---

# Any repo

Most of charter is not about an API. A repo with no Worker, no contract and no Fern folder takes these parts with one command: the docs site, the rules, the issue forms and labels, the repo on GitHub kept in shape, the upstream issues it works around, its jobs here and on GitHub. You need git, [mise](https://mise.jdx.dev) and `gh` signed in.

## Keep a repo the charter way

In the repo, with a `charter.toml` at its top that says what it is:

```toml
[github]
description = "<what the repo is>"
```

```sh
mise use github:joeblew999/charter   # charter, pinned in the repo's mise.toml
charter init                         # charter's files, its mise tasks, a workflow per job
charter repo                         # the repo on GitHub: labels, description, homepage, topics, Pages, merged branches deleted
charter ci repo                      # what the check on GitHub runs
```

Then commit what it wrote. Run again, `charter init` changes nothing.

| What | What `charter init` does |
|---|---|
| charter's files | `docs/charter/` (what to do and when, the rules, how to write a page), the site's files (`docs/_config.yml`, `docs/llms.txt`, `docs/_sass/`), the issue forms and labels, the workflows `charter-ci.yml` and `charter-repo.yml`, `renovate.json`, `AGENTS.md`, `CLAUDE.md` |
| What it starts, then the repo's | `docs/self/README.md` and `docs/self/rules.md` (the repo's own pages go in `docs/self/`), `mise.toml` (charter and the tools it runs), `.gitattributes` |
| Its commands as mise tasks | `.mise/tasks/charter.toml`: `mise run charter:<command>` |
| A project's own | In a folder whose `charter.toml` names a Worker: its tasks, the agent skill (`.claude/skills/charter`), in Go `.config/mise/conf.d/charter.toml` (the TinyGo and wasm-opt its Worker builds with, the versions charter's TinyGo patch is for; `charter check` refuses a `tinygo` or binaryen line in the repo's own `mise.toml`) |
| A `CLAUDE.md` or `AGENTS.md` of its own | Outside a project: moves what it says to the end of `docs/self/rules.md`, and writes charter's in its place |
| `.gitignore` | Adds `node_modules/`, in a repo with a `package.json` |

It changes nothing on GitHub: `charter repo` does that, from `charter.toml`'s `[github]`: the labels (charter's `.github/charter-labels.tsv`, GitHub's unused defaults removed), the description, homepage and topics (and `charter`, so charter's repos can be found), GitHub Pages for the docs folder, and a pull request's branch deleted once it is merged (squash merges leave branches no merge reaches, so without it they pile up). A private repo's docs are never published: its Pages stays off and its homepage is only what `[github]` says. `charter repo -check` changes nothing and fails on a difference.

## What to do, and when

`docs/charter/README.md` lists charter's commands, each with when to run it and what it does. `charter init` writes it.

## Settings

| Setting | In `charter.toml`'s `[github]` |
|---|---|
| The repo's description on GitHub | `description` |
| Its topics | `topics` |
| Its homepage | `homepage`; by default its GitHub Pages site (a public repo's) |
| The docs folder | `docs = "."`; by default `docs` |
| No `renovate.json`, no workflows | `renovate = false`, `workflows = false` |
| No mise tasks of charter's in an app (`.mise/tasks/charter.toml`): for one that calls `charter` from tasks of its own | `tasks = false`, at the top of `charter.toml`, not under `[github]` |

## Tell what is charter's

| Mark in the first lines | Whose | What happens |
|---|---|---|
| ``Written by `charter <command>` `` ... `don't edit` | Charter's | `charter init` writes it again |
| ``Started by `charter <command>` `` ... `yours to edit` | The repo's | Written once, then left alone |
| None | The repo's | Never written over, whatever its path |

`docs/charter/`, and the `charter-*` forms and workflows, are charter's whole: each version writes them as it has them and removes any other file in them. `charter init -check` fails when one of charter's differs from what this version writes; so does `charter check`.

## Checks on GitHub

Every repo gets charter's job `repo` (`.github/workflows/charter-repo.yml`): `init -check`, `docs`, `repo -check` and `upstream`, on pull requests and pushes to `main`, with a token that may only read. A repo's own jobs go in `charter.toml`, `[ci.jobs.<name>]` (every key: `go doc github.com/joeblew999/charter/project`), and `charter init` writes a workflow for each, `.github/workflows/self-<job>.yml`. Every workflow calls charter's job runner (`charter-ci.yml`): mise installs the tools `mise.toml` pins, charter among them, npm the packages if the repo has any, then charter runs the job as `charter ci <job>` runs it here. `charter ci <job> -remote` starts one on GitHub from here. `charter check` at the repo's top runs the jobs `[ci] check` lists.

## Keep the pins current

```sh
charter outdated   # every pin with a newer version upstream; exits 1 when one that isn't held is behind
charter update     # move each to its latest, go mod tidy in every module, then charter check
```

- **The pins** are the Go modules each `go.mod` in the repo requires directly or for a tool, and `mise.toml`'s tools. charter's own two, its library in `go.mod` and its command in `mise.toml`, move together, and `charter init` then writes charter's files again.
- **What stays where it is** goes in `deps.toml` at the repo's top, with why; `charter outdated` shows it, held:

  ```toml
  skip = ["examples/notes-go/sdk/go"]   # folders whose go.mod is generated
  [hold]
  "github.com/chromedp/chromedp" = "v0.17 rewrote its API"   # a Go module, or a tool as mise.toml names it
  "@moq/*" = "…"                                             # ending in *: every name that starts so
  [copies]
  workerd = ["intl/workerd/workerd.go"]   # files that write that pin's version too, as a quoted string
  "*" = ["internal/pins/repo.toml"]       # files written as mise.toml is: every tool they pin moves
  ```

- **Once a week** GitHub does it. A repo with jobs of its own gives one `on = ["weekly"]` and `run = ["charter ci update"]`, with `permissions = { contents = "write", pull-requests = "write" }`; an app whose CI is `charter.yml` has it already. `charter ci update` runs `[ci] update` (default `charter update`; a repo that pins more than charter can see names its own command), and what that changed becomes the branch `charter/update` and a pull request. An update that fails its check pushes nothing, and the run is red.
- **The pull request shows no checks of its own:** GitHub starts no workflow for a branch its own token pushed. They are the update's run, which the pull request links.
- **Not moved:** a `package.json`'s packages (`renovate.json` is for those, where the Renovate app is installed), and anything a repo pins in a file of its own.

The weekly job has not yet run on GitHub: `charter ci update` was run here, on a repository made for the test (2026-11-25).
