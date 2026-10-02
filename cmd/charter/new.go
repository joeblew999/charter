package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

func init() {
	commands["new"] = command{"-name <name> [-module <go module>] [-subdomain <workers.dev subdomain>] [-into <dir>] [-from <checkout>]",
		"create a new Go API project: a copy of the tested example (examples/notes-go in the charter repo) under your name", newProject}
	commands["version"] = command{"", "the release this tool is, which is what new pins a project to", func([]string) error {
		fmt.Println(orCheckout(toolVersion()))
		return nil
	}}
	anywhere["new"], anywhere["version"] = true, true
}

const (
	repoURL    = "https://github.com/joeblew999/charter"
	repoModule = "github.com/joeblew999/charter"
	docsURL    = "https://joeblew999.github.io/charter/"
	// The tool as a module's package: what `go run` and mise's go: backend take.
	toolPackage = repoModule + "/cmd/charter"

	// The example a new project is a copy of, and its names in the charter repo.
	exampleDir    = "examples/notes-go"
	exampleModule = repoModule + "/" + exampleDir
	exampleWorker = "charter-notes-go"
	// How the example's tasks run the tool: from the checkout they are in.
	exampleTool = "go run ../../cmd/charter"
	// Not copied: the committed Go SDK. It is generated from the specs, which carry the Worker's URL,
	// and it is a module under the example's path: `mise run sdk:publish` writes the project's own.
	exampleSDK = "sdk/go/"
)

// newProject makes a project that is the example under another name. The example is a complete
// project (its own mise.toml, Go module, package.json, Fern folder, tests), so new copies its files
// as they are and changes only what names it: the Worker, its URL, the Go module, Fern's
// organisation, and how the tasks run this tool. The files come from the charter repo at the tool's
// own version (a tag, cloned), or from -from, or from the checkout the tool is run in, so a new
// project starts from code that passed that repo's checks, not from a template kept beside it.
func newProject(args []string) error {
	var name, module, subdomain, into, from string
	flags("new", args, func(f *flag.FlagSet) {
		f.StringVar(&name, "name", "", "the project and its Worker, e.g. billing-api (lower case, digits, hyphens)")
		f.StringVar(&module, "module", "", "its Go module path (default: github.com/<your GitHub login>/<name>)")
		f.StringVar(&subdomain, "subdomain", placeholderSubdomain, "your account's workers.dev subdomain: the Worker's URL is https://<name>.<subdomain>.workers.dev (the default is a placeholder)")
		f.StringVar(&into, "into", "", "where to create it (default: ./<name>)")
		f.StringVar(&from, "from", "", "a checkout of charter to copy from and to run the tool from (default: this tool's release)")
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

	// Where the files come from. A release: its tag, cloned, and the project pins the tool and the
	// library to it. A checkout: the project runs the tool from there and builds against its library.
	version, checkout := toolVersion(), ""
	switch {
	case from != "":
		checkout = from
	case version == "":
		dir, ok := checkoutAbove(started)
		if !ok {
			return errors.New("new: run it as go run " + toolPackage + "@latest, or pass -from <checkout of charter>")
		}
		checkout = dir
	default:
		tmp, err := os.MkdirTemp("", "charter-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := quiet(".", nil, "git", "clone", "--quiet", "--depth", "1", "--branch", version, repoURL, tmp); err != nil {
			return fmt.Errorf("cloning %s at %s: %w", repoURL, version, err)
		}
		from = tmp
	}
	if checkout != "" {
		var err error
		if checkout, err = filepath.Abs(checkout); err != nil {
			return err
		}
		from = checkout
	}
	source := filepath.Join(from, filepath.FromSlash(exampleDir))
	if !exists(filepath.Join(source, "api", "contract.go")) {
		return fmt.Errorf("%s is not a checkout of charter (no %s)", from, exampleDir)
	}
	fmt.Println(pinned(version, checkout))

	tasks, err := os.ReadFile(filepath.Join(source, "mise.toml"))
	if err != nil {
		return err
	}
	// The example's Worker is on the charter repo owner's workers.dev subdomain; the project's is on
	// its own (mise.toml's default URL and the copied specs, which must agree for the spec check).
	owner := ownerSubdomain.FindSubmatch(tasks)
	if owner == nil {
		return fmt.Errorf("%s/mise.toml has no workers.dev default for API_URL", source)
	}
	tool := "charter" // on the path of every task: mise installs the pinned release
	if checkout != "" {
		tool = "go run " + filepath.Join(checkout, "cmd", "charter")
	}
	rename := strings.NewReplacer(
		exampleModule, module,
		exampleWorker+"."+string(owner[1])+".workers.dev", name+"."+subdomain+".workers.dev",
		exampleWorker, name,
		exampleTool, tool,
	)

	// Everything git knows of the example or would add (so not node_modules, build/ or sdk/out).
	listed, err := output(from, "git", "ls-files", "--cached", "--others", "--exclude-standard", "--", exampleDir)
	if err != nil {
		return fmt.Errorf("listing the files of %s in %s: %w", exampleDir, from, err)
	}
	for _, file := range strings.Split(listed, "\n") {
		rel := strings.TrimPrefix(file, exampleDir+"/")
		if file == "" || strings.HasPrefix(rel, exampleSDK) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(from, filepath.FromSlash(file)))
		if errors.Is(err, os.ErrNotExist) {
			continue // deleted in the checkout, not yet committed
		} else if err != nil {
			return err
		}
		text := string(content)
		if filepath.Base(rel) != "go.sum" {
			text = rename.Replace(text)
		}
		switch rel {
		case "mise.toml":
			if checkout == "" {
				text = strings.Replace(text, "[tools]\n", "[tools]\n# The tool every task runs: `mise up` moves to a newer release.\n\"go:"+toolPackage+"\" = \""+strings.TrimPrefix(version, "v")+"\"\n", 1)
			}
		case "fern/fern.config.json":
			text = fernOrganization.ReplaceAllString(text, `"organization": "`+name+`"`)
		}
		if err := write(filepath.Join(into, filepath.FromSlash(rel)), text); err != nil {
			return err
		}
	}

	// What a repo has around a project: its start pages, the docs folder, the GitHub workflows.
	for path, content := range map[string]string{
		"README.md":       projectReadme(name, orCheckout(version)),
		"AGENTS.md":       "# For agents\n\nEverything about this project is in [docs/](docs/README.md), the same pages developers read. Read [docs/README.md](docs/README.md), then [docs/rules.md](docs/rules.md): the rules are binding.\n",
		"CLAUDE.md":       "@AGENTS.md\n",
		"docs/README.md":  projectDocs(name),
		"docs/rules.md":   projectRules,
		"docs/writing.md": docsWriting,
	} {
		if err := write(filepath.Join(into, path), content); err != nil {
			return err
		}
	}
	if err := writeWorkflows(into); err != nil {
		return err
	}

	// The Go library: the release the tool is, or the checkout's.
	if checkout != "" {
		// Remove go.work and the replace line once you depend on a release.
		if err := quiet(into, nil, "go", "mod", "edit", "-replace="+libraryModule+"="+filepath.Join(checkout, "go")); err != nil {
			return err
		}
		// The tasks `go run` the tool from the checkout, which Go allows for a module of the workspace.
		if err := write(filepath.Join(into, "go.work"), "go "+goVersion(tasks)+"\n\nuse (\n\t.\n\t"+checkout+"\n)\n"); err != nil {
			return err
		}
	} else if err := quiet(into, nil, "go", "mod", "edit", "-dropreplace="+libraryModule, "-require="+libraryModule+"@"+version); err != nil {
		return err
	}
	if err := quiet(into, []string{"GOWORK=off"}, "go", "mod", "tidy"); err != nil {
		return fmt.Errorf("go mod tidy in the new project: %w", err)
	}
	// A new module path sorts differently among the imports: format, so the project's own lint passes.
	if err := quiet(into, nil, "gofmt", "-w", "."); err != nil {
		return err
	}

	fmt.Printf(`created %s (module %s, Worker %s)

  cd %s && git init
  mise install && mise run setup     # tools, then npm packages
  mise run check                     # lint, tests, spec drift, the TinyGo build, the live test natively and under workerd
  mise run run                       # the API natively: http://localhost:5174/api/hello
  mise run deploy                    # to Cloudflare, then: mise run live-test

The API is the notes example: change api/contract.go, then mise run spec.
The Go library (`+libraryModule+`) is a requirement in go.mod. Its Worker glue is not in the
project: mise run build writes it into build/ from the version the project requires.
To put your own API in its place: %s
With a GitHub repo: mise run docs:setup, mise run docs:pages. The workflows are in .github/workflows.

%s
Cost: on Cloudflare a simple read uses under 1 ms of CPU, a database read 1 to 2 ms, a write about 2 ms;
the first request in a new isolate about 10 ms (measured 2026-10-02). mise run bench measures yours.
`, into, module, name, into, replaceGuide, afterDeploy(name, subdomain))
	return nil
}

const (
	placeholderSubdomain = "your-subdomain"
	replaceGuide         = docsURL + "guides/replace-the-example.html"
)

var (
	// The workers.dev subdomain the example's Worker is deployed on, as its mise.toml names it.
	ownerSubdomain = regexp.MustCompile(`https://` + exampleWorker + `\.([a-z0-9-]+)\.workers\.dev`)
	// Fern's organisation: the name its generated READMEs and packages start from.
	fernOrganization = regexp.MustCompile(`"organization":\s*"[^"]*"`)
	goPin            = regexp.MustCompile(`(?m)^go = "([^"]+)"`)
)

// goVersion is the Go a project's mise.toml pins.
func goVersion(tasks []byte) string {
	if m := goPin.FindSubmatch(tasks); m != nil {
		return string(m[1])
	}
	return "1.27.1"
}

// checkoutAbove is the checkout of charter that dir is in, if it is in one.
func checkoutAbove(dir string) (string, bool) {
	for ; ; dir = filepath.Dir(dir) {
		if exists(filepath.Join(dir, "cmd", "charter", "main.go")) && exists(filepath.Join(dir, filepath.FromSlash(exampleDir))) {
			return dir, true
		}
		if dir == filepath.Dir(dir) {
			return "", false
		}
	}
}

// afterDeploy says where the project thinks its Worker is, and what to do when that is not so.
func afterDeploy(name, subdomain string) string {
	url := "https://" + name + "." + subdomain + ".workers.dev"
	fix := "put the URL it prints into mise.local.toml as API_URL (or make it the default in mise.toml, which CI reads too), then: mise run spec"
	if subdomain == placeholderSubdomain {
		return "The Worker's URL is a placeholder (" + url + ") in mise.toml and in the specs: -subdomain was not given.\nAfter the first mise run deploy, " + fix + "."
	}
	return "The Worker's URL is " + url + " in mise.toml and in the specs.\nIf mise run deploy prints another, " + fix + "."
}

// pinned is the first line new prints: which release the tool is, and what the project uses of it.
// (Minutes after a release, @latest can still be the one before.)
func pinned(version, checkout string) string {
	if checkout == "" {
		return fmt.Sprintf("charter %s: pins the tool to %s in mise.toml and the Go library to %s in go.mod", version, version, version)
	}
	return fmt.Sprintf("charter %s, copying from %s: the tasks run the tool from that checkout (go.work lets them), and go.mod builds against its library (a replace line)", orCheckout(version), checkout)
}

// orCheckout names the tool's version, or says that it has none.
func orCheckout(version string) string {
	if version == "" {
		return "(not a release: built from a checkout)"
	}
	return version
}

// toolVersion is the release this tool was built from (go run ...cmd/charter@v0.3.0), or "" from a
// checkout.
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

func projectReadme(name, version string) string {
	return "# " + name + `

A contract-first Go API on Cloudflare Workers: Huma on workers-go, built with TinyGo, with real-time
(SSE + WebSocket) and an MCP endpoint. The OpenAPI and AsyncAPI specs are generated from the Go
contract, and Fern generates SDKs and a CLI from them. Made with ` + "`charter new`" + ` from
[charter](` + repoURL + `) (` + version + `), whose [docs](` + docsURL + `) explain the design.

` + "```sh" + `
mise install && mise run setup     # tools, then npm packages
mise run check                     # every local check
mise run run                       # natively: http://localhost:5174/api/hello
mise run dev                       # under workerd (first time: mise run migrate:local)
mise run deploy                    # to Cloudflare, then: mise run live-test
mise tasks                         # everything else: every task is one line
` + "```" + `

The contract is ` + "`api/contract.go`" + `. After changing it: ` + "`mise run spec`" + `. Docs are in [docs/](docs/README.md).
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
| **The contract (the source; you edit this)** | ` + "`api/contract.go`" + ` (Huma: Go structs and their tags) |
| The server | ` + "`api/handlers.go`" + ` |
| The Worker's entry | ` + "`worker.mjs`" + `. The rest of the JavaScript is the Go library's: the build writes it into ` + "`build/`" + ` |
| Write the specs | ` + "`mise run spec`" + ` |
| **The specs (generated; never edit)** | ` + "`fern/openapi.json`, `fern/asyncapi.json`" + ` |
| Fern's settings | ` + "`fern/generators.yml`" + ` |
| Generate an SDK | ` + "`mise run sdk:gen <group>`" + ` (go, typescript, typescript-dist, cli) into ` + "`sdk/out/`" + ` |
| The D1 schema | ` + "`migrations/`" + ` |
| The tests a deploy must pass | ` + "`test/`" + ` (` + "`mise run live-test`, `mise run soak`" + `) |
| MCP | ` + "`/api/mcp`" + `: every one-shot operation of the contract is a tool |
| The tasks | ` + "`mise.toml`" + `: each is one line, and what needs more is a command of the charter tool it pins |

The project starts as the notes example. Which files hold it, and what to do with each when you put your
own API in: [Replace the example with your API](` + replaceGuide + `).

How it works, what Huma needs on workers-go, the real-time design and the measured costs are documented
once, in [charter's docs](` + docsURL + `): the Go library this project requires
(` + "`" + libraryModule + "`: `humaworkers`, `asyncapi`, `follow`, `hub`, `d1`, `humamcp`, `transport`, `specfile`" + `, and the Worker
glue the build writes into ` + "`build/`" + `) comes from there.
`
}

const projectRules = `---
title: Rules
nav_order: 2
---

# Rules for working in this project

- **` + "`docs/`" + ` is the single source of truth.** Write things down in a page here, following [writing.md](writing.md). ` + "`mise run docs:lint`" + ` checks what a program can, ` + "`mise run docs:review`" + ` has Claude check the rest.
- **mise drives everything, locally and on GitHub, and every task is one line.** Anything longer belongs in the charter tool the tasks call.
- **The contract is the source.** After changing ` + "`api/contract.go`" + `, run ` + "`mise run spec`" + `. ` + "`mise run check`" + ` fails if a committed spec is stale. Never edit ` + "`fern/openapi.json`" + ` or ` + "`fern/asyncapi.json`" + ` by hand.
- **Everything that ships to Workers builds with TinyGo** (` + "`mise run build`" + `). ` + "`go test`" + ` can't see TinyGo's gaps, so the check also runs the Wasm under workerd.
- **Test locally and on Cloudflare.** After a deploy, ` + "`mise run live-test`" + ` must pass against the deployed Worker: some bugs exist only in production.
- **Exact pins.** Tools in ` + "`mise.toml`" + `, Go modules in ` + "`go.mod`" + `, npm packages in ` + "`package.json`" + `. Lockfiles are committed.
- **Workarounds name their upstream issue:** ` + "`Upstream: <owner>/<repo>#<n> (when fixed: ...)`" + ` in the code; ` + "`mise run upstream:status`" + ` lists them.
`
