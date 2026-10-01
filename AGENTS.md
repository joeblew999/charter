# For agents

Everything about this repo is in [docs/](docs/README.md), the same pages developers read. Nothing is kept here, so there is one source of truth. The same index for machines: https://joeblew999.github.io/orpc-api/llms.txt

Read, in this order:

1. [docs/README.md](docs/README.md): what is what, and the index of every page. The repo has two servers, four contracts and four Fern folders, and they are easy to mix up.
2. [docs/rules.md](docs/rules.md): the working rules. They are binding.
3. The page for the part you are changing, from the index.
4. [docs/writing.md](docs/writing.md) before you write or change a page in `docs/`.

When you learn or change something, write it in the page in `docs/` it belongs to, and run `mise run docs:lint`. Don't add README files elsewhere.
