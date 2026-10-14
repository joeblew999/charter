package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// charter doctor: check what the project's tasks need: npm installs, Docker, Go, Rust (with a CLI), leftover containers
// Run by: mise run doctor (tasks/repo/tasks.toml)
func init() {
	commands["doctor"] = command{"", "check what the project's tasks need: npm installs, Docker, Go, Rust (with a CLI), leftover containers", doctor}
}

func doctor([]string) error {
	failed := false
	ok := func(format string, a ...any) { fmt.Printf("  ok    "+format+"\n", a...) }
	warn := func(format string, a ...any) { fmt.Printf("  WARN  "+format+"\n", a...) }
	bad := func(format string, a ...any) { fmt.Printf("  FAIL  "+format+"\n", a...); failed = true }
	if exists("node_modules") {
		ok("npm packages installed")
	} else {
		bad("no npm packages: mise run setup")
	}
	if err := docker(); err == nil {
		ok("docker running (Fern generates in containers)")
	} else {
		bad(err.Error())
	}
	tools := []struct{ name, arg, why string }{
		{"go", "version", "the tool itself, a Go Worker, sdk:check on Go SDKs"},
		{"gh", "--version", "upstream:status, adopt, release"},
	}
	// Rust and zig only for a project with a CLI.
	if hasCLI() {
		tools = append(tools,
			struct{ name, arg, why string }{"cargo", "--version", "the Fern CLI builds"},
			struct{ name, arg, why string }{"cargo-zigbuild", "--version", "the CLI for Linux and Windows, sdk:dist:cli"})
	} else {
		ok("no CLI (no cli group in fern/generators.yml): no Rust needed")
	}
	if err := cliPinsAgree("."); err != nil {
		warn("%v", err)
	}
	for _, tool := range tools {
		if out, err := exec.Command(tool.name, tool.arg).Output(); err == nil {
			ok("%s (%s)", strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0], tool.why)
		} else {
			warn("no %s: mise install (%s)", tool.name, tool.why)
		}
	}
	if os.Getenv("FERN_TOKEN") != "" {
		ok("FERN_TOKEN set")
	} else {
		warn("FERN_TOKEN not set: Fern documents local generation as an Enterprise feature needing it (it ran without one on 2026-10-01)")
	}
	if ids, _ := output(".", "docker", "ps", "-q", "--filter", wiremock); ids == "" {
		ok("no leftover WireMock containers")
	} else {
		warn("%d WireMock container(s) running: mise run sdk:clean", len(strings.Fields(ids)))
	}
	if failed {
		return errors.New("not ready")
	}
	fmt.Print("SDK groups: ")
	return sdkList(nil)
}
