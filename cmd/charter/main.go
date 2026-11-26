// Command charter is the one command of a Go app on Cloudflare Workers. It drives the tools that own each job (gsx,
// Vite, wrangler, Go) and adds what they have no answer for: Go runtimes kept between requests, an API's contract,
// specs and SDKs, languages and what search engines read.
//
//	go get -tool github.com/joeblew999/charter/cmd/charter
//	go tool charter <command>
package main

import (
	"fmt"
	"os"

	"github.com/joeblew999/charter/internal/command"
	"github.com/joeblew999/charter/internal/version"
)

type cmd struct {
	name, about string
	run         func(args []string)
}

// The commands are Vite+'s (https://viteplus.dev/guide/), for a Go app on Workers: the same verbs, each driving the
// tool that owns the job. What is no verb is a task of charter run. deploy is the one word of charter's own.
var commands = []cmd{
	{"create", "a new app from a template: charter create my-app [-from api|site|notes]", command.Create},
	{"install", "everything the app pins: npm's packages, Go's modules", command.Install},
	{"dev", "the loop you work in, on the real runtime: a save rebuilds and reloads; every other command works beside it", command.Dev},
	{"check", "everything that can be said without running the app: format, lint, the contract, the languages (--fix)", command.Check},
	{"fmt", "Go and gsx files as gofmt and gsx fmt have them (--check)", command.Fmt},
	{"lint", "go.mod tidy, go vet for the host and the Worker, structpages-lint", command.Lint},
	{"test", "go test, then the Worker on wrangler dev: its API, its own tests, the search-engine audit (a suite; --base url)", command.Test},
	{"build", "the messages, the components' Go, the browser's files (Vite) and the Wasm: what wrangler runs and deploys", command.Build},
	{"preview", "the production build on wrangler dev", command.Preview},
	{"deploy", "the D1 migrations, then wrangler deploy: charter deploy [staging|production]", command.Deploy},
	{"pack", "what others consume: an SDK from the contract, with Fern (--prove, --publish)", command.SDK},
	{"run", "a task: translate, languages, lighthouse, badges, logs, bench, … and the app's own (alone: lists them)", command.Run},
	{"exec", "one of the app's tools as it pins it: charter exec wrangler …, vite, gsx, gsxui", command.Exec},
	{"add", "something more: locale:de, ui:dialog, screen:language-list, d1:DB, a Go module, an npm package", command.Add},
	{"remove", "a Go module or an npm package", command.Remove},
	{"update", "every pin to its latest: Go modules, npm packages", command.Update},
	{"outdated", "every pin with a newer version", command.Outdated},
	{"upgrade", "charter itself to its latest", command.Upgrade},
	{"staged", "format and check what is staged for a commit (install: as the pre-commit hook)", command.Staged},
	{"cache", "charter cache clean: everything a build wrote, removed", command.Cache},
	{"env", "what this machine has of what an app needs", command.Env},
	{"version", "this charter's version", func([]string) { fmt.Println("charter", version.String()) }},
}

func main() {
	if len(os.Args) > 1 {
		for _, c := range commands {
			if c.name == os.Args[1] {
				c.run(os.Args[2:])
				return
			}
		}
	}
	fmt.Printf("charter %s: a Go app on Cloudflare Workers, with gsx, Vite and wrangler.\n\n", version.String())
	for _, c := range commands {
		fmt.Printf("  charter %-9s %s\n", c.name, c.about)
	}
	fmt.Println("\ncharter <command> -h: its flags.")
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		return
	}
	fmt.Fprintf(os.Stderr, "\ncharter has no command %s\n", os.Args[1])
	os.Exit(2)
}
