// Package specs is the body of a spec command: it writes the specs a contract generates to
// files, for Fern, or checks that the committed files are what the contract gives. charter spec
// writes that command for a Go project (build/_spec) and runs it: it calls Main with the contract's
// spec functions, after Must(Problems(api)), what else is wrong with the contract that no spec shows.
package specs

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/joeblew999/charter/humaworkers"
	"github.com/joeblew999/charter/serve"
)

// Problems is what is wrong with a contract that no spec shows: a route that does not name its own
// operation (a tool call finds its operation by that name), an operation that cannot be its MCP tool,
// and a constrained request field without a valid example (Fern shows an example of every request,
// and makes one up where the contract has none: whoever copies it gets a 422).
func Problems(api serve.API) []error {
	var problems []error
	for _, route := range api.Routes {
		ops := humaworkers.New(api.Config(), []humaworkers.Route{route}).Operations()
		if len(ops) != 1 || route.OperationID == "" || ops[0].OperationID != route.OperationID || ops[0].Method != route.Method || ops[0].Path != route.Path {
			problems = append(problems, fmt.Errorf("route %s %s (%q) does not register the one operation it names", route.Method, route.Path, route.OperationID))
		}
	}
	problems = append(problems, api.Tools()...)
	return append(problems, exampleProblems(humaworkers.New(api.Config(), api.Routes))...)
}

// Must prints the problems and ends the command if there are any: a contract with problems gets no
// specs, written or checked.
func Must(problems []error) {
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, problem)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// Main runs the command:
//
//	spec [-check] <file per spec>... [server-url]
//
// Each function gives one spec for a server URL, as compact JSON; it is written indented. With
// -check nothing is written, and the command fails if a file differs from what its function gives:
// someone changed the contract and didn't write the specs again.
func Main(specs ...func(server string) ([]byte, error)) {
	check := flag.Bool("check", false, "write nothing; fail if a file differs from the contract")
	flag.Parse()
	files := flag.Args()
	if len(files) < len(specs) {
		fmt.Fprintf(os.Stderr, "usage: %s [-check] <%d spec files> [server-url]\n", os.Args[0], len(specs))
		os.Exit(2)
	}
	server := "https://api.example.com"
	if len(files) > len(specs) {
		server = files[len(specs)]
	}
	for i, spec := range specs {
		if err := one(spec, files[i], server, *check); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *check {
		fmt.Println("specs match the Go contract")
		return
	}
	fmt.Printf("%s (%s)\n", strings.Join(files[:len(specs)], ", "), server)
}

func one(spec func(string) ([]byte, error), file, server string, check bool) error {
	compact, err := spec(server)
	if err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact, "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	if !check {
		return os.WriteFile(file, pretty.Bytes(), 0o644)
	}
	if committed, _ := os.ReadFile(file); !bytes.Equal(committed, pretty.Bytes()) {
		return fmt.Errorf("%s is stale: write the specs again from the contract (charter spec)", file)
	}
	return nil
}
