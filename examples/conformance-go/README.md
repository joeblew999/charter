# charter-conformance-go

The conformance API in Go on Cloudflare Workers: every Fern feature charter uses, in one small API, from its contract (`api/contract.go`), its SDKs tested against it. It proves charter's generated SDKs work: [SDKs](https://joeblew999.github.io/charter/self/sdks.html). `charter check` is every check that needs no Cloudflare account (`mise run charter:check`; in charter's repo, `go tool charter check`), and `charter.toml` says everything charter knows about it.
