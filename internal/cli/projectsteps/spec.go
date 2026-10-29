package projectsteps

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/fern"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/sqlc"
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

// spec writes (or with check, compares) what the contract gives, in the project's root.
func spec(p *project.Project, checkOnly bool) error {
	module := goModule(p.Root)
	if module == "" {
		return errors.New("charter spec writes a Go project's specs: this one has no go.mod")
	}
	server := p.SpecServer
	if server == "" {
		server = p.URL
	}
	if server == "" {
		return errors.New("charter.toml has no url: the specs name the server the Worker is deployed at")
	}
	program, err := specProgram(p.Root, module)
	if err != nil {
		return err
	}
	args := []string{"run", program}
	if checkOnly {
		args = append(args, "-check")
	}
	if err := run.Run(p.Root, nil, "go", append(args, "fern/openapi.json", "fern/asyncapi.json", server)...); err != nil {
		return err
	}
	if err := fernSettings(p, module+"/sdk/go", checkOnly); err != nil {
		return err
	}
	switch p.DB {
	case "":
	case "sqlc":
		return sqlc.Write(p.Root, checkOnly)
	default:
		return fmt.Errorf(`charter.toml: db = %q: a Go project's is "sqlc"`, p.DB)
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
