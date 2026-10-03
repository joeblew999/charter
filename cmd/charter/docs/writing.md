---
title: Writing docs
nav_order: 90
---

# Writing docs

Written by `charter docs` (the same page in every repo that uses it): don't edit it here, change `cmd/charter/docs/writing.md` in [charter](https://github.com/joeblew999/charter). `mise run docs:lint` checks what a program can; `mise run docs:review` has Claude check the rest.

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

| Section | What goes there | Size |
|---|---|---|
| Home (`README.md`) | What this is, what you get, the map, what is generated, the index | One screen |
| Getting started | From nothing to a working result, every step run as written | One page |
| Guides | One job per page: steps and commands | Up to about 500 words each |
| Reference | Commands, tasks, packages: tables, one line per row | As long as the tables are |
| How to help | Setting up, reporting a bug, the rules, upstream issues, benchmarks | A few short pages |

A page belongs to one section. Nothing a user needs lives only under How to help.

## The mechanics

- **Front matter first:** `title`, `nav_order`, and `parent` under another page. The home page also has `permalink: /`.
- **Links are relative,** and an anchor must match a heading. A new page gets a row in the home page's index.
- **No two opening curly braces together, and no curly brace followed by a percent sign:** the site's renderer reads them as template code.
- **No release version in a page:** link `releases/latest`, write `@latest`, or use `vX.Y.Z`.
- **Don't edit what is generated:** `_config.yml`, `_sass/`, `llms.txt`, this page, and the pages `_generated.toml` lists (change the code their command reads). The home page's "What is generated" table lists them.

## When the code changes

The page changes in the same commit, and a removed thing is removed from every page (`mise run docs:lint` finds the mentions).
