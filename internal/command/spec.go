package command

import (
	"errors"
	"flag"
	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/fern"
	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
)

// Spec runs charter check --fix: both specs from the contract (fern/openapi.json, fern/asyncapi.json), Fern's settings
// (fern/generators.yml) from them and [pack], and the database's code from migrations/ (db = "sqlc"). With -check it
// writes nothing, and fails where what is there is not what the contract gives.
func Spec(args []string) {
	fs := flag.NewFlagSet("charter check --fix", flag.ExitOnError)
	checkOnly := fs.Bool("check", false, "write nothing; fail where a file is not what the contract gives")
	fs.Parse(args)
	check(spec(load(), *checkOnly))
}

// spec writes (or with check, compares) what the contract gives, in the app's root, from its api.API.
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
	if module == "" {
		return errors.New("charter check --fix writes the specs from the contract, api.API in the app's Go module, and there is no go.mod here")
	}
	program, err := specProgram(p.Root, module)
	if err != nil {
		return err
	}
	command := []string{"go", "run", program}
	if checkOnly {
		command = append(command, "-check")
	}
	// The database's code first: it is written from migrations/ alone, and the contract's package, which the specs'
	// program compiles, calls it (a Go project's api/queries.go builds off Workers too).
	if p.DB != "" {
		if err := dbcode.Write(p.Root, p.DB, checkOnly); err != nil {
			return err
		}
	}
	if err := run.Run(p.Root, nil, command[0], append(command[1:], "fern/openapi.json", "fern/asyncapi.json", server)...); err != nil {
		return err
	}
	return fernSettings(p, module+"/sdk/go", checkOnly)
}

// specProgram writes the program that writes a Go project's specs, and returns its package: the contract's API
// (api.API in the project's module) given to charter's specs package, which first stops on what the specs don't show
// (api.Problems), then writes both specs or, with -check, compares them. It is the same in every project, so charter
// writes it, into .charter/ (charter's scratch folder, which no build empties); go vet and go test leave out a
// folder whose name starts with a dot.
func specProgram(root, module string) (string, error) {
	dir := filepath.Join(root, ".charter", "spec")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return "./.charter/spec", os.WriteFile(filepath.Join(dir, "main.go"), []byte(`// Written by charter check --fix, each time it runs: don't edit. The program that writes the API's
// specs (go run ./.charter/spec [-check] <openapi.json> <asyncapi.json> <server>).
package main

import (
	"`+module+`/api"
	charter "github.com/joeblew999/charter/api"
)

func main() {
	spec := api.API(api.Env{})
	charter.Must(charter.Problems(spec))
	charter.SpecsMain(spec.OpenAPI, spec.AsyncAPI)
}

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
			return errors.New("fern/generators.yml is not what the spec and charter.toml's [pack] give: charter check --fix")
		}
		say("fern/generators.yml is what the spec and charter.toml's [pack] give")
		return nil
	}
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		return err
	}
	say("fern/generators.yml: from the spec and charter.toml's [pack]")
	return nil
}
