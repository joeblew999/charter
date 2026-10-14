package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// charter cli-smoke: run the CLI that dist/ holds for this machine with --version: proves a binary built elsewhere starts here
// Run by: mise run sdk:cli:smoke (tasks/repo/tasks.toml)
func init() {
	commands["cli-smoke"] = command{"", "run the CLI that dist/ holds for this machine with --version: proves a binary built elsewhere starts here", cliSmoke}
}

// cliSmoke runs the CLI in dist/ built for this machine. The release workflow runs it on Windows,
// which no other machine can start a Windows binary on.
func cliSmoke(args []string) error {
	if len(args) != 0 {
		return errors.New("cli-smoke takes no arguments: it runs the CLI in dist/ for this machine")
	}
	if !hasCLI() {
		return errNoCLI
	}
	name, err := distName()
	if err != nil {
		return err
	}
	file := filepath.Join(dist, cliFile(name, cliTarget{os: runtime.GOOS, arch: runtime.GOARCH}))
	if !exists(file) {
		return fmt.Errorf("no %s: mise run sdk:dist:cli builds it (or the release workflow's cli job)", file)
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	if err := os.Chmod(abs, 0o755); err != nil { // a downloaded artifact loses the mode
		return err
	}
	said, err := output(".", abs, "--version")
	if err != nil {
		return fmt.Errorf("%s --version: %w", file, err)
	}
	fmt.Printf("ran: %s --version: %s\n", file, said)
	return nil
}
