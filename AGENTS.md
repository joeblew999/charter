# For agents

Everything about this repo is in [docs/](docs/README.md), the same pages developers read. Nothing is kept here, so there is one source of truth. The same index for machines: https://joeblew999.github.io/charter/llms.txt

The start page and the rules come with this file: each line below that starts with @ is that page itself, for an agent that reads them (Claude does), so nothing has to be looked up first. Any other agent: read them first, in the order below.

@docs/README.md
@docs/repo/rules.md
@docs/rules.md
@docs/repo/charter.md

Read, in this order:

1. [docs/README.md](docs/README.md): what is what, and the index of every page. The repo has two servers, four contracts and four Fern folders, and they are easy to mix up.
2. The rules, which are binding: [docs/repo/rules.md](docs/repo/rules.md), the same in every repo charter keeps, and [docs/rules.md](docs/rules.md), this repo's own.
3. The page for the part you are changing, from the index.
4. [docs/writing.md](docs/writing.md) before you write or change a page in `docs/`.

When you learn or change something, write it in the page in `docs/` it belongs to, and run `mise run docs:lint`. Don't add README files elsewhere.
