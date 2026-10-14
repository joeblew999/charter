package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// charter dist-cli: HEAVY: generate the project's Fern CLI and build it into dist/ for darwin, linux and windows on amd64 and arm64: every target this machine builds (a Mac builds all six)
// Run by: mise run sdk:dist:cli (tasks/repo/tasks.toml)
func init() {
	commands["dist-cli"] = command{"[-target <os>-<arch>,...]", "HEAVY: generate the project's Fern CLI and build it into dist/ for darwin, linux and windows on amd64 and arm64: every target this machine builds (a Mac builds all six)", distCLI}
}

var cliTargets = []cliTarget{
	{"darwin", "amd64", "x86_64-apple-darwin"},
	{"darwin", "arm64", "aarch64-apple-darwin"},
	{"linux", "amd64", "x86_64-unknown-linux-musl"},
	{"linux", "arm64", "aarch64-unknown-linux-musl"},
	{"windows", "amd64", "x86_64-pc-windows-gnu"},
	{"windows", "arm64", "aarch64-pc-windows-gnullvm"},
}

// cliTargetsFor are the targets to build on a machine running host: those named (-target), or
// every one it can build. macOS's are built with Apple's linker, so only on a Mac; the others with
// zig (cargo-zigbuild), on any machine. skipped are the ones it can't.
func cliTargetsFor(host, only string) (build, skipped []cliTarget, err error) {
	can := func(t cliTarget) bool { return t.os != "darwin" || host == "darwin" }
	if only == "" {
		for _, t := range cliTargets {
			if can(t) {
				build = append(build, t)
			} else {
				skipped = append(skipped, t)
			}
		}
		return build, skipped, nil
	}
	for _, name := range strings.Split(only, ",") {
		at := slices.IndexFunc(cliTargets, func(t cliTarget) bool { return t.String() == strings.TrimSpace(name) })
		switch {
		case at < 0:
			return nil, nil, fmt.Errorf("no CLI target %q: the targets are %v", name, cliTargets)
		case !can(cliTargets[at]):
			return nil, nil, fmt.Errorf("%s builds only on a Mac (Apple's linker)", name)
		}
		build = append(build, cliTargets[at])
	}
	return build, nil, nil
}

func distCLI(args []string) error {
	var only string
	rest := flags("dist-cli", args, func(f *flag.FlagSet) {
		f.StringVar(&only, "target", "", "only these, comma-separated: darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64, windows-arm64 (default: every one this machine builds)")
	})
	if len(rest) != 0 {
		return errors.New("dist-cli takes no arguments: it builds the project's cli group")
	}
	if !hasCLI() {
		return errNoCLI
	}
	targets, skipped, err := cliTargetsFor(runtime.GOOS, only)
	if err != nil {
		return err
	}
	name, err := distName()
	if err != nil {
		return err
	}
	if err := distDir(); err != nil {
		return err
	}
	dir := sdkOut("cli")
	if err := sdkGen([]string{"cli"}); err != nil {
		return err
	}
	bin, err := cargoBin(dir)
	if err != nil {
		return err
	}
	for _, t := range targets {
		start := time.Now()
		if err := quiet(".", nil, "rustup", "target", "add", t.rust); err != nil {
			return fmt.Errorf("rustup target add %s: the project's mise.toml pins Rust (mise install)", t.rust)
		}
		program, build := "cargo", []string{"build", "--release", "--bin", bin, "--no-default-features", "--features", "rustls", "--target", t.rust}
		if t.os != "darwin" {
			if _, look := exec.LookPath("cargo-zigbuild"); look != nil {
				return errors.New("no cargo-zigbuild: the project's mise.toml pins it and zig (mise install)")
			}
			program, build[0] = "cargo-zigbuild", "zigbuild"
		}
		cargo := exec.Command(program, build...)
		cargo.Dir, cargo.Stdout, cargo.Stderr = dir, os.Stdout, os.Stderr
		if err := cargo.Run(); err != nil {
			return fmt.Errorf("the CLI for %s (%s): %w", t, t.rust, err)
		}
		built := bin
		if t.os == "windows" {
			built += ".exe"
		}
		content, err := os.ReadFile(filepath.Join(dir, "target", t.rust, "release", built))
		if err != nil {
			return err
		}
		out := filepath.Join(dist, cliFile(name, t))
		if err := os.WriteFile(out, content, 0o755); err != nil {
			return err
		}
		// About 450 MB a target, and no use to the next build: the crate Fern writes again compiles
		// every dependency again.
		if err := os.RemoveAll(filepath.Join(dir, "target", t.rust)); err != nil {
			return err
		}
		fmt.Printf("built: %s (%s, %s)\n", out, t.rust, time.Since(start).Round(time.Second))
	}
	for _, t := range skipped {
		fmt.Printf("skipped: %s (it builds only on a Mac)\n", t)
	}
	// The one this machine runs, run: a build that does not start fails here, not on a user's machine.
	if exists(filepath.Join(dist, cliFile(name, cliTarget{os: runtime.GOOS, arch: runtime.GOARCH}))) {
		return cliSmoke(nil)
	}
	return nil
}
