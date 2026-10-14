# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on Cloudflare Workers, in TypeScript ([oRPC](https://orpc.dev)) or Go ([Huma](https://huma.rocks)), and one way to keep any repo: its docs site, its rules, its issue forms, its settings on GitHub. Everything is in [docs/](docs/README.md), rendered at <https://joeblew999.github.io/charter/>.

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.6.0/charter-ts-0.6.0.tgz charter-ts new my-api   # a new API
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.6.0/charter-ts-0.6.0.tgz charter-ts adopt        # the repo you have
```
