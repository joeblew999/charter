# charter-conformance-ts

The conformance API in TypeScript on Cloudflare Workers: every Fern feature charter uses, in one small API, from its contract (`api/contract.ts`), its SDKs tested against it, inside the Worker too. It proves charter's generated SDKs work: [SDKs](https://joeblew999.github.io/charter/self/sdks.html). `npm run check` is every check that needs no Cloudflare account; the commands are the scripts in `package.json`, and `charter.config.ts` says what they need.
