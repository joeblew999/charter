---
title: Any repo
nav_order: 7
parent: Guides
---

# Any repo

Most of charter is not about an API. A repo with no Worker, no contract and no Fern folder takes these parts with one command: the docs site, the rules, the issue forms and labels, the repo on GitHub kept in shape, the upstream issues it works around, and one check on GitHub. You need Node 26, git and `gh` signed in.

## Adopt

In the repo:

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts adopt --description "<what the repo is>"
npm install
npm run repo        # the repo on GitHub: labels, description, homepage, topics, Pages
npm run repo:ci     # what the check on GitHub runs: the kit, the docs, the repo, the upstream issues
```

Then commit what it wrote, `package-lock.json` among it. `adopt` writes what the repo lacks and keeps what it has; run again, it changes nothing.

| What | What `adopt` does |
|---|---|
| `package.json` | Writes one if there is none. Otherwise adds what it lacks: charter-ts (from its release, in `devDependencies`) and the scripts. A script the repo has stays its own |
| `.gitignore` | Adds `node_modules/` |
| The kit | Writes `docs/charter/`, the site's files (`docs/_config.yml`, `docs/llms.txt`, `docs/_sass/`), the issue forms and labels, the job runner (`.github/workflows/charter-ci.yml`), `renovate.json`, `AGENTS.md`, `CLAUDE.md`; starts `ci.config.ts` (yours to edit), and writes a workflow per job from it |
| `docs/README.md`, `docs/rules.md` | Starts them where there are none; then they are the repo's |
| The repo's own `CLAUDE.md` or `AGENTS.md` | Moves what it says to the end of `docs/rules.md`, and writes the kit's in its place |
| `charter.toml` | Moves its description and topics into `package.json` and removes it |

It changes nothing on GitHub: `npm run repo` does that.

## The steps

Every step is an npm script that runs `charter-ts`, listed in the order you run them in the repo's [Tasks](../charter/tasks.md) page.

## The tasks page

`docs/charter/tasks.md` is the repo's npm scripts, in the order `package.json` has them, written by `npm run adopt`. A script that runs one `charter-ts` command shows what that command does. When the scripts change, `npm run docs:check` fails until `npm run adopt` writes the page again.

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

`docs/charter/`, and the `charter-*.yml` forms and workflows, are charter's whole: each version writes them as it has them and removes any other file in them (an old `charter.yml` too). `npm run docs:check` fails when one of charter's differs from what this version writes.

## Checks on GitHub

The repo's CI is `ci.config.ts`, typed: its jobs, and when GitHub runs them. Adopt starts it with one job, `repo`, which runs `npm run repo:ci` (`docs:check`, `repo --check`, `upstream:status`) on pull requests and pushes to `main`, with a token that may only read. It is yours: add a job beside it, then `npx charter-ts ci:write`. Each job is a workflow (`.github/workflows/self-<job>.yml`) that calls the job runner the kit writes (`charter-ci.yml`): mise installs the tools, npm the packages, and `npx charter-ts ci <job>` runs the job, the command that runs it on your machine, so a pass here is a pass on GitHub. `npx charter-ts ci:remote <job>` starts one on GitHub from here. `"charter": { "workflow": false }` in `package.json`: no `ci.config.ts`, and no workflows.

## Move to another release

```sh
npm i -D https://github.com/joeblew999/charter/releases/download/ts-vX.Y.Z/charter-ts-X.Y.Z.tgz
npm run adopt
```

Renovate opens that pull request by itself when a release comes out.

Checked on 2026-10-14: a throwaway repo with files of its own, adopted from the packed package, passed `npm run repo:ci` against a repo on GitHub.
