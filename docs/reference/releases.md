---
title: Releases
nav_order: 5
parent: Reference
---

# Releases: what one contains, and how a project moves to a new one

What a release of charter is, the files it has, how its versions are tagged, how your project picks one up, and what is promised between versions. Read it before you pin or update the tool the tasks run (`charter`) or the Go packages. The file list was checked against the latest GitHub Release on 2026-10-01.

The newest release: [github.com/joeblew999/charter/releases/latest](https://github.com/joeblew999/charter/releases/latest)

## How to get the latest release

```sh
go run github.com/joeblew999/charter/cmd/charter@latest help                 # the tool, no install (needs Go)
go get github.com/joeblew999/charter/go@latest                   # the Go packages, run in the project
curl -fsSL https://github.com/joeblew999/charter/releases/latest/download/charter_darwin_arm64.tar.gz | tar xz charter   # the tool as a binary, into ./charter
```

As a mise tool: [The charter tool](dev.md#how-to-run-it).

## What a release contains

One GitHub Release per version, with these files. No file name carries the version.

| File | What it is |
|---|---|
| `charter_linux_amd64.tar.gz`, `charter_linux_arm64.tar.gz`, `charter_darwin_amd64.tar.gz`, `charter_darwin_arm64.tar.gz` | The tool as a binary for that system. Each archive holds the binary `charter` and a `README.md` |
| `checksums.txt` | The SHA-256 of each of those four archives |
| `notes-go-sdk-go.tar.gz`, `notes-go-sdk-typescript.tar.gz` | The Go and TypeScript SDK sources that Fern generates for charter's own Go Worker. Generated fresh for the release, then checked |
| `notes-go-specs.tar.gz` | That Worker's `openapi.json` and `asyncapi.json` |
| `notes-go-cli-linux-amd64` | The Fern CLI of that Worker, for Linux on amd64 |
| `notes-ts-sdk-go.tar.gz`, `notes-ts-sdk-typescript.tar.gz`, `notes-ts-specs.tar.gz`, `notes-ts-cli-linux-amd64` | The same four for charter's TypeScript Worker |

What a project uses from a release is the tool and the Go packages. The Go packages are not files of the Release: Go fetches them from the repository at the release's tag. The SDK, spec and CLI files are those of charter's own example API, not of yours. Your project's release workflow (`.github/workflows/release.yml`, written by `mise run workflows`) attaches the same kinds of files for your API to your repository's releases.

Not in a release:

- **No Windows build of the tool.** It manages process groups, which is Unix-only.
- **No SDK packages.** The SDKs are source archives, not packages on npm or Go modules of their own.
- **The Fern CLI only for Linux on amd64.**

## The stable links

Because the file names carry no version, one link always gives the newest file:

```text
https://github.com/joeblew999/charter/releases/latest/download/<file>
```

For example `https://github.com/joeblew999/charter/releases/latest/download/checksums.txt`. Use `releases/latest` and `@latest` in anything you write down: a link with a version in it is out of date after the next release.

## The tags of a release

A release is four git tags on one commit.

| Tag | What it is for |
|---|---|
| `vX.Y.Z` | The release itself, and the version of the root Go module `github.com/joeblew999/charter`, which holds the tool (`cmd/charter`). The maintainer pushes this one; the GitHub Release is made for it |
| `go/vX.Y.Z` | The version of the Go module `github.com/joeblew999/charter/go`, which holds the packages and the Worker glue |
| `examples/notes-go/sdk/go/vX.Y.Z` | The version of the Go module `github.com/joeblew999/charter/examples/notes-go/sdk/go`, which is the Go SDK of the notes API |

The two extra tags exist because of a rule of Go: a module that sits in a subdirectory of a repository only has a version under a tag that starts with the directory. Without `go/vX.Y.Z`, `go get` would not find the release; without `examples/notes-go/sdk/go/vX.Y.Z`, another repo could only get the SDK at a branch or a commit. The release workflow adds all three on the commit of `vX.Y.Z`. No release has carried the `sdk/go` tag yet: the releases up to 2026-10-01 were cut before that module existed.

A version is a semantic version: `v1.2.3`, or `v1.2.3-rc.1` for a pre-release. When `charter release` creates the GitHub Release of a tag with a hyphen, it marks it as a pre-release.

Your own project's Go module is at its root, so `vX.Y.Z` is its version. It has one Go module in a subdirectory, `sdk/go/`, once you publish its Go SDK ([Giving the Go SDK to another repo](../guides/sdks.md#giving-the-go-sdk-to-another-repo)). Its task `release:tags` adds `sdk/go/vX.Y.Z` to your releases for the same reason.

## How a project picks up a new release

A project made by `charter new` depends on a release in two places ([Configuration](config.md#pinned-versions-and-where-each-pin-lives)).

**The tool**, pinned in `mise.toml`:

```sh
mise up --bump "go:github.com/joeblew999/charter/cmd/charter"   # install the newest release and write it into mise.toml
```

Where the pin is `latest`, `mise up` alone installs the newest release.

**The Go packages**, pinned in `go.mod`:

```sh
go get -u github.com/joeblew999/charter/go && go mod tidy   # the newest release of the packages
```

Then prove the project against both:

```sh
mise run workflows   # the workflows, as the new tool writes them
mise run check           # every local check
```

What an update does not touch: the files `charter new` copied into your project. They are yours, and no command updates them: the contract and handlers, `worker.mjs`, the two `platform_*.go` files, the spec command, the test programs in `test/`, and the tasks in `mise.toml`. A fix to one of those in charter reaches your project only if you copy it. Compare with the [charter repository](https://github.com/joeblew999/charter) when a release's notes mention them.

### Getting the faster Worker in a project made before it

A project made before Go runtimes were reused keeps working after an update, at its old cost: ten times the CPU per request or more ([Benchmarks](../benchmarks.md)). Three of the copied files carry the change. After updating the tool and the Go packages as above:

1. **The build task** in `mise.toml`, if it still calls `tinygo build` itself:

   ```toml
   [tasks."build"]
   run = "charter wasm-build"
   ```

2. **The Worker's entry:** replace the project's own copies of the Worker's JavaScript (the folder beside `main.go`) with one file, `worker.mjs`, as [charter's](https://github.com/joeblew999/charter/blob/main/examples/notes-go/worker.mjs), and name it in `cloudflare.config.ts`. It imports `./build/go.mjs`, which the build now writes from the Go library, and `await go.warm({ paths: ["/api/openapi.json"] })` starts two Go runtimes while the module loads.

3. **`main.go`:** `transport.Run(api.Handler(env()))` in place of `workers.Serve(transport.Serve(api.Handler(env())))`, so the Go program stays alive after a response.

Then `mise run check`, deploy, and `mise run bench`. One thing to read first: a package variable can now hold what an earlier request left there ([Go on Cloudflare Workers](../concepts/workers-go.md#a-go-runtime-is-not-a-server-process)).

## What compatibility is promised

None. The releases are `v0` versions, and under semantic versioning a `v0` release may change anything. Between two releases the exported names of the Go packages, the tool's commands and flags, the tasks and the workflow templates can all change. The release notes are generated from the commits: there is no separate list of breaking changes. Pin exact versions, update on purpose, and run `mise run check` after.

What the checks do guarantee, for the commit a release is built from:

- **The Go packages pass their tests and run under TinyGo.** The check workflow runs lint, the Go tests, the spec drift check, the TinyGo build, and the real-time and MCP tests against both the native build and the Wasm under workerd.
- **The SDKs in the release were generated from that commit's specs and passed their checks** (build, vet and tests for Go; a typecheck for TypeScript), in the release workflow itself.
- **A project scaffolded by `charter new` builds under its own name.** A test of the tool makes one, checks its names, and runs `go build`, `go vet` and three of its tests.

What they do not guarantee:

- **That a new project's full `mise run check` passes.** That is run by hand before a release, not by a workflow.
- **That anything works on Cloudflare itself.** The tests against a deployed Worker (`mise run live-test`, `mise run soak`) are run by hand. Run `mise run live-test` after each of your own deploys.
- **That a tag was only pushed on a green commit.** Nothing stops a tag on a commit whose checks failed; the release workflows build what is there.
