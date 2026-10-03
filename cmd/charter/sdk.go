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
	"runtime"
	"slices"
	"strings"
)

// A project's Fern folder is fern/: fern.config.json, generators.yml and the two generated specs.
// Each group in generators.yml is one SDK (or the CLI), generated into sdk/out/<group>.
//
// A project ships a CLI if and only if generators.yml has a group named cli: that group is the one
// switch. Without it nothing builds, ships or checks a CLI, and the project pins no Rust, zig or
// cargo-zigbuild (cliPinsAgree holds mise.toml to the switch).

func init() {
	commands["sdk-gen"] = command{"<group>", "generate one SDK with Fern (Docker) into sdk/out/<group>", sdkGen}
	commands["sdk-check"] = command{"<group>", "prove a generated SDK works (sdk/out/<group>): Go = build, vet, tests against WireMock; TypeScript = typecheck", sdkCheck}
	commands["sdk-ready"] = command{"[group...]", "generate and build what the tests use, if missing: TypeScript and Go SDKs, the Fern CLI if the project has one (or only the groups named)", sdkReady}
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

// sdkStamp is where sdk-gen notes what a group was generated from (beside sdk/out/<group>, which
// Fern replaces whole).
func sdkStamp(group string) string { return filepath.Join("sdk/out", "."+group+sdkMadeFrom) }

// sdkFresh reports whether sdk/out/<group> was generated from the Fern folder as it is now.
func sdkFresh(group string) bool {
	made, err := os.ReadFile(sdkStamp(group))
	source, err2 := sdkSourceHash()
	return err == nil && err2 == nil && strings.TrimSpace(string(made)) == source
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
		// project's node_modules. Fern's SDKs are written for TypeScript 5: a project whose own code
		// is on a newer one installs 5.9 beside it as typescript-sdk ("npm:typescript@~5.9.3").
		config := filepath.Join(filepath.Dir(entry), "tsconfig.check.json")
		if err := os.WriteFile(config, fmt.Appendf(nil, sdkTSConfig, filepath.Base(entry)), 0o644); err != nil {
			return err
		}
		compiler := []string{npmBin("tsc")}
		if exists(tscForSDKs) {
			compiler = []string{"node", tscForSDKs}
		}
		if err := sh(".", compiler[0], append(compiler[1:], "-p", config)...); err != nil {
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
	if slices.Contains(groups, "cli") && !hasCLI() {
		return errNoCLI
	}
	// By default, the CLI too when the project has one.
	wanted := func(group string) bool {
		return (len(groups) == 0 && (group != "cli" || hasCLI())) || slices.Contains(groups, group)
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
		// Missing, or generated from other specs or settings: generate it (again).
		if wanted(need.group) && (!exists(filepath.Join("sdk/out", need.file)) || !sdkFresh(need.group)) {
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
	// The binary is older than its sources when they were generated again.
	if built, err := os.Stat(filepath.Join(cli, "target/release", exe(bin))); err != nil || olderThan(built, sdkStamp("cli")) {
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
// repo's examples to it). A project with a CLI has the group cli after them.
var sdkCommon = []string{"go", "typescript", "typescript-dist"}

// errNoCLI is what the CLI's commands say in a project without one.
var errNoCLI = errors.New("this project has no CLI (no cli group in fern/generators.yml): " + docsURL + "guides/sdks.html#add-the-cli")

// hasCLI reports whether the project ships a CLI: its generators.yml has the group cli.
func hasCLI() bool {
	groups, _ := sdkGroups(generators)
	return slices.Contains(groups, "cli")
}

// The tools that only the CLI needs, as a project's mise.toml pins them: Rust builds it, zig and
// cargo-zigbuild link it for the other systems. new -cli copies these lines from the notes example.
var cliPin = regexp.MustCompile(`(?m)^(rust|zig|"aqua:rust-cross/cargo-zigbuild") = "[^"]*"$`)

// cliPinsAgree fails unless the mise.toml in dir pins the CLI's tools exactly when generators.yml
// in dir has the cli group: the group is the switch, and the pins follow it.
func cliPinsAgree(dir string) error {
	tasks, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		return err
	}
	groups, err := sdkGroups(filepath.Join(dir, generators))
	if err != nil {
		return err
	}
	pins := len(cliPin.FindAll(tasks, -1))
	switch cli := slices.Contains(groups, "cli"); {
	case cli && pins != 3:
		return errors.New("fern/generators.yml has the cli group, but mise.toml does not pin rust, zig and \"aqua:rust-cross/cargo-zigbuild\" (the notes example's lines)")
	case !cli && pins != 0:
		return errors.New("mise.toml pins the CLI's tools (rust, zig, cargo-zigbuild), but fern/generators.yml has no cli group: remove them")
	}
	return nil
}

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

// exe is the file name of a program this system built: on Windows it ends in .exe.
func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
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
	if !hasCLI() {
		return errNoCLI
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
		fmt.Printf("built: %s/target/release/%s\n", dir, exe(bin))
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

// olderThan reports whether info was modified before file (false if file is missing).
func olderThan(info os.FileInfo, file string) bool {
	other, err := os.Stat(file)
	return err == nil && info.ModTime().Before(other.ModTime())
}
