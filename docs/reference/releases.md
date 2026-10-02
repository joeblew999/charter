---
title: Releases
nav_order: 5
parent: Reference
---

# Releases: what one contains, and how a project moves to a new one

What a release of orpc-api is, the files it has, how its versions are tagged, how your project picks one up, and what is promised between versions. Read it before you pin or update the tool the tasks run (`dev`) or the Go packages. The file list was checked against the latest GitHub Release on 2026-10-01.

The newest release: [github.com/joeblew999/orpc-api/releases/latest](https://github.com/joeblew999/orpc-api/releases/latest)

## How to get the latest release

```sh
go run github.com/joeblew999/orpc-api/dev@latest help                 # the tool, no install (needs Go)
go get github.com/joeblew999/orpc-api/api-go@latest                   # the Go packages, run in api-go/
curl -fsSL https://github.com/joeblew999/orpc-api/releases/latest/download/dev_darwin_arm64.tar.gz | tar xz dev   # the tool as a binary, into ./dev
```

As a mise tool: [The dev tool](dev.md#how-to-run-it).

## What a release contains

One GitHub Release per version, with these files. No file name carries the version.

| File | What it is |
|---|---|
| `dev_linux_amd64.tar.gz`, `dev_linux_arm64.tar.gz`, `dev_darwin_amd64.tar.gz`, `dev_darwin_arm64.tar.gz` | The tool as a binary for that system. Each archive holds the binary `dev` and a `README.md` |
| `checksums.txt` | The SHA-256 of each of those four archives |
| `api-go-sdk-go.tar.gz`, `api-go-sdk-typescript.tar.gz` | The Go and TypeScript SDK sources that Fern generates for orpc-api's own Go Worker. Generated fresh for the release, then checked |
| `api-go-specs.tar.gz` | That Worker's `openapi.json` and `asyncapi.json` |
| `api-go-cli-linux-amd64` | The Fern CLI of that Worker, for Linux on amd64 |
| `api-sdk-go.tar.gz`, `api-sdk-typescript.tar.gz`, `api-specs.tar.gz`, `api-cli-linux-amd64` | The same four for orpc-api's TypeScript Worker |

What a project uses from a release is the tool and the Go packages. The Go packages are not files of the Release: Go fetches them from the repository at the release's tag. The SDK, spec and CLI files are those of orpc-api's own example API, not of yours. Your project's release workflow (`.github/workflows/sdk-release.yml`, written by `mise run dev:workflows`) attaches the same kinds of files for your API to your repository's releases.

Not in a release:

- **No Windows build of the tool.** It manages process groups, which is Unix-only.
- **No SDK packages.** The SDKs are source archives, not packages on npm or Go modules of their own.
- **The Fern CLI only for Linux on amd64.**

## The stable links

Because the file names carry no version, one link always gives the newest file:

```text
https://github.com/joeblew999/orpc-api/releases/latest/download/<file>
```

For example `https://github.com/joeblew999/orpc-api/releases/latest/download/checksums.txt`. Use `releases/latest` and `@latest` in anything you write down: a link with a version in it is out of date after the next release.

## The tags of a release

A release is four git tags on one commit.

| Tag | What it is for |
|---|---|
| `vX.Y.Z` | The release itself. The maintainer pushes this one; the GitHub Release is made for it |
| `api-go/vX.Y.Z` | The version of the Go module `github.com/joeblew999/orpc-api/api-go`, which holds the packages |
| `dev/vX.Y.Z` | The version of the Go module `github.com/joeblew999/orpc-api/dev`, which is the tool |
| `sdk/go/vX.Y.Z` | The version of the Go module `github.com/joeblew999/orpc-api/sdk/go`, which is the Go SDK of the notes API |

The three extra tags exist because of a rule of Go: a module that sits in a subdirectory of a repository only has a version under a tag that starts with the directory. Without `api-go/vX.Y.Z`, `go get` would not find the release; without `dev/vX.Y.Z`, `go run ...dev@latest` and mise's `go:` tools would not; without `sdk/go/vX.Y.Z`, another repo could only get the SDK at a branch or a commit. The release workflow adds all three on the commit of `vX.Y.Z`. No release has carried the `sdk/go` tag yet: the releases up to 2026-10-01 were cut before that module existed.

A version is a semantic version: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release. When `dev release` creates the GitHub Release of a tag with a hyphen, it marks it as a pre-release.

Your own project has one Go module in a subdirectory, `api-go/`, and a second, `sdk/go/`, once you publish its Go SDK ([Giving the Go SDK to another repo](../guides/sdks.md#giving-the-go-sdk-to-another-repo)). Its task `release:tags` adds `api-go/vX.Y.Z` and `sdk/go/vX.Y.Z` to your releases for the same reason.

## How a project picks up a new release

A project made by `dev new` depends on a release in two places ([Configuration](config.md#pinned-versions-and-where-each-pin-lives)).

**The tool**, pinned in `mise.toml`:

```sh
mise up --bump "go:github.com/joeblew999/orpc-api/dev"   # install the newest release and write it into mise.toml
```

Where the pin is `latest`, `mise up` alone installs the newest release.

**The Go packages**, pinned in `api-go/go.mod`:

```sh
cd api-go && go get -u github.com/joeblew999/orpc-api/api-go && go mod tidy   # the newest release of the packages
```

Then prove the project against both:

```sh
mise run dev:workflows   # the workflows, as the new tool writes them
mise run check           # every local check
```

What an update does not touch: the files `dev new` copied into your project. They are yours, and no command updates them: the contract and handlers, `api-go/worker/`, the two `platform_*.go` files, `api-go/cmd/spec/`, the test programs in `test/`, and the tasks in `mise.toml`. A fix to one of those in orpc-api reaches your project only if you copy it. Compare with the [orpc-api repository](https://github.com/joeblew999/orpc-api) when a release's notes mention them.

### Getting the faster Worker in a project made before it

A project made before Go runtimes were reused keeps working after an update, at its old cost: ten times the CPU per request or more ([Benchmarks](../benchmarks.md)). Three of the copied files carry the change. After updating the tool and the Go packages as above:

1. **The build task** in `mise.toml`, if it still calls `tinygo build` itself:

   ```toml
   [tasks."api-go:build"]
   run = "dev wasm-build -dir api-go"
   ```

2. **`api-go/worker/go.mjs`:** copy it from [orpc-api](https://github.com/joeblew999/orpc-api/blob/main/api-go/worker/go.mjs), and in `api-go/worker/index.mjs` replace the import of `../build/worker.mjs` with:

   ```js
   import "../build/wasm_exec.js";
   import * as build from "../build/runtime.mjs";
   import { goWorker } from "./go.mjs";
   ```

   and, below the imports, `const go = goWorker(build);` and `await go.warm({ paths: ["/api/openapi.json"] });`, which starts two Go runtimes while the module loads. Use `go` where the file used `goWorker`.

3. **`api-go/main.go`:** `transport.Run(api.Handler(env()))` in place of `workers.Serve(transport.Serve(api.Handler(env())))`, so the Go program stays alive after a response.

Then `mise run check`, deploy, and `mise run api-go:bench`. One thing to read first: a package variable can now hold what an earlier request left there ([Go on Cloudflare Workers](../concepts/workers-go.md#a-go-runtime-is-not-a-server-process)).

## What compatibility is promised

None. The releases are `v0` versions, and under semantic versioning a `v0` release may change anything. Between two releases the exported names of the Go packages, the tool's commands and flags, the tasks and the workflow templates can all change. The release notes are generated from the commits: there is no separate list of breaking changes. Pin exact versions, update on purpose, and run `mise run check` after.

What the checks do guarantee, for the commit a release is built from:

- **The Go packages pass their tests and run under TinyGo.** The check workflow runs lint, the Go tests, the spec drift check, the TinyGo build, and the real-time and MCP tests against both the native build and the Wasm under workerd.
- **The SDKs in the release were generated from that commit's specs and passed their checks** (build, vet and tests for Go; a typecheck for TypeScript), in the release workflow itself.
- **A project scaffolded by `dev new` builds under its own name.** A test of the tool makes one, checks its names, and runs `go build`, `go vet` and three of its tests.

What they do not guarantee:

- **That a new project's full `mise run check` passes.** That is run by hand before a release, not by a workflow.
- **That anything works on Cloudflare itself.** The tests against a deployed Worker (`mise run api-go:live-test`, `mise run api-go:soak`) are run by hand. Run `mise run api-go:live-test` after each of your own deploys.
- **That a tag was only pushed on a green commit.** Nothing stops a tag on a commit whose checks failed; the release workflows build what is there.
