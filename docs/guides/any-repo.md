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
| Agents and rules | `AGENTS.md`, `CLAUDE.md`, `rules.md` in `docs/` | Any repo |
| Issues and plans | The issue forms, the labels, `charter issue`, the task `issues`; a plan is an issue | Any repo |
| Upstream | `Upstream:` tags and `upstream:status` | Any repo |
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
| `AGENTS.md`, `CLAUDE.md` | Pointers to `docs/` | Kept |
| `README.md` and `rules.md` in `docs/` | The start page to fill in, and the rules every repo shares | Kept |
| `writing.md`, and with the repo on GitHub the site's config, in `docs/` | What `charter docs` writes | Written again if stale: they are charter's |
| `mise.toml` | The tool pinned at its release, and the include below at that tag | Kept: it prints the lines to add, and the tasks of yours that take the place of charter's |

It writes no workflows, no issue forms and nothing on GitHub: `mise run repo` does the last two.

## The include

```toml
[tools]
"github:joeblew999/charter" = "X.Y.Z"

[task_config]
includes = ["git::https://github.com/joeblew999/charter.git//tasks/repo?ref=vX.Y.Z"]
```

That folder has `repo`, `repo:check`, `issues`, `upstream:status`, `docs:setup`, `docs:lint`, `docs:check`, `docs:review` and `docs:pages` ([Tasks](../reference/tasks.md#any-repo)), and none of a project's: no release, workflows, SDK or Cloudflare tasks. They run `charter`; a repo that runs the tool another way sets `CHARTER_TOOL` under `[env]`. A task of the same name in `mise.toml` takes the place of the included one.

## What each aspect needs

| Aspect | Needs | Then |
|---|---|---|
| Repo | `charter.toml`, the repo on GitHub, `gh` signed in | `mise run repo`, and `mise run repo:check` in CI ([what it keeps](deploy.md#keep-the-repo-in-shape)) |
| Docs site | Pages in `docs/` that follow [Writing docs](../writing.md) | `mise run docs:check` before a commit |
| Generated pages | A command of the repo that prints the page | [Generated pages](deploy.md#generated-pages) |
| Agents and rules | Nothing | Write what you learn in `docs/`, never in `AGENTS.md` |
| Issues and plans | `mise run repo`, which writes the forms and labels | `charter issue plan > body.md`; `mise run issues` |
| Upstream | A comment `Upstream: <owner>/<repo>#<n> (when fixed: ...)` at each workaround | `mise run upstream:status` |

`mise run repo` in a repo that is not a project writes no workflows. An issue form the repo wrote itself is kept: charter's forms start with a line that says charter wrote them, and a file in `.github/ISSUE_TEMPLATE/` without it is the repo's own, which `charter issue` then prints. Delete it to get charter's.

Limits: `charter adopt` from a checkout of charter needs `-from <checkout>` and pins no tool. The docs site's config needs the repo on GitHub.
