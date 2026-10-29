---
title: Rules for every repo
nav_order: 1
parent: What to do
---

# Rules for every repo

Written by `charter init` (the same page in every repo that uses it): don't edit it in another repo, change `docs/charter/rules.md` in [charter](https://github.com/joeblew999/charter). A repo's own rules are in its `docs/self/rules.md`, and add to these.

Binding, for developers and agents alike.

| Rule | Why |
|---|---|
| **One source for each fact.** `docs/` is where things are written; `README.md`, `AGENTS.md` and `CLAUDE.md` only point to it. What can be written from the code is generated from it | Two copies drift, and the wrong one gets read |
| **Never edit what is generated, or what charter writes.** The first lines of such a file say so, and say what to change instead; the repo's docs list the rest | A hand edit is lost, and the check fails |
| **Everything is a command of charter's, and the CI runs here.** `charter <command>`, the charter the repo's `mise.toml` pins (each is also `mise run charter:<command>`). A workflow runs one job and nothing else, as `charter ci <job>` runs it here. Run it here, and act on what it says. Never use GitHub as the edit loop, and never wait for a run there | A runner may not come for an hour; your machine is here now |
| **Every command is safe to run again,** and says what it acts on before it acts | Nobody remembers what was run once |
| **Start with the issues:** `charter issues`. One labelled `needs-triage` comes before anything else | That is where a person said something is wrong |
| **A plan is an issue,** never a page: `charter issues plan` prints the form. So is a bug, and a fault in something this repo is built on | The docs say what is true now; an issue has a state, and people can see it |
| **Workarounds name their issue:** `Upstream: <owner>/<repo>#<n> (when fixed: ...)` in the code. `charter upstream` lists them | Otherwise it is never removed |
| **Say what failed, first, unasked.** Report as working only what you ran, and say where and when | A reader acts on it |
| **Do the whole job in one pass:** what a change makes stale, the pages and the tests among it, changes in the same commit | A half-done change is found by someone else, later |
| **`charter check` before every commit** | It is what GitHub runs that needs no Cloudflare account |
| **Add files to a commit by name.** No secret, token or personal address in a file, a commit or a log | Other work is in the same tree, and a repo may be public |

## Working beside other agents

More than one agent may be working on a machine, and on a repo.

- **Your own git worktree,** and your own ports for anything you start.
- **Do not stop, reset or delete what you did not start.** A folder where someone has something running is not yours.
- **Never clear a cache other projects share** (`npm cache clean`, `mise cache clear`): pin a version instead.
- **One heavy job at a time,** and a lock where two runs would use the same deployed thing.
