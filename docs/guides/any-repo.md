---
title: Any repo
nav_order: 7
parent: Guides
---

# Any repo

Most of charter is not about an API. A repo with no Worker, no contract and no Fern folder takes these parts, with one command and one include. The rest of this site is about a charter project.

| Aspect | What a repo gets | For |
|---|---|---|
| Repo | `charter.toml`; description, topics, homepage, labels, GitHub Pages kept on GitHub by `repo` and `repo:check` | Any repo |
| Docs site | `docs/` rendered the same everywhere; charter's own pages in `docs/charter/`; `adopt`, `adopt`, `docs:lint`, `docs:check`, `docs:review`, `docs:pages` | Any repo |
| Generated pages | `_generated.toml` in `docs/`: pages a command of the repo writes, checked for freshness | Any repo |
| Tasks page | The repo's mise tasks as `docs/charter/tasks.md`, from mise; more pages of them with `charter docs-tasks` | Any repo |
| Checks, here and on GitHub | One task, `repo:ci` (`docs:check`, `repo:check`, `upstream:status`), and one workflow, `charter.yml`, that runs only that task | A repo that is not a project |
| Agents and rules | `AGENTS.md` and `CLAUDE.md`, the same in every repo: they bring `docs/` with them (the start page and the rules load with `AGENTS.md`, for an agent that reads `@` lines) and point at the rest; `docs/charter/rules.md`, the rules every repo shares, written by charter and the same in each; `docs/rules.md`, the repo's own | Any repo |
| Issues and plans | The issue forms (`charter-*.yml`), the labels, the tasks `issue` and `issues`; a plan is an issue | Any repo |
| Upstream | `Upstream:` tags and `upstream:status` | Any repo |
| Release | `mise run release -- vX.Y.Z`: the repo's own `check`, the tag, the GitHub Release with notes from the commits | Any repo |
| Cloudflare | Access, secrets, migrations, logs | A repo with a Worker ([Deploy and CI](deploy.md), [Auth](auth.md)) |
| API project | Contract, specs, SDKs, CLI, release, the GitHub workflows | A charter project ([Getting started](../getting-started.md)) |

## Adopt it

In the repo, which needs [mise](https://mise.jdx.dev), git and `gh`:

```sh
mise x github:joeblew999/charter@X.Y.Z -- charter adopt    # X.Y.Z: the latest release, named (mise caches the list); -description "<what the repo is>" for a new charter.toml
mise install && mise tasks
```

`charter adopt` writes what the repo lacks and keeps what it has; running it again changes nothing. It never writes over a file of the repo's own, whatever its path ([Tell what is charter's](#tell-what-is-charters)). In a project, charter's own repo among them, `AGENTS.md`, `CLAUDE.md` and the start pages are the project's, and it leaves them.

| It writes | If missing | Otherwise |
|---|---|---|
| `charter.toml` | The description from GitHub, `-description`, or the folder's name | Kept |
| `AGENTS.md`, `CLAUDE.md` | The start page and the rules, loaded with `AGENTS.md` (`@` lines), and the order to read the rest in: charter's, the same in every repo | Written again if they differ: what is the repo's own goes in `docs/` |
| `README.md` and `rules.md` in `docs/` | The start page to fill in, and the rules every repo shares | Kept |
| `docs/charter/`, `llms.txt` and `_sass/` in `docs/`, and with the repo on GitHub the site's `_config.yml` | What `charter docs` writes | Written again if stale, where the file is charter's; one without charter's mark is kept |
| `mise.toml` | The tool pinned at its release, and the include below at that tag | Kept: it prints the lines to add, and the tasks of yours that take the place of charter's |

It writes the whole kit (`cmd/charter/kit/`, copied as it is): `docs/charter/` and the site's files, the issue forms, `charter-labels.tsv`, the workflows, `renovate.json`, `AGENTS.md` and `CLAUDE.md`. It changes nothing on GitHub: `mise run repo` does that.

## Upgrade

```sh
mise x github:joeblew999/charter@X.Y.Z -- charter upgrade   # the pin and the include together; AGENTS.md and CLAUDE.md as X.Y.Z writes them
mise install && mise tasks
```

`mise up --bump github:joeblew999/charter` would move only the pin, and mise takes no release younger than a day that way.

## Tell what is charter's

Charter's part of a repo is in places of its own, the same in every repo, charter's included:

| Where | What |
|---|---|
| `docs/charter/` | Its pages: [how the repo is kept, the rules every repo shares](../charter/README.md), using charter, writing docs, the tasks |
| `docs/_config.yml`, `docs/llms.txt`, `docs/_sass/` | The docs site's files, at the top of `docs/` because GitHub Pages reads them there |
| `.github/ISSUE_TEMPLATE/charter-*.yml`, `config.yml` | The issue forms |
| `charter.yml` in `.github/workflows/` | In a repo that is not a project, the checks: a call to charter's own workflow at the pinned release |
| `AGENTS.md`, `CLAUDE.md`, `renovate.json`, `.github/charter-labels.tsv` | What agents read first; the update bot's settings; the labels `mise run repo` gives the repo on GitHub |

`docs/charter/`, the `charter-*.yml` forms and `charter.yml` are charter's whole: each release writes them as it has them, and removes any other file in them, so a new release needs no steps of its own. The other files it writes only where there is none or the one there carries its mark: one without the mark is the repo's own, and `repo`, `docs` and `adopt` keep it and say so. The labels are not a file: `mise run repo` gives the repo charter's.

Every file charter writes says so in its first lines: in a comment, or in `renovate.json` its `description`.

| The first lines say | The file is | To change it |
|---|---|---|
| ``Written by `charter <command>` `` ... `don't edit` | Charter's: `charter adopt` writes it again | Change it in charter |
| ``Started by `charter <command>` `` ... `yours to edit` | Yours: written once, by `adopt` or `new` | Edit it |

```sh
charter files    # every file here with either mark; charter's with matches, differs or missing
```

`mise run docs:check` is the check (`charter adopt -check`): it fails when one of charter's differs.

## The include

```toml
[tools]
"github:joeblew999/charter" = "X.Y.Z"

[task_config]
includes = ["git::https://github.com/joeblew999/charter.git//tasks/repo?ref=vX.Y.Z"]

[settings]
experimental = true
```

mise counts a task include from a `git::` source as experimental: without `experimental = true` it skips the include, with only a warning, and the repo has none of these tasks. `adopt` and `new` write it; `adopt` names it among the lines to add to a `mise.toml` you have.

That folder has `adopt`, `release`, `repo`, `repo:check`, `repo:ci`, `issue`, `issues`, `upstream:status`, `adopt`, `docs:lint`, `docs:check`, `docs:review` and `docs:pages` ([Tasks](../reference/tasks.md#any-repo)), and none of a project's: no release, workflows, SDK or Cloudflare tasks. They run `charter`; a repo that runs the tool another way sets `CHARTER_TOOL` under `[env]`. A task of the same name in `mise.toml` takes the place of the included one.

## What each aspect needs

| Aspect | Needs | Then |
|---|---|---|
| Repo | `charter.toml`, the repo on GitHub, `gh` signed in | `mise run repo`, and `mise run repo:check` in CI ([what it keeps](deploy.md#keep-the-repo-in-shape)) |
| Docs site | Pages in `docs/` that follow [Writing docs](../charter/writing.md) | `mise run docs:check` before a commit |
| Generated pages | A command of the repo that prints the page | [Generated pages](deploy.md#generated-pages) |
| Tasks page | A row in `_generated.toml`, a link from a page | [The tasks page](#the-tasks-page) |
| Checks on GitHub | `mise run adopt`, which writes the workflow | [Checks on GitHub](#checks-on-github) |
| Agents and rules | Nothing | Write what you learn in `docs/`, never in `AGENTS.md` |
| Issues and plans | `mise run adopt`, which writes the forms and labels, then `mise run repo`, which gives the repo the labels | `mise run issue -- plan > body.md`; `mise run issues` |
| Upstream | A comment `Upstream: <owner>/<repo>#<n> (when fixed: ...)` at each workaround | `mise run upstream:status` |

Charter's forms are named `charter-*.yml` and are charter's. A form of the repo's own goes beside them, under any other name.

## The tasks page

Every repo has `docs/charter/tasks.md`: all its tasks, written by `mise run adopt` from what mise knows of them, and stale for `mise run docs:check` when a task changed and the page did not. For another page of them, of some sources only, list it in `_generated.toml`:

```toml
# docs/_generated.toml
[[generated]]
page = "reference/tasks.md"
run = "charter docs-tasks"
```

Each is in the site and in `llms.txt` like any page with front matter.

- **Grouped by source:** the repo's `mise.toml`, then each entry of `[task_config] includes` as listed, a git include named `owner/repo//folder`.
- **In the order written** in each task file, not mise's order by name. Hidden tasks are left out.
- **A table per source** (task, what it does), then each task with its usage, arguments and flags as `mise generate task-docs` documents them.
- **`-only <source,...>`, `-not <source,...>`:** a part of a source's name is enough. `charter docs-tasks -not joeblew999/charter` leaves out charter's; `-only tasks.toml` lists one file. `-title <text>` names the page.

## Checks on GitHub

`mise run adopt` writes the workflow `charter.yml` into `.github/workflows/` in a repo that is not a project: on pull requests and pushes to `main`, one job on Linux runs `mise run repo:ci`, and nothing else.

- **It does not run the repo's own `check`:** that can be slow or need secrets, and belongs in the repo's own workflow, which charter leaves alone.
- **`workflow = false`** in `charter.toml`: charter writes none.
- **A project gets none:** its `check` workflow runs `docs:check` within `check`. It does not run `repo:check` or `upstream:status`.

Limits: `charter adopt` from a checkout of charter needs `-from <checkout>` and pins no tool. The docs site's config needs the repo on GitHub.
