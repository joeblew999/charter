---
title: Tasks
nav_order: 4
parent: How this repo is kept
---

# Tasks

Written by `charter-ts adopt` from the repo's npm scripts (package.json): don't edit, change the scripts, then `npm run adopt`.

Run one with `npm run <script>`; its arguments go after `--`. In the order you run them:

| Script | What it does |
|---|---|
| `adopt` | write charter's kit into the repo you are in, charter's own included: docs/charter/, the docs site's files, the issue forms, the workflow, AGENTS.md, CLAUDE.md, renovate.json, never a file of the repo's own; and what it lacks of package.json (charter-ts, the scripts), docs/README.md and docs/rules.md. Run again, it brings the kit to this version; --check writes nothing and fails if the kit differs |
| `repo` | keep the repo on GitHub in shape from package.json: labels, description, homepage, topics (its keywords), GitHub Pages; --check changes nothing and fails on drift |
| `issues` | REMOTE, read-only: the open issues, newest first: what is reported and what is planned (a plan is an issue) |
| `issue` | print an issue body with the headings of that issue form, and the gh command that files it |
| `check` | `tsc && npm run check --workspaces && charter-ts go:check go/lib && charter-ts docs:check` |
| `docs:check` | fail if a file of charter's kit is not what this version writes, or docs/ fails the lint |
| `repo:ci` | REMOTE, read-only: what the charter workflow runs, here: docs:check, repo --check, upstream:status. A pass here is a pass on GitHub |
| `docs:review` | have Claude (the claude command) bring docs/ up to date and into line with docs/charter/writing.md; --print only shows the prompt |
| `release` | `node release.ts` |
