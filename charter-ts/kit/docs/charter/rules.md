---
title: Rules for every repo
nav_order: 1
parent: How this repo is kept
---

# Rules for every repo

Written by `charter-ts adopt` (the same page in every repo that uses it): don't edit it here, change `charter-ts/kit/docs/charter/rules.md` in [charter](https://github.com/joeblew999/charter). A repo's own rules are in its `docs/rules.md`, and add to these.

Binding, for developers and agents alike.

| Rule | Why |
|---|---|
| **One source for each fact.** `docs/` is where things are written; `README.md`, `AGENTS.md` and `CLAUDE.md` only point to it. What can be written from the code is generated from it | Two copies drift, and the wrong one gets read |
| **Never edit what is generated, or what charter writes.** The first lines of such a file say so, and say what to change instead | A hand edit is lost, and the check fails |
| **Everything is an npm script, and the CI runs here.** A workflow runs one script and nothing else: `npm run repo:ci` in a repo with no API, `npm run check` in a project. Run that script here, and act on what it says. Never use GitHub as the edit loop, and never wait for a run there | A runner may not come for an hour; your machine is here now |
| **Every script is safe to run again,** and says what it acts on before it acts | Nobody remembers what was run once |
| **Start with the issues:** `npm run issues`. One labelled `needs-triage` comes before anything else | That is where a person said something is wrong |
| **A plan is an issue,** never a page: `npm run issue -- plan` prints the form. So is a bug, and a fault in something this repo is built on | The docs say what is true now; an issue has a state, and people can see it |
| **Workarounds name their issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code. `npm run upstream:status` lists them | Otherwise it is never removed |
| **Say what failed, first, unasked.** Report as working only what you ran, and say where and when | A reader acts on it |
| **Do the whole job in one pass:** what a change makes stale, the pages and the tests among it, changes in the same commit | A half-done change is found by someone else, later |
| **`npm run check` before a commit that matters, `npm run release -- vX.Y.Z` to release** | The check is the same one a release and GitHub run |
| **Add files to a commit by name.** No secret, token or personal address in a file, a commit or a log | Other work is in the same tree, and a repo may be public |

## Working beside other agents

More than one agent may be working on a machine, and on a repo.

- **Your own git worktree,** and your own ports for anything you start.
- **Do not stop, reset or delete what you did not start.** A folder where someone has something running is not yours.
- **Never clear a cache other projects share** (`npm cache clean`, `mise cache clear`): pin a version instead.
- **One heavy job at a time,** and a lock where two runs would use the same deployed thing.
