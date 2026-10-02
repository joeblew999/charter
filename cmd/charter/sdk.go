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
	"slices"
	"strings"
)

// A project's Fern folder is fern/: fern.config.json, generators.yml and the two generated specs.
// Each group in generators.yml is one SDK (or the CLI), generated into sdk/out/<group>.

func init() {
	commands["sdk-gen"] = command{"<group>", "generate one SDK with Fern (Docker) into sdk/out/<group>", sdkGen}
	commands["sdk-check"] = command{"<group>", "prove a generated SDK works (sdk/out/<group>): Go = build, vet, tests against WireMock; TypeScript = typecheck", sdkCheck}
	commands["sdk-ready"] = command{"[group...]", "generate and build what the tests use, if missing: TypeScript and Go SDKs, the Fern CLI (or only the groups named)", sdkReady}
	commands["sdk-list"] = command{"", "the SDK groups the project defines (fern/generators.yml)", sdkList}
	commands["sdk-clean"] = command{"", "remove generated SDKs (sdk/out) and stop leftover WireMock containers", sdkClean}
	commands["cli-build"] = command{"[-linux]", "HEAVY: build the generated Rust CLI (sdk/out/cli), natively or for Linux in Docker", cliBuild}
}

const (
	fernDir    = "fern"
	generators = fernDir + "/generators.yml"
	wiremock   = "ancestor=wiremock/wiremock:3.9.1"
)

func docker() error {
	if exec.Command("docker", "info").Run() != nil {
		return errors.New("Docker is not running: start it first")
	}
	return nil
}

func sdkGen(args []string) error {
	if len(args) != 1 {
		return errors.New("sdk-gen needs <group>: a group in " + generators + " (charter sdk-list)")
	}
	if err := docker(); err != nil {
		return err
	}
	return sh(".", fern, "generate", "--local", "--group", args[0], "--force", "--log-level", "warn")
}

// sdkOut is where Fern puts a group: the output path generators.yml names.
func sdkOut(group string) string {
	return filepath.Join("sdk/out", group)
}

func sdkCheck(args []string) error {
	if len(args) != 1 {
		return errors.New("sdk-check needs <group>, as sdk-gen does: it checks sdk/out/<group>")
	}
	dir := sdkOut(args[0])
	if !exists(dir) {
		return fmt.Errorf("no %s: mise run sdk:gen %s", dir, args[0])
	}
	return sdkCheckDir(dir)
}

// How a generated TypeScript SDK is typechecked: standard fetch, no settings of the project's own.
const sdkTSConfig = `{
  "//": "Written by charter sdk-check: how a generated TypeScript SDK is typechecked (bundler mode, default libs).",
  "compilerOptions": {
    "target": "es2022",
    "module": "preserve",
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "strict": true,
    "noEmit": true,
    "skipLibCheck": true
  },
  "include": [%q]
}
`

func sdkCheckDir(dir string) error {
	if exists(filepath.Join(dir, "go.mod")) {
		// A generated module is its own: no workspace file above it applies.
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
		// With the project's TypeScript (package.json), which resolves the SDK's imports from the
		// project's node_modules.
		config := filepath.Join(filepath.Dir(entry), "tsconfig.check.json")
		if err := os.WriteFile(config, fmt.Appendf(nil, sdkTSConfig, filepath.Base(entry)), 0o644); err != nil {
			return err
		}
		if err := sh(".", tsc, "-p", config); err != nil {
			return err
		}
		fmt.Printf("typecheck ok (%s)\n", entry)
		return nil
	}
	if entries, _ := os.ReadDir(dir); len(entries) == 0 {
		return fmt.Errorf("%s: nothing generated", dir)
	}
	// Another language: Fern generated it, and nothing here builds it.
	fmt.Printf("generated (%s): this tool builds and tests Go and TypeScript SDKs only\n", dir)
	return nil
}

func sdkReady(groups []string) error {
	wanted := func(group string) bool {
		return len(groups) == 0 || slices.Contains(groups, group)
	}
	needs := []struct{ file, group string }{
		{"typescript-dist/esm/index.mjs", "typescript-dist"},
		{"go/go.mod", "go"},
		{"cli/Cargo.toml", "cli"},
	}
	// Any other group named (a project's own, like a showcase's typescript-public) is there when its folder is.
	for _, group := range groups {
		if !slices.ContainsFunc(needs, func(need struct{ file, group string }) bool { return need.group == group }) {
			needs = append(needs, struct{ file, group string }{group, group})
		}
	}
	for _, need := range needs {
		if wanted(need.group) && !exists(filepath.Join("sdk/out", need.file)) {
			if err := sdkGen([]string{need.group}); err != nil {
				return err
			}
		}
	}
	if !wanted("cli") {
		return nil
	}
	cli := sdkOut("cli")
	bin, err := cargoBin(cli)
	if err != nil {
		return err
	}
	if !exists(filepath.Join(cli, "target/release", bin)) {
		return cliBuildDir(cli, false)
	}
	return nil
}

func sdkList([]string) error {
	groups, err := sdkGroups(generators)
	if err != nil {
		return err
	}
	fmt.Println(strings.Join(groups, " "))
	return nil
}

// sdkCommon are the groups every project's generators.yml defines, in this order (a test holds this
// repo's examples to it).
var sdkCommon = []string{"go", "typescript", "typescript-dist", "cli"}

// sdkGroups are the groups a generators.yml defines, in the file's order.
func sdkGroups(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	group := regexp.MustCompile(`^  ([A-Za-z0-9_-]+):`)
	var groups []string
	in := false
	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		if strings.HasPrefix(scanner.Text(), "groups:") {
			in = true
		} else if m := group.FindStringSubmatch(scanner.Text()); in && m != nil {
			groups = append(groups, m[1])
		}
	}
	return groups, nil
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

func cliBuild(args []string) error {
	var linux bool
	rest := flags("cli-build", args, func(f *flag.FlagSet) {
		f.BoolVar(&linux, "linux", false, "build for Linux inside Docker (the container's arch)")
	})
	if len(rest) != 0 {
		return errors.New("cli-build takes no arguments: it builds sdk/out/cli")
	}
	dir := sdkOut("cli")
	if !exists(dir) {
		return fmt.Errorf("no %s: mise run sdk:gen cli", dir)
	}
	return cliBuildDir(dir, linux)
}

var rustVersion = regexp.MustCompile(`^rustc (\d+\.\d+\.\d+)`)

// cliBuildDir compiles every Rust dependency the first time: minutes at full CPU.
func cliBuildDir(dir string, linux bool) error {
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
	// The same Rust as the native build: the one the project's mise.toml pins.
	have, err := output(".", "rustc", "--version")
	channel := rustVersion.FindStringSubmatch(have)
	if err != nil || channel == nil {
		return errors.New("no rustc: the project's mise.toml pins Rust (mise install)")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// Named volumes cache crates between runs; target-linux keeps it apart from a native build.
	run := append([]string{"run", "--rm", "-v", abs + ":/src", "-w", "/src", "-v", "charter-cargo-registry:/usr/local/cargo/registry",
		"-e", "CARGO_TARGET_DIR=/src/target-linux", "rust:" + channel[1], "cargo"}, build...)
	if err := sh(".", "docker", run...); err != nil {
		return err
	}
	fmt.Printf("built: %s/target-linux/release/%s (Linux)\n", dir, bin)
	return nil
}
