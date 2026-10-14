package main

import (
	"errors"
	"flag"
	"fmt"
)

// charter cli-build: HEAVY: build the generated Rust CLI (sdk/out/cli), natively or for Linux in Docker
// Run by: mise run sdk:cli:build (tasks/repo/tasks.toml)
func init() {
	commands["cli-build"] = command{"[-linux]", "HEAVY: build the generated Rust CLI (sdk/out/cli), natively or for Linux in Docker", cliBuild}
}

func cliBuild(args []string) error {
	var linux bool
	rest := flags("cli-build", args, func(f *flag.FlagSet) {
		f.BoolVar(&linux, "linux", false, "build for Linux inside Docker (the container's arch)")
	})
	if len(rest) != 0 {
		return errors.New("cli-build takes no arguments: it builds sdk/out/cli")
	}
	if !hasCLI() {
		return errNoCLI
	}
	dir := sdkOut("cli")
	if !exists(dir) {
		return fmt.Errorf("no %s: mise run sdk:gen cli", dir)
	}
	return cliBuildDir(dir, linux)
}
