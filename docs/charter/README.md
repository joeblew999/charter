---
title: How this repo is kept
nav_order: 80
has_children: true
permalink: /charter/
---

# How this repo is kept

Written by `charter adopt` (the same folder in every repo that uses it): don't edit these pages here, change `cmd/charter/kit/docs/charter/` in [charter](https://github.com/joeblew999/charter).

## The flow

1. **Adopt charter, once.** In the repo, with X.Y.Z the [latest release](https://github.com/joeblew999/charter/releases/latest):

   ```sh
   mise x github:joeblew999/charter@X.Y.Z -- charter adopt
   ```

   Name the release: without it, mise takes the newest one it has cached, which can be an old one.

   It copies charter's kit into the repo: the files listed at the end of this page. It also starts `charter.toml`, `docs/README.md`, `docs/rules.md` and the `mise.toml` lines, if the repo lacks them. Those are yours from then on.

2. **Install, and set the repo on GitHub.**

   ```sh
   mise install
   mise run repo
   ```

   `repo` sets the labels, description, topics and Pages from `charter.toml`.

3. **Check before every commit.**

   ```sh
   mise run docs:check
   mise run check    # where the repo has it
   ```

   GitHub runs the same checks on every push.

4. **Move to a new release of charter.**

   ```sh
   mise x github:joeblew999/charter@X.Y.Z -- charter upgrade
   mise run adopt
   ```

   `upgrade` moves the pin, and `adopt` writes the kit again as that release has it.

Every repo that uses charter is kept the same way, so a developer or an agent who knows one knows them all. This folder is charter's: what is the same in every repo is here, and what is this repo's own is on its other pages.

| Page | What it is |
|---|---|
| [Rules for every repo](rules.md) | The working rules, for developers and agents alike. This repo's own rules, if it has more, are in its [rules.md](../rules.md) |
| [Using charter](using.md) | The tool this repo is kept with: how to run it, how to move to another release, and every command |
| [Writing docs](writing.md) | The rules a page in `docs/` is held to |
| [Tasks](tasks.md) | This repo's tasks, written from mise |

## What is charter's

Charter writes only these. This folder, the `charter-*.yml` forms and `charter.yml` are charter's whole: each release writes them as it has them, and removes anything else in them. The other files it writes only where there is none or the one there says charter wrote it: a file without that mark is the repo's own, and charter leaves it alone.

| Where | What |
|---|---|
| `docs/charter/` | These pages |
| `docs/_config.yml`, `docs/llms.txt`, `docs/_sass/` | The docs site's look and its index for agents: GitHub Pages reads them at the top of `docs/` |
| `.github/ISSUE_TEMPLATE/charter-*.yml`, `config.yml`, `.github/charter-labels.tsv` | The issue forms and the labels |
| `.github/workflows/charter.yml` | The checks on GitHub, in a repo with no API |
| `.github/workflows/charter-check.yml`, `charter-deploy.yml`, `charter-sdk-check.yml`, `charter-release.yml`, `charter-release-cli.yml` | In a charter project, its workflows instead (the last with a CLI) |
| `AGENTS.md`, `CLAUDE.md`, `renovate.json` | What agents read first, and the update bot's settings |

`charter adopt` writes them all, copied as they are from the kit; `charter files` lists them, and whether each is what this release writes. What each part needs: [Any repo](https://joeblew999.github.io/charter/guides/any-repo.html).
