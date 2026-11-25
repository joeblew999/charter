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
	"github.com/joeblew999/charter/internal/command/intl"
	"github.com/joeblew999/charter/internal/command/seo"
	"github.com/joeblew999/charter/internal/version"
)

type cmd struct {
	name, about string
	run         func(args []string)
}

var commands = []cmd{
	{"dev", "the dev loop: a save rebuilds and reloads (gsx dev; wrangler dev for an app with no gsx)", command.Dev},
	{"build", "the messages, the components' Go, the browser's files (Vite) and the Wasm: what wrangler runs and deploys", command.Build},
	{"preview", "the Worker as Cloudflare runs it, on wrangler dev", command.Preview},
	{"check", "everything before a push: format, vet, the specs, go test, the languages, the Worker on wrangler dev", command.Check},
	{"test", "one of charter.toml's [test.suites] against the Worker (charter test e2e; -base https://… for a deployed one)", command.Test},
	{"with", "a command run with the Worker on wrangler dev, BASE its URL", command.With},
	{"deploy", "the D1 migrations, then wrangler deploy (charter deploy production)", command.Deploy},
	{"live-test", "the standard test and [test] live against the deployed Worker", command.LiveTest},
	{"spec", "an API's specs from its contract: OpenAPI, AsyncAPI, Fern's settings, the queries' Go (-check)", command.Spec},
	{"sdk", "an SDK from the specs with Fern, in Docker (-prove, -publish)", command.SDK},
	{"intl", "the app's languages: charter intl help", intl.Main},
	{"seo", "what search engines read: charter seo help", seo.Main},
	{"new", "a new app from an example: charter new my-app [-from api|site|notes]", command.New},
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
		fmt.Printf("  charter %-10s %s\n", c.name, c.about)
	}
	fmt.Println("\ncharter <command> -h: its flags.")
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help" {
		return
	}
	fmt.Fprintf(os.Stderr, "\ncharter has no command %s\n", os.Args[1])
	os.Exit(2)
}
