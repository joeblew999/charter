<!-- Written by `charter adopt` from the kit (github.com/joeblew999/charter, cmd/charter/kit/AGENTS.md): don't edit. The same in every repo charter keeps; what is this repo's own is in docs/. -->

# For agents

Everything about this repo is in [docs/](docs/README.md), the same pages developers read. Nothing is kept here, so there is one source of truth.

The start page, the rules, how to use charter and the tasks come with this file: each line below that starts with @ is that page itself, for an agent that reads them (Claude does), so nothing has to be looked up first. Any other agent: read them before anything else, in this order: [docs/README.md](docs/README.md), what this repo is and the index of every page; [docs/charter/rules.md](docs/charter/rules.md), the rules every repo shares; [docs/rules.md](docs/rules.md), this repo's own; [docs/charter/using.md](docs/charter/using.md), the tool it is kept with. The rules are binding.

@docs/README.md
@docs/charter/rules.md
@docs/rules.md
@docs/charter/using.md
@docs/charter/tasks.md

Then:

1. The open issues, `mise run issues`: what is reported and what is planned. One labelled `needs-triage` comes first.
2. The page for the part you are changing, from the start page's index.
3. [docs/charter/writing.md](docs/charter/writing.md) before you write or change a page in `docs/`.

`mise tasks` lists what there is to run; `mise run check`, where the repo has it, is what must pass.

When you learn or change something, write it in the page in `docs/` it belongs to, and run `mise run docs:check`. Don't add README files elsewhere. [docs/charter/](docs/charter/README.md) is charter's, written by it: change what is yours, not that.
