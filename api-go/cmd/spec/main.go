// Command spec writes the API's OpenAPI and AsyncAPI specs from the Go contract, offline (no Worker
// needed), for Fern:
//
//	go run ./cmd/spec [-check] <openapi.json> <asyncapi.json> [server-url]
//
// With -check it writes nothing and fails if either file differs from what the contract gives:
// someone changed the contract and didn't run `mise run api-go:spec`. The same functions the Worker serves /api/openapi.json and /api/asyncapi.json with (api/spec.go).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/joeblew999/orpc-api/api-go/api"
)

func main() {
	check := flag.Bool("check", false, "write nothing; fail if a file differs from the contract")
	flag.Parse()
	files := flag.Args()
	if len(files) < 2 {
		fmt.Fprintln(os.Stderr, "usage: spec [-check] <openapi.json> <asyncapi.json> [server-url]")
		os.Exit(2)
	}
	server := "https://api.example.com"
	if len(files) > 2 {
		server = files[2]
	}
	for i, spec := range []func(string) ([]byte, error){api.OpenAPI, api.AsyncAPI} {
		compact, err := spec(server)
		var pretty bytes.Buffer
		if err == nil {
			err = json.Indent(&pretty, compact, "", "  ")
			pretty.WriteByte('\n')
		}
		if err == nil && *check {
			if committed, _ := os.ReadFile(files[i]); !bytes.Equal(committed, pretty.Bytes()) {
				err = fmt.Errorf("%s is stale: mise run api-go:spec", files[i])
			}
		} else if err == nil {
			err = os.WriteFile(files[i], pretty.Bytes(), 0o644)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *check {
		fmt.Println("specs match the Go contract")
		return
	}
	fmt.Printf("%s, %s (%s)\n", files[0], files[1], server)
}
