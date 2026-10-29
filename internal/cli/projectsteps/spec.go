package projectsteps

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/fern"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/project"
)

// Spec runs charter spec: both specs from the contract (fern/openapi.json, fern/asyncapi.json), Fern's settings
// (fern/generators.yml) from them and [sdks], and the database's code from migrations/ (db = "sqlc"). With -check it
// writes nothing, and fails where what is there is not what the contract gives.
func Spec(args []string) {
	fs := flag.NewFlagSet("charter spec", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "write nothing; fail where a file is not what the contract gives")
	fs.Parse(args)
	check(spec(load(), *checkOnly))
}

// spec writes (or with check, compares) what the contract gives, in the project's root: from a Go project's api.API,
// or a TypeScript project's api/spec.ts.
func spec(p *project.Project, checkOnly bool) error {
	server := p.SpecServer
	if server == "" {
		server = p.URL
	}
	if server == "" {
		return errors.New("charter.toml has no url: the specs name the server the Worker is deployed at")
	}
	// The Go SDK's module: the project's own Go module, or the Worker's name, with /sdk/go.
	module := goModule(p.Root)
	var command []string
	switch {
	case module != "":
		program, err := specProgram(p.Root, module)
		if err != nil {
			return err
		}
		command = []string{"go", "run", program}
	case exists(filepath.Join(p.Root, "api", "spec.ts")):
		program, err := tsSpecProgram(p.Root)
		if err != nil {
			return err
		}
		command, module = []string{"node", program}, p.Name
	default:
		return errors.New("charter spec writes the specs from the contract: a Go project's api.API (go.mod), or a TypeScript project's api/spec.ts, and this project has neither")
	}
	if checkOnly {
		command = append(command, "-check")
	}
	if err := run.Run(p.Root, nil, command[0], append(command[1:], "fern/openapi.json", "fern/asyncapi.json", server)...); err != nil {
		return err
	}
	if err := fernSettings(p, module+"/sdk/go", checkOnly); err != nil {
		return err
	}
	if p.DB != "" {
		return dbcode.Write(p.Root, p.DB, checkOnly)
	}
	return nil
}

// specProgram writes the program that writes a Go project's specs, and returns its package: the contract's API
// (api.API in the project's module) given to charter's specs package, which first stops on what the specs don't show
// (specs.Problems), then writes both specs or, with -check, compares them. It is the same in every project, so charter
// writes it, into build/ (charter's); go vet and go test leave out a folder whose name starts with _.
func specProgram(root, module string) (string, error) {
	dir := filepath.Join(root, "build", "_spec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return "./build/_spec", os.WriteFile(filepath.Join(dir, "main.go"), []byte(`// Written by charter spec, each time it runs: don't edit. The program that writes the API's
// specs (go run ./build/_spec [-check] <openapi.json> <asyncapi.json> <server>).
package main

import (
	"`+module+`/api"
	"github.com/joeblew999/charter/specs"
)

func main() {
	spec := api.API(api.Env{})
	specs.Must(specs.Problems(spec))
	specs.Main(spec.OpenAPI, spec.AsyncAPI)
}
`), 0o644)
}

// tsSpecProgram writes the program that writes a TypeScript project's specs, and returns its path: the specs
// api/spec.ts exports (the contract's, with charter-lib/serve's specsOf), each written as JSON.stringify indents it or,
// with -check, compared. Node runs it, and the TypeScript it imports, as it is. It is the same in every project, so
// charter writes it, into build/ (charter's).
func tsSpecProgram(root string) (string, error) {
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		return "", err
	}
	return "build/_spec.mjs", os.WriteFile(filepath.Join(root, "build", "_spec.mjs"), []byte(`// Written by charter spec, each time it runs: don't edit. The program that writes the API's
// specs (node build/_spec.mjs [-check] <openapi.json> <asyncapi.json> <server>).
import { readFileSync, writeFileSync } from "node:fs";
import { specs } from "../api/spec.ts";

const args = process.argv.slice(2);
const check = args[0] === "-check";
const [openapi, asyncapi, server] = check ? args.slice(1) : args;
for (const [file, spec] of [[openapi, await specs.openapi(server)], [asyncapi, await specs.asyncapi(server)]]) {
	const text = JSON.stringify(spec, null, 2) + "\n";
	if (!check) writeFileSync(file, text);
	else if (readFileSync(file, "utf8") !== text) {
		console.error(file + " is stale: write the specs again from the contract (charter spec)");
		process.exit(1);
	}
}
console.log(check ? "specs match the TypeScript contract" : openapi + ", " + asyncapi + " (" + server + ")");
`), 0o644)
}

// fernSettings writes fern/generators.yml for the project's specs; with check, it writes nothing and fails if the one
// there is not that.
func fernSettings(p *project.Project, goModule string, checkOnly bool) error {
	if p.SDKs.Name == "" {
		return nil
	}
	spec, err := os.ReadFile(filepath.Join(p.Root, "fern", "openapi.json"))
	if err != nil {
		return err
	}
	text, err := fern.Settings(spec, p.SDKs, goModule, exists(filepath.Join(p.Root, "fern", "overlays.yml")))
	if err != nil {
		return err
	}
	file := filepath.Join(p.Root, "fern", "generators.yml")
	if checkOnly {
		if b, err := os.ReadFile(file); err != nil || string(b) != text {
			return errors.New("fern/generators.yml is not what the spec and charter.toml's [sdks] give: charter spec")
		}
		say("fern/generators.yml is what the spec and charter.toml's [sdks] give")
		return nil
	}
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		return err
	}
	say("fern/generators.yml: from the spec and charter.toml's [sdks]")
	return nil
}

// flagsFirst is args with its flags (-x) before the rest, which Go's flag package stops at: charter sdk group -if-stale
// is charter sdk -if-stale group. sdk's flags take no values, so none is taken from its place.
func flagsFirst(args []string) []string {
	var flags, rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			rest = append(rest, a)
		}
	}
	return append(flags, rest...)
}

// SDK runs charter sdk: an SDK with Fern (Docker) into sdk/out, by default the TypeScript one; -prove: every client,
// proven in its language; -publish: the Go SDK into sdk/go, which the project commits.
func SDK(args []string) {
	fs := flag.NewFlagSet("charter sdk", flag.ExitOnError)
	prove := fs.Bool("prove", false, "generate every client (or those named) and prove each in its language")
	publish := fs.Bool("publish", false, "the Go SDK into sdk/go, proven ([sdks] go = true)")
	ifStale := fs.Bool("if-stale", false, "only when fern/ is not what the SDK was made from")
	fs.Parse(flagsFirst(args))
	p := load()
	switch {
	case *publish:
		if !p.SDKs.Go {
			check(errors.New("sdk -publish needs [sdks] go = true in charter.toml: a project that commits its Go SDK"))
		}
		check(fern.Publish(p.Root, false))
	case *prove:
		check(fern.Prove(p.Root, fs.Args(), p.SDKs.Go))
	default:
		group := "typescript-dist"
		if fs.NArg() > 0 {
			group = fs.Arg(0)
		}
		_, err := fern.Generate(p.Root, group, *ifStale, false)
		check(err)
	}
}
