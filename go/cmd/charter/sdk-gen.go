package main

import (
	"errors"
	"os"
)

// charter sdk:gen: generate one SDK with Fern (Docker) into sdk/out/<group>
// Run by: mise run sdk:gen (tasks/repo/tasks.toml)
func init() {
	commands["sdk:gen"] = command{"<group>", "generate one SDK with Fern (Docker) into sdk/out/<group>", sdkGen}
}

func sdkGen(args []string) error {
	if len(args) != 1 {
		return errors.New("sdk:gen needs <group>: a group in " + generators + " (charter sdk:list)")
	}
	if args[0] == "cli" && !hasCLI() {
		return errNoCLI
	}
	if err := docker(); err != nil {
		return err
	}
	source, err := sdkSourceHash()
	if err != nil {
		return err
	}
	if err := sh(".", npmBin("fern"), "generate", "--local", "--group", args[0], "--force", "--log-level", "warn"); err != nil {
		return err
	}
	// What it was made from, so sdk-ready knows when it is stale.
	return os.WriteFile(sdkStamp(args[0]), []byte(source+"\n"), 0o644)
}
