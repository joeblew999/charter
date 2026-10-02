---
title: A docs site for your repo
nav_order: 9
parent: Guides
---

# A docs site for your repo

This page gets the Markdown in your repository's `docs/` folder published as a site on GitHub Pages, with a sidebar, search and an `llms.txt` for agents, and gets it checked. Read it when the project is on GitHub and has something to document. It works for any repository with a `docs/` folder, not only one made by `charter new`.

What was run for this page, on 2026-10-01: `mise run docs:lint` and `mise run charter:check` in a project made by `charter new`, and the setup command where a GitHub repository is needed, in charter itself, whose site is made this way. `mise run docs:pages` and `mise run docs:review` were not run.

## Set it up

The repository must be on GitHub, with `gh` logged in: the setup asks GitHub for the repository's name, description and default branch.

```sh
mise run docs:setup     # writes the site's files into docs/
git add docs && git commit -m "Docs site" && git push
mise run docs:pages     # REMOTE, once per repo: turns GitHub Pages on for docs/ on main
```

`docs:setup` ends by printing the site's address:

```
docs site: https://<owner>.github.io/<repo>/ (first time: mise run docs:pages)
```

Without a GitHub repository it stops with `docs needs a GitHub repo here (gh repo view failed)`.

There is no build step and no workflow. GitHub Pages renders the folder itself, with its built-in Jekyll and the Just the Docs theme. A push to `main` updates the site.

In a repository that has no such tasks, the same three steps are:

```sh
go run github.com/joeblew999/charter/cmd/charter@latest docs -into .
git add docs && git commit -m "Docs site" && git push
gh api -X POST 'repos/{owner}/{repo}/pages' -f 'source[branch]=main' -f 'source[path]=/docs'
```

`gh` fills in `{owner}` and `{repo}` itself: type them as they are.

## What gets generated

| File | What it is |
|---|---|
| `docs/_config.yml` | The site's settings: its title and description (from GitHub), the theme, search, the "Edit this page" links, diagrams from `mermaid` code blocks |
| `docs/_sass/custom/custom.scss` | The one style change: a larger font on phones |
| `docs/writing.md` | The rules a page is held to. A page of the site like any other |
| `docs/llms.txt` | A template the site fills in: every page in sidebar order, each linked as raw Markdown. Served at the site's address followed by `llms.txt` |

- **Do not edit these four.** They are the same in every repository. Run `mise run docs:setup` again to bring them up to date, for example after `mise up` moved the tool to a newer release.
- **`mise run charter:check` fails when one differs** from what the tool would write. It asks GitHub too, so it needs `gh` logged in once `docs/_config.yml` exists.
- **Your pages are everything else** in `docs/`. A new project starts with `docs/README.md` (the start page) and `docs/rules.md`.

## What each page needs

A page starts with front matter. The sidebar is built from it:

```
---
title: Deploying
nav_order: 3
---
```

| Field | What it does | Needed |
|---|---|---|
| `title` | The page's name in the sidebar. Keep it short | always |
| `nav_order` | Its position among its siblings | always |
| `parent` | The `title` of the page it sits under, written exactly as there | on a page in a section |
| `has_children: true` | Marks a page that others name as their `parent` | on that parent page |
| `permalink: /` | Makes the page the site's home page | on `docs/README.md` only. Without it the site has no home page |

A section is a parent page with a table of its pages, and each of those pages names it:

```
---
title: Guides
nav_order: 3
has_children: true
---
```

```
---
title: Deploying
nav_order: 1
parent: Guides
---
```

A page without `title` and `nav_order` is still rendered, but it is in neither the sidebar nor `llms.txt`.

## Check the pages: `docs:lint`

```sh
mise run docs:lint
```

```
docs: 3 pages, nothing a program can fault
```

Elsewhere: `go run github.com/joeblew999/charter/cmd/charter@latest docs-lint -into .`

It reads every `.md` file under `docs/`, except in folders whose name starts with an underscore, and reports one line per problem:

| It checks | It reports when |
|---|---|
| Front matter | A page has no `title` or no `nav_order`; the start page has no `permalink: /` |
| Template code | A page contains two opening curly braces together, or a curly brace followed by a percent sign |
| Reachability | No other page links to a page |
| Links | A relative link's file does not exist, or its anchor matches no heading in that file |
| Versions | A page links to one tagged release (a `releases/tag/` or `releases/download/v...` link), or names a version of an charter module after an `@` |
| Tasks | A `mise run` command names a task that is not in `mise.toml` |
| Paths | A path in a code span starts with one of the repository's top-level folders and does not exist. A path that git ignores passes |

- **A link inside code is an example** and is not checked.
- **Tasks, paths and versions are not checked** in `docs/plans/`, `docs/findings.md` and `docs/writing.md`: those record what is planned or what was.
- **It cannot tell whether a page is true.** That is the next task.

## Have the pages reviewed: `docs:review`

```sh
mise run docs:review     # needs the claude command (Claude Code) on your PATH
git diff                 # read what it changed before you commit
```

It hands Claude one prompt: bring `docs/` up to date and into line with `docs/writing.md`. The prompt includes what `docs:lint` found. Claude then checks each page's statements against the repository (the tasks, the paths, the code a page names), corrects what is wrong, and prints a report page by page.

- **It edits files.** It may read and write files and run `mise run docs:lint`, `mise run charter:check`, `mise tasks` and read-only git commands. Nothing else.
- **It is told not to change code, tasks or specs,** only documentation.
- **Without `claude` it stops** with `docs-review needs the claude command (Claude Code) on PATH`.
- **To see the prompt without running anything:** `go run github.com/joeblew999/charter/cmd/charter@latest docs-review -print`

## The rules that keep the site working

The full set is in `docs/writing.md`. These are the ones that break the site or the check when ignored:

- **Links between pages are relative paths to the `.md` file:** `[Deploying](deploying.md)`, or `[Deploying](guides/deploying.md)` from a page one folder up. They work on GitHub and on the site. An anchor must match a heading: lower case, spaces as hyphens, punctuation dropped.
- **No hard-coded release version.** Link `releases/latest`, write `@latest`, or use the placeholder `vX.Y.Z`. A version written into a page is wrong after the next release.
- **The curly-brace rule.** The site's renderer reads two opening curly braces together, and a curly brace followed by a percent sign, as template code. The page then renders wrong or not at all. This also applies inside code blocks. So a GitHub workflow expression, or a mise template, cannot be quoted in a page: describe it in words.
- **A new page gets a link** from its section's table, or from the start page. The lint reports a page nobody links to.
- **Every command in a page is a real task.** The lint checks each `mise run` against `mise.toml`, so a renamed task shows up in the pages that still name it.

## Limits

- **GitHub Pages on a private repository** needs a paid GitHub plan.
- **The site is built from `main`** and the folder `docs/`. `mise run docs:pages` sets exactly that.
- **No preview.** There is no task to render the site on your machine. GitHub's own view of a `.md` file is close, without the sidebar.
- **One long file for agents (`llms-full.txt`) is not generated.** `llms.txt` links the pages one by one.
