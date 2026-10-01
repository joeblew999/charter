// Package specfile is the body of a spec command: it writes the specs a contract generates to
// files, for Fern, or checks that the committed files are what the contract gives. One command per
// contract calls Main with the contract's spec functions (cmd/spec, cmd/showcase-spec).
package specfile

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

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
		return fmt.Errorf("%s is stale: write the specs again from the contract (the :spec task)", file)
	}
	return nil
}
