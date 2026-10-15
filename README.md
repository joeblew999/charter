# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on Cloudflare Workers, in TypeScript ([oRPC](https://orpc.dev)) or Go ([Huma](https://huma.rocks)): you write the contract, and the specs, SDKs, MCP tools, sign-in, tests and CI follow from it. Why, and how: [docs/](docs/self/README.md), rendered at <https://joeblew999.github.io/charter/>.

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.8.1/charter-ts-0.8.1.tgz charter-ts new my-api   # a new API (--go: in Go)
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.8.1/charter-ts-0.8.1.tgz charter-ts adopt        # the repo you have
```
