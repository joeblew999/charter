package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// charter dist: build everything the project ships into an emptied dist/: its SDKs and specs (dist-sdk), then its CLI if it has one (dist-cli)
// Run by: mise run dist (tasks/repo/tasks.toml)
func init() {
	commands["dist"] = command{"[-target <os>-<arch>,...]", "build everything the project ships into an emptied dist/: its SDKs and specs (dist-sdk), then its CLI if it has one (dist-cli)", distAll}
}

// distAll builds what the project ships into an emptied dist/, so nothing left from an older build
// goes onto a release: the SDKs and the specs, then the CLI if the project has one.
func distAll(args []string) error {
	var only string
	rest := flags("dist", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "target", "", "only the CLI for these targets, as dist-cli takes them")
	})
	if len(rest) != 0 {
		return errors.New("dist takes no arguments: it builds what the project ships")
	}
	if only != "" && !hasCLI() {
		return fmt.Errorf("dist -target picks the CLI's targets: %w", errNoCLI)
	}
	if err := os.RemoveAll(dist); err != nil {
		return err
	}
	if err := distSDK(nil); err != nil {
		return err
	}
	if !hasCLI() {
		fmt.Println("no CLI: the project has no cli group in fern/generators.yml, so dist/ holds the SDKs and the specs")
		return nil
	}
	if only == "" {
		return distCLI(nil)
	}
	return distCLI([]string{"-target", only})
}
