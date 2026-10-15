---
title: Any repo
nav_order: 10
---

# Any repo

Most of charter is not about an API. A repo with no Worker, no contract and no Fern folder takes these parts with one command: the docs site, the rules, the issue forms and labels, the repo on GitHub kept in shape, the upstream issues it works around, and one check on GitHub. You need Node 26, git and `gh` signed in.

## Adopt

In the repo:

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts adopt --description "<what the repo is>"
npm install
npm run repo        # the repo on GitHub: labels, description, homepage, topics, Pages
npm run ci -- repo  # what the check on GitHub runs: charter's files, the docs, the repo, the upstream issues
```

Then commit what it wrote, `package-lock.json` among it. `adopt` writes what the repo lacks and keeps what it has; run again, it changes nothing.

| What | What `adopt` does |
|---|---|
| `package.json` | Writes one if there is none. Otherwise adds what it lacks: charter-ts (from its release, in `devDependencies`) and the scripts. A script the repo has stays its own |
| `.gitignore` | Adds `node_modules/` |
| charter's files | Writes `docs/charter/` (what to do, and when; the rules), the site's files (`docs/_config.yml`, `docs/llms.txt`, `docs/_sass/`), the issue forms and labels, the job runner (`.github/workflows/charter-ci.yml`), `renovate.json`, `AGENTS.md`, `CLAUDE.md`; starts `ci.config.ts` (yours to edit), and writes a workflow per job from it |
| `docs/self/README.md`, `docs/self/rules.md` | Starts them where there are none; then they are the repo's. `docs/self/` holds the repo's own pages |
| The repo's own `CLAUDE.md` or `AGENTS.md` | Moves what it says to the end of `docs/self/rules.md`, and writes charter's in its place |
| `charter.toml` | Moves its description and topics into `package.json` and removes it |

It changes nothing on GitHub: `npm run repo` does that.

## What to do, and when

`docs/charter/README.md` is the repo's scripts, each with when to run it and what it does, in the order a developer meets them. `npm run adopt` writes it from `package.json`, and `npm run adopt -- --check` fails while the scripts and the page differ.

## Settings

| Setting | In `package.json` |
|---|---|
| The repo's description on GitHub | `description` |
| Its topics | `keywords`; `adopt` and `repo` add `charter`, so charter's repos can be found |
| Its homepage | `homepage`; by default its GitHub Pages site |
| The docs folder | `"charter": { "docs": "." }`; by default `docs` |
| No `renovate.json`, no workflow | `"charter": { "renovate": false, "workflow": false }` |

## Tell what is charter's

| Mark in the first lines | Whose | What happens |
|---|---|---|
| ``Written by `charter-ts <command>` `` ... `don't edit` | Charter's | `npm run adopt` writes it again |
| ``Started by `charter-ts <command>` `` ... `yours to edit` | The repo's | Written once, then left alone |
| None | The repo's | Never written over, whatever its path |

`docs/charter/`, and the `charter-*.yml` forms and workflows, are charter's whole: each version writes them as it has them and removes any other file in them (an old `charter.yml` too). `npm run adopt -- --check` fails when one of charter's differs from what this version writes.

## Checks on GitHub

Every repo gets charter's job `repo` (`.github/workflows/charter-repo.yml`): `adopt --check`, `docs`, `repo --check` and `upstream`, on pull requests and pushes to `main`, with a token that may only read. A repo's own jobs go in `ci.config.ts`, typed (a project starts with one, `check`); `npm run adopt` writes a workflow per job, `.github/workflows/self-<job>.yml`. Every workflow calls charter's job runner (`charter-ci.yml`): mise installs the tools, npm the packages, and `npm run ci -- <job>` runs the job, the command that runs it on your machine, so a pass here is a pass on GitHub. `npm run ci -- <job> --remote` starts one on GitHub from here. `"charter": { "workflow": false }` in `package.json`: no workflows.

## Move to another release

```sh
npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz
npm run adopt
```

Renovate opens that pull request by itself when a release comes out.

Checked on 2026-10-14: a throwaway repo with files of its own, adopted from the packed package, passed the repo job (`repo:ci`, as it was called then) against a repo on GitHub.
