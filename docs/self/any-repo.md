---
title: Any repo
nav_order: 10
---

# Any repo

Most of charter is not about an API. A repo with no Worker, no contract and no Fern folder takes these parts with one command: the docs site, the rules, the issue forms and labels, the repo on GitHub kept in shape, the upstream issues it works around, and charter's check on GitHub. You need Node 26, git and `gh` signed in.

## Adopt

In the repo:

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts adopt --description "<what the repo is>"
npm install
npm run repo        # the repo on GitHub: labels, description, homepage, topics, Pages
npm run ci -- repo  # what the check on GitHub runs
```

Then commit what it wrote, `package-lock.json` among it. Run again, `adopt` changes nothing.

| What | What `adopt` does |
|---|---|
| `package.json` | Writes one if there is none. Otherwise adds what it lacks: charter-ts (from its release, in `devDependencies`) and the scripts. A script the repo has stays its own, unless it runs a charter-ts command by an old name: that one goes |
| `.gitignore` | Adds `node_modules/` |
| charter's files | `docs/charter/` (what to do and when, the rules, how to write a page), the site's files (`docs/_config.yml`, `docs/llms.txt`, `docs/_sass/`), the issue forms and labels, the workflows `charter-ci.yml` and `charter-repo.yml`, `renovate.json`, `AGENTS.md`, `CLAUDE.md` |
| What it starts, then the repo's | `docs/self/README.md` and `docs/self/rules.md` (the repo's own pages go in `docs/self/`), `mise.toml` (the tools), `.gitattributes`; in a project, `ci.config.ts` |
| A `CLAUDE.md` or `AGENTS.md` of its own | Outside a project: moves what it says to the end of `docs/self/rules.md`, and writes charter's in its place |

It changes nothing on GitHub: `npm run repo` does that, from `package.json`: the labels (charter's `.github/charter-labels.tsv`, GitHub's unused defaults removed), the description, homepage and topics (its `keywords`, and `charter`, so charter's repos can be found), and GitHub Pages for the docs folder. `npm run repo -- --check` changes nothing and fails on a difference.

## What to do, and when

`docs/charter/README.md` lists the repo's scripts, each with when to run it and what it does. `npm run adopt` writes it from `package.json`.

## Settings

| Setting | In `package.json` |
|---|---|
| The repo's description on GitHub | `description` |
| Its topics | `keywords` |
| Its homepage | `homepage`; by default its GitHub Pages site |
| The docs folder | `"charter": { "docs": "." }`; by default `docs` |
| No `renovate.json`, no workflows | `"charter": { "renovate": false, "workflow": false }` |

## Tell what is charter's

| Mark in the first lines | Whose | What happens |
|---|---|---|
| ``Written by `charter-ts <command>` `` ... `don't edit` | Charter's | `npm run adopt` writes it again |
| ``Started by `charter-ts <command>` `` ... `yours to edit` | The repo's | Written once, then left alone |
| None | The repo's | Never written over, whatever its path |

`docs/charter/`, and the `charter-*` forms and workflows, are charter's whole: each version writes them as it has them and removes any other file in them. `npm run adopt -- --check` fails when one of charter's differs from what this version writes.

## Checks on GitHub

Every repo gets charter's job `repo` (`.github/workflows/charter-repo.yml`): `adopt --check`, `docs`, `repo --check` and `upstream`, on pull requests and pushes to `main`, with a token that may only read. A repo's own jobs go in `ci.config.ts`, typed, and `npm run adopt` writes a workflow for each, `.github/workflows/self-<job>.yml`. Every workflow calls charter's job runner (`charter-ci.yml`): mise installs the tools `mise.toml` pins, npm the packages, then charter-ts runs the job as `npm run ci -- <job>` runs it here. `npm run ci -- <job> --remote` starts one on GitHub from here.
