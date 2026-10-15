# charter

[![latest release](https://img.shields.io/github/v/release/joeblew999/charter)](https://github.com/joeblew999/charter/releases/latest)

Contract-first APIs on Cloudflare Workers, in TypeScript ([oRPC](https://orpc.dev)) or Go ([Huma](https://huma.rocks)).

An API needs much more than its handlers: input checked, a spec, SDKs for its callers, tools for AI agents, sign-in, rate limits, streams that survive a deploy, tests on the real runtime, CI and docs. Usually each is set up by hand, differently in every repo, and drifts from the others as the API changes. With charter you write one thing, the contract, and the rest follows from it and is checked against it, the same way in every project, on your machine and on GitHub.

The same APIs are kept here in TypeScript and in Go, and a program holds them to the same answers on every change, so what charter gives you is tested, not promised.

Everything else is in [docs/](docs/self/README.md), rendered at <https://joeblew999.github.io/charter/>.

```sh
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts new my-api   # a new API
npx --package https://github.com/joeblew999/charter/releases/download/ts-v0.7.0/charter-ts-0.7.0.tgz charter-ts adopt        # the repo you have
```
