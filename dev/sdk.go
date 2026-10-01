package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	commands["sdk-gen"] = command{"<api> <group>", "generate one SDK with Fern (Docker) into sdk/out/<api>/<group>", sdkGen}
	commands["sdk-check"] = command{"<dir>", "prove a generated SDK works: Go = build, vet, tests against WireMock; TypeScript = typecheck", sdkCheck}
	commands["sdk-ready"] = command{"<api>", "generate and build what the tests use, if missing: TypeScript and Go SDKs, the Fern CLI", sdkReady}
	commands["sdk-list"] = command{"", "the APIs in sdk/fern/apis and the SDK groups each one defines", sdkList}
	commands["sdk-clean"] = command{"", "remove generated SDKs (sdk/out) and stop leftover WireMock containers", sdkClean}
	commands["cli-build"] = command{"[-linux] <dir>", "HEAVY: build a Fern-generated Rust CLI, natively or for Linux in Docker", cliBuild}
}

const wiremock = "ancestor=wiremock/wiremock:3.9.1"

func docker() error {
	if exec.Command("docker", "info").Run() != nil {
		return errors.New("Docker is not running: start it first")
	}
	return nil
}

func sdkGen(args []string) error {
	if len(args) != 2 {
		return errors.New("sdk-gen needs <api> <group>: a folder in sdk/fern/apis and a group in its generators.yml")
	}
	if err := docker(); err != nil {
		return err
	}
	return sh("sdk", fern, "generate", "--local", "--api", args[0], "--group", args[1], "--force", "--log-level", "warn")
}

func sdkCheck(args []string) error {
	if len(args) != 1 {
		return errors.New("sdk-check needs <dir>, e.g. sdk/out/petstore/go")
	}
	dir := args[0]
	if exists(filepath.Join(dir, "go.mod")) {
		// Generated modules aren't in go.work: build them on their own.
		off := []string{"GOWORK=off"}
		if err := quiet(dir, off, "go", "build", "./..."); err != nil {
			return err
		}
		fmt.Println("build ok")
		if err := quiet(dir, off, "go", "vet", "./..."); err != nil {
			return err
		}
		fmt.Println("vet ok")
		if compose := filepath.Join(dir, "wiremock", "docker-compose.test.yml"); exists(compose) {
			if err := quiet(".", nil, "docker", "compose", "-f", compose, "up", "-d", "--wait"); err != nil {
				return err
			}
			defer quiet(".", nil, "docker", "compose", "-f", compose, "down")
			// WireMock gets a random host port: tell the tests where it is.
			port, err := output(".", "docker", "compose", "-f", compose, "port", "wiremock", "8080")
			if err != nil {
				return err
			}
			off = append(off, "WIREMOCK_URL=http://"+port)
		}
		if err := quiet(dir, off, "go", "test", "./..."); err != nil {
			return err
		}
		fmt.Println("tests ok")
		return nil
	}
	for _, entry := range []string{"index.ts", "src/index.ts", "sdk/index.ts"} {
		entry = filepath.Join(dir, entry)
		if !exists(entry) {
			continue
		}
		// A tsconfig beside the SDK that extends sdk/tsconfig.base.json (standard fetch, no Node types).
		base, _ := filepath.Abs("sdk/tsconfig.base.json")
		config := filepath.Join(filepath.Dir(entry), "tsconfig.check.json")
		if err := os.WriteFile(config, fmt.Appendf(nil, "{ \"extends\": %q, \"include\": [%q] }\n", base, filepath.Base(entry)), 0o644); err != nil {
			return err
		}
		if err := sh(".", "sdk/node_modules/.bin/tsc", "-p", config); err != nil {
			return err
		}
		fmt.Printf("typecheck ok (%s)\n", entry)
		return nil
	}
	return fmt.Errorf("%s: no go.mod or index.ts", dir)
}

func sdkReady(args []string) error {
	if len(args) != 1 {
		return errors.New("sdk-ready needs <api>: a folder in sdk/fern/apis")
	}
	api, out := args[0], filepath.Join("sdk/out", args[0])
	for _, need := range []struct{ file, group string }{
		{"typescript-dist/esm/index.mjs", "typescript-dist"},
		{"go/go.mod", "go"},
		{"cli/Cargo.toml", "cli"},
	} {
		if !exists(filepath.Join(out, need.file)) {
			if err := sdkGen([]string{api, need.group}); err != nil {
				return err
			}
		}
	}
	cli := filepath.Join(out, "cli")
	bin, err := cargoBin(cli)
	if err != nil {
		return err
	}
	if !exists(filepath.Join(cli, "target/release", bin)) {
		return cliBuild([]string{cli})
	}
	return nil
}

func sdkList([]string) error {
	apis, err := filepath.Glob("sdk/fern/apis/*/generators.yml")
	if err != nil {
		return err
	}
	group := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):`)
	for _, file := range apis {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		var groups []string
		in := false
		for scanner := bufio.NewScanner(f); scanner.Scan(); {
			if strings.HasPrefix(scanner.Text(), "groups:") {
				in = true
			} else if m := group.FindStringSubmatch(scanner.Text()); in && m != nil {
				groups = append(groups, m[1])
			}
		}
		f.Close()
		fmt.Printf("%s: %s\n", filepath.Base(filepath.Dir(file)), strings.Join(groups, " "))
	}
	return nil
}

func sdkClean([]string) error {
	for _, dir := range []string{"sdk/out", "sdk/.work"} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if ids, _ := output(".", "docker", "ps", "-q", "--filter", wiremock); ids != "" {
		if err := quiet(".", nil, "docker", append([]string{"rm", "-f"}, strings.Fields(ids)...)...); err != nil {
			return err
		}
		fmt.Println("stopped WireMock containers")
	}
	fmt.Println("clean")
	return nil
}

// cargoBin is the name of the first [[bin]] in a generated CLI's Cargo.toml.
func cargoBin(dir string) (string, error) {
	manifest, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return "", err
	}
	_, bins, ok := strings.Cut(string(manifest), "[[bin]]")
	if m := regexp.MustCompile(`(?m)^name\s*=\s*"([^"]+)"`).FindStringSubmatch(bins); ok && m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("%s/Cargo.toml has no [[bin]] name", dir)
}

// cliBuild compiles every Rust dependency the first time: minutes at full CPU.
func cliBuild(args []string) error {
	var linux bool
	rest := flags("cli-build", args, func(f *flag.FlagSet) {
		f.BoolVar(&linux, "linux", false, "build for Linux inside Docker (the container's arch)")
	})
	if len(rest) != 1 {
		return errors.New("cli-build needs <dir>, e.g. sdk/out/api/cli")
	}
	dir := rest[0]
	bin, err := cargoBin(dir)
	if err != nil {
		return err
	}
	build := []string{"build", "--release", "--bin", bin, "--no-default-features", "--features", "rustls"}
	if !linux {
		if err := sh(dir, "cargo", build...); err != nil {
			return err
		}
		fmt.Printf("built: %s/target/release/%s\n", dir, bin)
		return nil
	}
	if err := docker(); err != nil {
		return err
	}
	toolchain, err := os.ReadFile("rust-toolchain.toml")
	if err != nil {
		return err
	}
	channel := regexp.MustCompile(`(?m)^channel\s*=\s*"([^"]+)"`).FindStringSubmatch(string(toolchain))
	if channel == nil {
		return errors.New("rust-toolchain.toml has no channel")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Named volumes cache crates between runs; target-linux keeps it apart from a native build.
	run := append([]string{"run", "--rm", "-v", abs + ":/src", "-w", "/src", "-v", "orpc-api-cargo-registry:/usr/local/cargo/registry",
		"-e", "CARGO_TARGET_DIR=/src/target-linux", "rust:" + channel[1], "cargo"}, build...)
	if err := sh(".", "docker", run...); err != nil {
		return err
	}
	fmt.Printf("built: %s/target-linux/release/%s (Linux)\n", dir, bin)
	return nil
}
