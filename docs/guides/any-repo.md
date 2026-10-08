---
title: Any repo
nav_order: 7
parent: Guides
---

# Any repo

Most of charter is not about an API. A repo with no Worker, no contract and no Fern folder takes these parts, with one command and one include. The rest of this site is about a charter project.

| Aspect | What a repo gets | For |
|---|---|---|
| Repo | `charter.toml`; description, topics, homepage, GitHub Pages kept by `repo` and `repo:check` | Any repo |
| Docs site | `docs/` rendered the same everywhere; `writing.md`; `docs:setup`, `docs:lint`, `docs:check`, `docs:review`, `docs:pages` | Any repo |
| Generated pages | `_generated.toml` in `docs/`: pages a command of the repo writes, checked for freshness | Any repo |
| Tasks page | The repo's mise tasks as a page of its docs, from `charter docs-tasks` | Any repo |
| Checks, here and on GitHub | One task, `repo:ci` (`docs:check`, `repo:check`, `upstream:status`), and one workflow, `repo-check`, that runs only that task | A repo that is not a project |
| Agents and rules | `AGENTS.md` and `CLAUDE.md`, which point at `docs/` and are the same in every repo; `repo.md` and `repo/rules.md` in `docs/`, the rules every repo shares, written by charter and the same in each; `rules.md`, the repo's own | Any repo |
| Issues and plans | The issue forms, the labels, the tasks `issue` and `issues`; a plan is an issue | Any repo |
| Upstream | `Upstream:` tags and `upstream:status` | Any repo |
| Release | `mise run release -- vX.Y.Z`: the repo's own `check`, the tag, the GitHub Release with notes from the commits | Any repo |
| Cloudflare | Access, secrets, migrations, logs | A repo with a Worker ([Deploy and CI](deploy.md), [Auth](auth.md)) |
| API project | Contract, specs, SDKs, CLI, release, the GitHub workflows | A charter project ([Getting started](../getting-started.md)) |

## Adopt it

In the repo, which needs [mise](https://mise.jdx.dev), git and `gh`:

```sh
mise x github:joeblew999/charter -- charter adopt    # -description "<what the repo is>" for a new charter.toml
mise install && mise tasks
```

`charter adopt` writes what the repo lacks and keeps what it has; running it again changes nothing.

| It writes | If missing | Otherwise |
|---|---|---|
| `charter.toml` | The description from GitHub, `-description`, or the folder's name | Kept |
| `AGENTS.md`, `CLAUDE.md` | Pointers to `docs/`, with the order to read it in: charter's, the same in every repo | Written again if they differ: what is the repo's own goes in `docs/` |
| `README.md` and `rules.md` in `docs/` | The start page to fill in, and the rules every repo shares | Kept |
| `writing.md`, and with the repo on GitHub the site's config, in `docs/` | What `charter docs` writes | Written again if stale: they are charter's |
| `mise.toml` | The tool pinned at its release, and the include below at that tag | Kept: it prints the lines to add, and the tasks of yours that take the place of charter's |

It writes no workflow, no issue forms and nothing on GitHub: `mise run repo` does those.

## Tell what is charter's

Every file charter writes says so in its first lines: in a comment, or in `renovate.json` its `description`.

| The first lines say | The file is | To change it |
|---|---|---|
| ``Written by `charter <command>` `` ... `don't edit` | Charter's: `mise run repo` or `mise run docs:setup` writes it again | Change it in charter |
| ``Started by `charter <command>` `` ... `yours to edit` | Yours: written once, by `adopt` or `new` | Edit it |

```sh
charter files    # every file here with either mark; charter's with matches, differs or missing
```

`mise run repo:check` is the check: it fails when one of charter's differs.

## The include

```toml
[tools]
"github:joeblew999/charter" = "X.Y.Z"

[task_config]
includes = ["git::https://github.com/joeblew999/charter.git//tasks/repo?ref=vX.Y.Z"]
```

That folder has `release`, `repo`, `repo:check`, `repo:ci`, `issue`, `issues`, `upstream:status`, `docs:setup`, `docs:lint`, `docs:check`, `docs:review` and `docs:pages` ([Tasks](../reference/tasks.md#any-repo)), and none of a project's: no release, workflows, SDK or Cloudflare tasks. They run `charter`; a repo that runs the tool another way sets `CHARTER_TOOL` under `[env]`. A task of the same name in `mise.toml` takes the place of the included one.

## What each aspect needs

| Aspect | Needs | Then |
|---|---|---|
| Repo | `charter.toml`, the repo on GitHub, `gh` signed in | `mise run repo`, and `mise run repo:check` in CI ([what it keeps](deploy.md#keep-the-repo-in-shape)) |
| Docs site | Pages in `docs/` that follow [Writing docs](../writing.md) | `mise run docs:check` before a commit |
| Generated pages | A command of the repo that prints the page | [Generated pages](deploy.md#generated-pages) |
| Tasks page | A row in `_generated.toml`, a link from a page | [The tasks page](#the-tasks-page) |
| Checks on GitHub | `mise run repo`, which writes the workflow | [Checks on GitHub](#checks-on-github) |
| Agents and rules | Nothing | Write what you learn in `docs/`, never in `AGENTS.md` |
| Issues and plans | `mise run repo`, which writes the forms and labels | `mise run issue -- plan > body.md`; `mise run issues` |
| Upstream | A comment `Upstream: <owner>/<repo>#<n> (when fixed: ...)` at each workaround | `mise run upstream:status` |

An issue form the repo wrote itself is kept: charter's forms start with a line that says charter wrote them, and a file in `.github/ISSUE_TEMPLATE/` without it is the repo's own, which `charter issue` then prints. Delete it to get charter's.

## The tasks page

```toml
# docs/_generated.toml
[[generated]]
page = "reference/tasks.md"
run = "charter docs-tasks"
```

`mise run docs:setup` writes the page from what mise knows of the tasks, and `mise run docs:check` fails when a task changed and the page did not. It is in the site and in `llms.txt` like any page with front matter.

- **Grouped by source:** the repo's `mise.toml`, then each entry of `[task_config] includes` as listed, a git include named `owner/repo//folder`.
- **In the order written** in each task file, not mise's order by name. Hidden tasks are left out.
- **A table per source** (task, what it does), then each task with its usage, arguments and flags as `mise generate task-docs` documents them.
- **`-only <source,...>`, `-not <source,...>`:** a part of a source's name is enough. `charter docs-tasks -not joeblew999/charter` leaves out charter's; `-only tasks.toml` lists one file. `-title <text>` names the page.

## Checks on GitHub

`mise run repo` writes the workflow `repo-check.yml` into `.github/workflows/` in a repo that is not a project: on pull requests and pushes to the default branch, one job on Linux runs `mise run repo:ci`, and nothing else. That task is `docs:check`, `repo:check` and `upstream:status`, written once in `tasks/repo`: run it on your machine and you have run what GitHub runs. It reads GitHub with the run's own token and may write nothing (`contents`, `issues`, `pull-requests`, `pages`: read).

- **It does not run the repo's own `check`:** that can be slow or need secrets, and belongs in the repo's own workflow, which charter leaves alone.
- **`workflow = false`** in `charter.toml`: charter writes none.
- **A project gets none:** its `check` workflow runs `docs:check` within `check`. It does not run `repo:check` or `upstream:status`.

Limits: `charter adopt` from a checkout of charter needs `-from <checkout>` and pins no tool. The docs site's config needs the repo on GitHub.
