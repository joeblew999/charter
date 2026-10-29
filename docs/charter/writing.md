---
title: Writing docs
nav_order: 2
parent: What to do
---

# Writing docs

Written by `charter init` (the same page in every repo that uses it): don't edit it in another repo, change `docs/charter/writing.md` in [charter](https://github.com/joeblew999/charter). `charter docs` checks what a program can; `charter docs -review` has Claude check the rest.

## The reader

Someone who wants to use this, or to help with it, and has a job to do. Write for that job. The docs are not a record of how the project was built: git history and the issues hold that.

## The rules

1. **Short.** Steps and commands first, one sentence of why where it matters. If a paragraph does not change what the reader does, cut it.
2. **True at this commit.** No history ("previously", "we changed"), no plans: plans are GitHub issues.
3. **Checkable.** Every command is real and pasteable. Every path exists. Every number says where and when it was measured.
4. **One place per fact.** Other pages link to it.
5. **Honest about limits,** next to the claim they limit.
6. **Plain words, one name per thing.** Tables for mappings, bullets for lists, code blocks for commands.

## The layout

| Folder | What goes there |
|---|---|
| `docs/charter/` | What to do, and when; the rules every repo shares; this page. charter's, the same in every repo: changed in charter, never in a repo that uses it |
| `docs/self/` | Everything else: the repo's own pages. `README.md` is the home page: what this is, why, and an index of every page (one screen). Then a page per job a reader has (steps and commands first, up to about 500 words), reference tables, and how to help |

## The mechanics

- **Front matter first:** `title` and `nav_order`. The home page also has `permalink: /`.
- **Links are relative,** and an anchor must match a heading. A new page gets a row in the home page's index.
- **No two opening curly braces together, and no curly brace followed by a percent sign:** the site's renderer reads them as template code.
- **A version only in a URL that installs it;** anywhere else it goes stale.
- **Don't edit what is generated:** `docs/charter/README.md` (written from charter's commands), and what the first lines of a file say is generated.

## When the code changes

The page changes in the same commit, and a removed thing is removed from every page: `charter docs` finds a removed command, script, path or link; search for any other name.
