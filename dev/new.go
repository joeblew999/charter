package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

func init() {
	commands["new"] = command{"-name <name> [-module <go module>] [-subdomain <workers.dev subdomain>] [-into <dir>] [-from <checkout>]",
		"create a new Go API project: the tested example (api/go/) under your name, with its tasks, Fern folder, tests and docs", newProject}
	commands["version"] = command{"", "the release this tool is, which is what new pins a project to", func([]string) error {
		fmt.Println(orCheckout(toolVersion()))
		return nil
	}}
	anywhere["new"], anywhere["version"] = true, true
}

const (
	repoURL    = "https://github.com/joeblew999/orpc-api"
	repoModule = "github.com/joeblew999/orpc-api"
)

// What a new project takes from this repo, as it is apart from names. Folders are copied whole.
var projectFiles = []string{
	"api/go/main.go", "api/go/platform_js.go", "api/go/platform_other.go", "api/go/cloudflare.config.ts",
	"api/go/package.json", "api/go/package-lock.json", "api/go/vite.config.ts", "api/go/.gitignore",
	"api/go/go.mod", "api/go/go.sum", "api/go/api/", "api/go/cmd/spec/", "api/go/worker/",
	"migrations/",
	"sdk/package.json", "sdk/package-lock.json", "sdk/tsconfig.base.json", "sdk/fern/fern.config.json", "sdk/fern/apis/api-go/",
	"test/live-test.mjs", "test/sdk-live-test.mjs", "test/mcp-test.mjs", "test/soak.mjs", "test/soak-go/",
	"rust-toolchain.toml", ".gitignore", ".gitattributes",
}

// Left behind: what only makes sense beside the oRPC Worker.
var projectSkip = map[string]bool{"api/go/api/surface_test.go": true, "test/soak-go/soak-go": true}

// The tasks a Go-only project keeps from mise.toml.
var projectTasks = regexp.MustCompile(`^(setup|check|doctor|upstream:status|dev:check|dev:workflows|docs:.*|api:go:.*|sdk:(list|check-spec|gen|check|ready|publish|publish:check|publish:fresh|cli:build|clean|dist|dist:cli)|cloudflare:.*|release|release:tags)$`)

// newProject makes a project that is the Go half of this repo under another name: the notes API as
// a starting contract, every task, the Fern folder, the tests, a docs folder. The files come from
// this repo at the tool's own version (a tag, cloned), or from -from, or from the checkout the tool
// is run in, so a new project starts from code that passed this repo's checks, not from a template
// kept beside it.
func newProject(args []string) error {
	var name, module, subdomain, into, from string
	flags("new", args, func(f *flag.FlagSet) {
		f.StringVar(&name, "name", "", "the project and its Worker, e.g. billing-api (lower case, digits, hyphens)")
		f.StringVar(&module, "module", "", "its Go module path (default: github.com/<your GitHub login>/<name>)")
		f.StringVar(&subdomain, "subdomain", placeholderSubdomain, "your account's workers.dev subdomain: the Worker's URL is https://<name>.<subdomain>.workers.dev (the default is a placeholder)")
		f.StringVar(&into, "into", "", "where to create it (default: ./<name>)")
		f.StringVar(&from, "from", "", "a checkout of orpc-api to copy from (default: this tool's version)")
	})
	if !regexp.MustCompile(`^[a-z][a-z0-9-]{1,40}[a-z0-9]$`).MatchString(name) {
		return errors.New("new needs -name: lower-case letters, digits and hyphens, e.g. -name billing-api")
	}
	if !regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`).MatchString(subdomain) {
		return errors.New("new: -subdomain is the one word before .workers.dev, e.g. -subdomain acme")
	}
	if into == "" {
		into = filepath.Join(started, name)
	} else if !filepath.IsAbs(into) {
		into = filepath.Join(started, into)
	}
	if entries, _ := os.ReadDir(into); len(entries) > 0 {
		return fmt.Errorf("%s is not empty", into)
	}
	if module == "" {
		login, err := output(".", "gh", "api", "user", "--jq", ".login")
		if err != nil || login == "" {
			return errors.New("new needs -module (or a gh login to default it from), e.g. -module github.com/you/" + name)
		}
		module = "github.com/" + login + "/" + name
	}

	// Where the files come from, and which version of the dev tool and the Go packages the project pins.
	version, local := toolVersion(), false
	switch {
	case from != "":
		local = true
	case version == "":
		dir, ok := root()
		if !ok {
			return errors.New("new: run it as go run " + repoModule + "/dev@<version>, or pass -from <checkout of orpc-api>")
		}
		from, local = dir, true
	default:
		tmp, err := os.MkdirTemp("", "orpc-api-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := quiet(".", nil, "git", "clone", "--quiet", "--depth", "1", "--branch", version, repoURL, tmp); err != nil {
			return fmt.Errorf("cloning %s at %s: %w", repoURL, version, err)
		}
		from = tmp
	}
	from, err := filepath.Abs(from)
	if err != nil {
		return err
	}
	if !exists(filepath.Join(from, "api", "go", "api", "contract.go")) {
		return fmt.Errorf("%s is not a checkout of orpc-api", from)
	}
	pin := version
	if pin == "" {
		pin = "latest" // from a checkout: the tasks name the newest release of the tool
	}

	fmt.Println(pinned(version, local, from))

	tasks, err := os.ReadFile(filepath.Join(from, "mise.toml"))
	if err != nil {
		return err
	}
	names := renamer(name, module)
	// This repo's Workers are on its owner's workers.dev subdomain; the project's are on its own
	// (mise.toml's default URL and the copied specs, which must agree for the spec check to pass).
	owner := ownerSubdomain.FindSubmatch(tasks)
	if owner == nil {
		return fmt.Errorf("%s/mise.toml has no workers.dev default for API_GO_URL", from)
	}
	renamed := names
	names = func(s string) string {
		return strings.ReplaceAll(renamed(s), "."+string(owner[1])+".workers.dev", "."+subdomain+".workers.dev")
	}
	for _, entry := range projectFiles {
		source := filepath.Join(from, entry)
		err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(from, path)
			if projectSkip[filepath.ToSlash(rel)] {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.HasSuffix(rel, "package-lock.json") && !strings.HasSuffix(rel, "go.sum") {
				content = []byte(names(string(content)))
			}
			return write(filepath.Join(into, rel), string(content))
		})
		if err != nil {
			return err
		}
	}

	generated := map[string]string{
		"mise.toml":       names(projectMise(string(tasks), name, pin)),
		"go.work":         "go 1.27.1\n\ntoolchain go1.27.1\n\nuse ./api/go\n",
		"README.md":       projectReadme(name, pin),
		"AGENTS.md":       "# For agents\n\nEverything about this project is in [docs/](docs/README.md), the same pages developers read. Read [docs/README.md](docs/README.md), then [docs/rules.md](docs/rules.md): the rules are binding.\n",
		"CLAUDE.md":       "@AGENTS.md\n",
		"docs/README.md":  projectDocs(name),
		"docs/rules.md":   projectRules,
		"docs/writing.md": docsWriting,
		"sdk/.gitignore":  "node_modules/\nout/\n.work/\n",
		"test/.gitignore": "soak-go/soak-go\n",
	}
	for path, content := range generated {
		if err := write(filepath.Join(into, path), content); err != nil {
			return err
		}
	}

	// The project's own module: its packages are its own, the reusable ones stay imports of orpc-api's.
	gomod, err := os.ReadFile(filepath.Join(into, "api", "go", "go.mod"))
	if err != nil {
		return err
	}
	lines := strings.SplitN(string(gomod), "\n", 2)
	mod := "module " + module + "/api/go\n" + lines[1]
	if err := write(filepath.Join(into, "api", "go", "go.mod"), mod); err != nil {
		return err
	}
	library := repoModule + "/api/go"
	if local {
		// From a checkout: build against that checkout, so unreleased changes work. Remove the
		// replace line once you depend on a release.
		if err := quiet(filepath.Join(into, "api", "go"), nil, "go", "mod", "edit", "-require="+library+"@v0.0.0", "-replace="+library+"="+filepath.Join(from, "api", "go")); err != nil {
			return err
		}
	} else if err := quiet(filepath.Join(into, "api", "go"), nil, "go", "mod", "edit", "-require="+library+"@"+version); err != nil {
		return err
	}
	if err := quiet(filepath.Join(into, "api", "go"), []string{"GOWORK=off"}, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy in the new project: %w", err)
	}
	// A new module path sorts differently among the imports: format, so the project's own lint passes.
	if err := quiet(into, nil, "gofmt", "-w", "api/go", "test/soak-go"); err != nil {
		return err
	}

	fmt.Printf(`created %s (module %s/api/go, Worker %s)

  cd %s && git init
  mise install && mise run setup     # tools, then npm packages
  mise run check                     # lint, tests, spec drift, the TinyGo build, the live test natively and under workerd
  mise run api:go:run                # the API natively: http://localhost:5174/api/hello
  mise run api:go:deploy             # to Cloudflare, then: mise run api:go:live-test

The API is the notes example: change api/go/api/contract.go, then mise run api:go:spec.
To put your own API in its place: %s
With a GitHub repo: mise run dev:workflows, mise run docs:setup, mise run docs:pages.

%s
Cost: on Cloudflare a simple read uses under 1 ms of CPU, a database read 1 to 2 ms, a write about 2 ms;
the first request in a new isolate about 10 ms (measured 2026-10-02). mise run api:go:bench measures yours.
`, into, module, name, into, replaceGuide, afterDeploy(name, subdomain))
	return nil
}

const (
	placeholderSubdomain = "your-subdomain"
	replaceGuide         = "https://joeblew999.github.io/orpc-api/guides/replace-the-example.html"
)

// The workers.dev subdomain this repo's own Workers are deployed on, as its mise.toml names it.
var ownerSubdomain = regexp.MustCompile(`https://orpc-api-go\.([a-z0-9-]+)\.workers\.dev`)

// afterDeploy says where the project thinks its Worker is, and what to do when that is not so.
func afterDeploy(name, subdomain string) string {
	url := "https://" + name + "." + subdomain + ".workers.dev"
	fix := "put the URL it prints into mise.local.toml as API_GO_URL (or make it the default in mise.toml, which CI reads too), then: mise run api:go:spec"
	if subdomain == placeholderSubdomain {
		return "The Worker's URL is a placeholder (" + url + ") in mise.toml and in the specs: -subdomain was not given.\nAfter the first mise run api:go:deploy, " + fix + "."
	}
	return "The Worker's URL is " + url + " in mise.toml and in the specs.\nIf mise run api:go:deploy prints another, " + fix + "."
}

// pinned is the first line new prints: which release the tool is, and what the project pins to it.
// (Minutes after a release, dev@latest can still be the one before.)
func pinned(version string, local bool, from string) string {
	if !local {
		return fmt.Sprintf("dev %s: pins the dev tool to %s in mise.toml and the Go packages to %s in api/go/go.mod", version, version, version)
	}
	tool := version
	if tool == "" {
		tool = "latest"
	}
	return fmt.Sprintf("dev %s, copying from %s: pins the dev tool to %s in mise.toml; api/go/go.mod builds against that checkout (a replace line)", orCheckout(version), from, tool)
}

// orCheckout names the tool's version, or says that it has none.
func orCheckout(version string) string {
	if version == "" {
		return "(not a release: built from a checkout)"
	}
	return version
}

// toolVersion is the release this tool was built from (go run ...dev@v0.2.0), or "" from a checkout.
func toolVersion() string {
	if built != "" && !strings.Contains(built, "SNAPSHOT") {
		return built // a binary from a GitHub Release
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || !strings.HasPrefix(info.Main.Version, "v") || strings.Contains(info.Main.Version, "-0.") {
		return "" // (devel), or a pseudo-version of an untagged commit
	}
	return info.Main.Version
}

func write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// renamer turns this repo's names into the project's: the Worker (orpc-api-go), the project
// (orpc-api), the SDK's names (OrpcApi, orpcapi), and the import paths of what is the project's own:
// the example's package and the Go SDK's module (<module>/sdk/go, so another repo can go get it).
// The reusable packages keep their orpc-api import path: the project imports them.
func renamer(name, module string) func(string) string {
	pascal, flat := "", strings.ReplaceAll(name, "-", "")
	for _, part := range strings.Split(name, "-") {
		if part != "" {
			pascal += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	const library = "\x00library\x00" // keeps the orpc-api module path out of the renaming
	own := strings.NewReplacer(`"`+repoModule+`/api/go/api"`, `"`+module+`/api/go/api"`, repoModule+"/sdk/go", module+"/sdk/go", repoModule, library)
	rename := strings.NewReplacer("orpc-api-go", name, "orpc-api", name, "OrpcApi", pascal, "orpcapi", flat)
	return func(s string) string {
		return strings.ReplaceAll(rename.Replace(own.Replace(s)), library, repoModule)
	}
}

// projectMise is this repo's mise.toml cut down to a Go-only project: the same tools, the Go and
// SDK tasks, and the dev tool run at a pinned version instead of from ./dev.
func projectMise(source, name, pin string) string {
	// The dev tool is a mise tool of the project: one pin, and `dev` on the path of every task.
	const tool = "dev"
	pinned := strings.TrimPrefix(pin, "v")
	var out []string
	keep, task := true, ""
	for _, line := range strings.Split(source, "\n") {
		if task == "check" && strings.HasPrefix(line, "depends =") {
			line = `depends = ["api:go:check", "dev:check"]` // whatever this repo checks, a Go project checks these
		}
		switch {
		case strings.HasPrefix(line, "# ----"), strings.HasPrefix(line, "# orpc-api:"), strings.HasPrefix(line, "# and docs generated by Fern."):
			continue // section banners, the old first two lines
		case strings.HasPrefix(line, "[tasks"):
			task = strings.Trim(strings.TrimSuffix(strings.TrimPrefix(line, "[tasks."), "]"), `"`)
			keep = projectTasks.MatchString(task)
		case strings.HasPrefix(line, "["):
			keep, task = true, ""
		}
		if strings.HasPrefix(line, "goreleaser =") || strings.HasPrefix(line, "# Builds and publishes the dev tool") {
			continue // this repo releases the tool; a project uses it
		}
		if !keep || strings.HasPrefix(line, "API_URL =") || strings.HasPrefix(line, "HARNESS_") || strings.HasPrefix(line, "SHOWCASE_") || strings.HasPrefix(line, "PORT =") {
			continue
		}
		out = append(out, line)
	}
	text := strings.Join(out, "\n")
	text = strings.NewReplacer(
		"go run ./dev", tool, "go run ../../dev", tool,
		"for dir in api/ts api/go sdk sdk/harness;", "for dir in api/go sdk;",
		`test -z "$(gofmt -l dev)" && go vet ./dev && go test ./dev && `, "",
		"dist-sdk api-ts api-go", "dist-sdk api-go",
		"Install the npm packages: api/ts/, api/go/ (cf), sdk/ (Fern), sdk/harness/", "Install the npm packages: api/go/ (cf), sdk/ (Fern)",
		"# every local check (needs Docker, no Cloudflare account)", "# every local check (no Cloudflare account)",
		"Every LOCAL check (no Cloudflare account; needs Docker)", "Every LOCAL check (no Cloudflare account)",
		"The dev tool's own checks (gofmt, vet, tests), the generated workflows", "The generated workflows",
		"and the same SDK surface as the oRPC contract", "and the contract's rules",
		"what a release ships for both APIs into dist/", "what a release ships into dist/",
		"`api/ts/node_modules/.bin/cf auth login`", "`api/go/node_modules/.bin/cf auth login`",
		"release-tags api/go dev sdk/go", "release-tags api/go sdk/go",
		"api/go/vX.Y.Z, dev/vX.Y.Z, sdk/go/vX.Y.Z", "api/go/vX.Y.Z, sdk/go/vX.Y.Z",
		"# Local dev ports.", "# The local dev port.",
		"# Where the Workers are deployed (cf deploy prints it; there is no cf command to look it up). On\n# another Cloudflare account, set these in mise.local.toml (gitignored).", "# Where the Worker is deployed (cf deploy prints it; there is no cf command to look it up). If it is\n# not this, set it in mise.local.toml (gitignored) or change the default here, then: mise run api:go:spec.",
		"# Every task is one line of plain sh", "# "+name+": a Go API on Cloudflare Workers (Huma on workers-go), made with `dev new` from\n# "+repoURL+".\n# Every task is one line of plain sh",
		"is a command of ./dev (go run ./dev help)", "is a command of the dev tool (`dev help`), pinned under [tools]",
		"\n[env]", "# The tool every task runs ("+repoModule+"/dev): `mise up` moves to a newer release.\n\"go:"+repoModule+"/dev\" = \""+pinned+"\"\n\n[env]",
	).Replace(text)
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
}

func projectReadme(name, pin string) string {
	return "# " + name + `

A contract-first Go API on Cloudflare Workers: Huma on workers-go, built with TinyGo, with real-time
(SSE + WebSocket) and an MCP endpoint. The OpenAPI and AsyncAPI specs are generated from the Go
contract, and Fern generates SDKs and a CLI from them. Made with ` + "`dev new`" + ` from
[orpc-api](` + repoURL + `) (` + pin + `), whose [docs](https://joeblew999.github.io/orpc-api/) explain the design.

` + "```sh" + `
mise install && mise run setup     # tools, then npm packages
mise run check                     # every local check
mise run api:go:run                # natively: http://localhost:5174/api/hello
mise run api:go:dev                # under workerd (first time: mise run api:go:migrate:local)
mise run api:go:deploy             # to Cloudflare, then: mise run api:go:live-test
mise tasks                         # everything else: every task is one line
` + "```" + `

The contract is ` + "`api/go/api/contract.go`" + `. After changing it: ` + "`mise run api:go:spec`" + `. Docs are in [docs/](docs/README.md).
`
}

func projectDocs(name string) string {
	return `---
title: Start here
nav_order: 1
permalink: /
---

# ` + name + `

Everything written about this project lives in this folder. ` + "`AGENTS.md`" + ` only points here.

| Page | What it covers |
|---|---|
| This page | What is what |
| [rules.md](rules.md) | The working rules |
| [writing.md](writing.md) | The rules a page in ` + "`docs/`" + ` is held to |

## What is what

You write the contract in Go; everything else is generated from it.

| | |
|---|---|
| **The contract (the source; you edit this)** | ` + "`api/go/api/contract.go`" + ` (Huma: Go structs and their tags) |
| The server | ` + "`api/go/api/handlers.go`" + ` |
| Write the specs | ` + "`mise run api:go:spec`" + ` |
| **The specs (generated; never edit)** | ` + "`sdk/fern/apis/api-go/*.json`" + ` |
| Fern's settings | ` + "`sdk/fern/apis/api-go/generators.yml`" + ` |
| Generate an SDK | ` + "`mise run sdk:gen api-go <group>`" + ` (go, typescript, typescript-dist, cli) into ` + "`sdk/out/`" + ` |
| The D1 schema | ` + "`migrations/`" + ` |
| The tests a deploy must pass | ` + "`test/`" + ` (` + "`mise run api:go:live-test`, `mise run api:go:soak`" + `) |
| MCP | ` + "`/api/mcp`" + `: every one-shot operation of the contract is a tool |

The project starts as the notes example. Which files hold it, and what to do with each when you put your
own API in: [Replace the example with your API](` + replaceGuide + `).

How it works, what Huma needs on workers-go, the real-time design and the measured costs are documented
once, in [orpc-api's docs](https://joeblew999.github.io/orpc-api/): the Go packages this project imports
(` + "`humaworkers`, `asyncapi`, `follow`, `hub`, `d1`, `humamcp`, `transport`, `specfile`" + `) come from there.
`
}

const projectRules = `---
title: Rules
nav_order: 2
---

# Rules for working in this project

- **` + "`docs/`" + ` is the single source of truth.** Write things down in a page here, following [writing.md](writing.md). ` + "`mise run docs:lint`" + ` checks what a program can, ` + "`mise run docs:review`" + ` has Claude check the rest.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything longer belongs in the dev tool the tasks call.
- **The contract is the source.** After changing ` + "`api/go/api/contract.go`" + `, run ` + "`mise run api:go:spec`" + `. ` + "`mise run check`" + ` fails if a committed spec is stale. Never edit ` + "`sdk/fern/apis/api-go/*.json`" + ` by hand.
- **Everything that ships to Workers builds with TinyGo** (` + "`mise run api:go:build`" + `). ` + "`go test`" + ` can't see TinyGo's gaps, so the check also runs the Wasm under workerd.
- **Test locally and on Cloudflare.** After a deploy, ` + "`mise run api:go:live-test`" + ` must pass against the deployed Worker: some bugs exist only in production.
- **Exact pins.** Tools in ` + "`mise.toml`" + `, Go in ` + "`go.work`" + `, Rust in ` + "`rust-toolchain.toml`" + `. Lockfiles are committed.
- **Workarounds name their upstream issue:** ` + "`Upstream: <owner>/<repo>#<n> (when fixed: ...)`" + ` in the code; ` + "`mise run upstream:status`" + ` lists them.
`
