package fern

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/joeblew999/charter/internal/run"
)

// The toolchains a proof needs, as mise loads them: the Rust a CLI is built with; the uv and Python a Python
// client is proven with, and the packages: the client's own (httpx, pydantic, typing-extensions, anyio) and the
// checkers.
const (
	rustVersion   = "1.98.1"
	uvVersion     = "0.13.0"
	pythonVersion = "3.13"
)

var pythonPackages = []string{"httpx==0.28.1", "pydantic==2.14.1", "typing-extensions==4.16.0", "anyio==4.15.1", "pytest==9.1.1", "mypy==2.4.0"}

// SpecsHash is the hash of what an SDK is made from: every file in dir's fern/.
func SpecsHash(dir string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "fern"))
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "fern", e.Name()))
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s %d\n", e.Name(), len(b))
		h.Write(b)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)) + " fern", nil
}

// Groups are the groups dir's fern/generators.yml declares, in its order.
func Groups(dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "fern", "generators.yml"))
	if err != nil {
		return nil, err
	}
	_, block, _ := strings.Cut(string(b), "\ngroups:\n")
	if loc := regexp.MustCompile(`(?m)^\S`).FindStringIndex(block); loc != nil {
		block = block[:loc[0]]
	}
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^ {2}([a-z][a-z0-9-]*):\s*$`).FindAllStringSubmatch(block, -1) {
		out = append(out, m[1])
	}
	return out, nil
}

// madeFrom is what each SDK in sdk/out was made from (SpecsHash, by group), in one file beside them.
func madeFrom(dir string) string { return filepath.Join(dir, "sdk", "out", ".made-from.json") }

func stamps(dir string) map[string]string {
	m := map[string]string{}
	if b, err := os.ReadFile(madeFrom(dir)); err == nil {
		json.Unmarshal(b, &m)
	}
	return m
}

var stampMu sync.Mutex

func stamp(dir, group string) error {
	hash, err := SpecsHash(dir)
	if err != nil {
		return err
	}
	stampMu.Lock()
	defer stampMu.Unlock()
	m := stamps(dir)
	m[group] = hash
	b, _ := json.MarshalIndent(m, "", "\t")
	return os.WriteFile(madeFrom(dir), append(b, '\n'), 0o644)
}

// UpToDate reports whether sdk/out/<group> was made from fern/ as it is.
func UpToDate(dir, group string) bool {
	if _, err := os.Stat(filepath.Join(dir, "sdk", "out", group)); err != nil {
		return false
	}
	hash, err := SpecsHash(dir)
	return err == nil && stamps(dir)[group] == hash
}

// Generate generates a group of dir's fern/generators.yml with Fern (Docker) into sdk/out/<group>, and records the
// hash of fern/ it was made from beside it. With ifStale, only when there is no SDK yet or fern/ is not what it was
// made from: the specs come from the contract, so a contract change reaches the SDK. With quiet, Fern's output is
// kept rather than shown, so several can run at once (each in a container of its own) without their lines mixing.
func Generate(dir, group string, ifStale, quiet bool) (time.Duration, error) {
	if ifStale && UpToDate(dir, group) {
		fmt.Printf("sdk %s: up to date (made from fern/ as it is), not generated again\n", group)
		return 0, nil
	}
	if err := Ready(); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "sdk", "out"), 0o755); err != nil {
		return 0, err
	}
	env, err := ContainerEnv()
	if err != nil {
		return 0, err
	}
	args := []string{"generate", "--local", "--group", group, "--force", "--log-level", "warn"}
	start := time.Now()
	if quiet {
		_, err = run.Quietly(dir, env, "fern", args...)
	} else {
		err = run.Run(dir, env, "fern", args...)
	}
	if err != nil {
		return 0, err
	}
	// A generator that fails inside its container (pnpm unable to reach npm, say) exits 0 there, and Fern then
	// reports "Finished" over an empty folder, so the failure would only show later, as a module the SDK lacked.
	if entries, _ := os.ReadDir(filepath.Join(dir, "sdk", "out", group)); len(entries) == 0 {
		return 0, fmt.Errorf("fern generate --group %s finished but wrote nothing to sdk/out/%s: its generator failed inside its container, and Fern does not pass that on. Its own output says why: fern generate --local --group %s --log-level debug", group, group, group)
	}
	return time.Since(start), stamp(dir, group)
}

// tsconfig is how a generated TypeScript SDK is typechecked: standard fetch, none of the project's own settings.
// The projects stay on @types/node 22: with 26, a generated TypeScript SDK's Headers does not typecheck.
// Upstream: fern-api/fern#18313 (when fixed: @types/node 26 in every package.json)
func tsconfig(entry string) []byte {
	b, _ := json.MarshalIndent(map[string]any{
		"//": "Written by charter sdk -prove: how a generated TypeScript SDK is typechecked (bundler mode, default libs).",
		"compilerOptions": map[string]any{"target": "es2022", "module": "preserve", "moduleResolution": "bundler", "allowImportingTsExtensions": true,
			"strict": true, "noEmit": true, "skipLibCheck": true},
		"include": []string{entry},
	}, "", "  ")
	return b
}

// ProveGroup proves a generated group, in its language, by what is in sdk/out/<group>. What it runs keeps its
// output, shown only if it fails: the groups are proven side by side.
func ProveGroup(dir, group string) error {
	out := filepath.Join(dir, "sdk", "out", group)
	if entries, _ := os.ReadDir(out); len(entries) == 0 {
		return fmt.Errorf("%s: nothing generated (charter sdk %s)", out, group)
	}
	exists := func(rel string) bool { _, err := os.Stat(filepath.Join(out, rel)); return err == nil }
	switch {
	case exists("go.mod"):
		// A generated module is its own: no workspace above it applies.
		off := map[string]string{"GOWORK": "off"}
		for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
			if _, err := run.Quietly(out, off, "go", args...); err != nil {
				return err
			}
		}
		compose := filepath.Join(out, "wiremock", "docker-compose.test.yml")
		if !exists(filepath.Join("wiremock", "docker-compose.test.yml")) {
			_, err := run.Quietly(out, off, "go", "test", "./...")
			return err
		}
		if err := Ready(); err != nil {
			return err
		}
		if _, err := run.Quietly("", nil, "docker", "compose", "-f", compose, "up", "-d", "--wait"); err != nil {
			return err
		}
		defer run.Quietly("", nil, "docker", "compose", "-f", compose, "down")
		port, err := run.Output("", nil, "docker", "compose", "-f", compose, "port", "wiremock", "8080")
		if err != nil {
			return err
		}
		_, err = run.Quietly(out, map[string]string{"GOWORK": "off", "WIREMOCK_URL": "http://" + strings.TrimSpace(port)}, "go", "test", "./...")
		return err
	case firstOf(out, "index.ts", "src/index.ts", "sdk/index.ts") != "":
		// Fern's SDKs are written for TypeScript 5: a project on a newer one has 5.9 beside it as typescript-sdk
		// ("npm:typescript@~5.9.3"); a project on 5.9 already, its own.
		entry := firstOf(out, "index.ts", "src/index.ts", "sdk/index.ts")
		config := filepath.Join(out, "tsconfig.check.json")
		if err := os.WriteFile(config, tsconfig(entry), 0o644); err != nil {
			return err
		}
		defer os.Remove(config)
		if tsc := firstOf(dir, "node_modules/typescript-sdk/bin/tsc", "../../node_modules/typescript-sdk/bin/tsc"); tsc != "" {
			_, err := run.Quietly(dir, nil, "node", filepath.Join(dir, tsc), "-p", config)
			return err
		}
		_, err := run.Quietly(dir, nil, "tsc", "-p", config)
		return err
	case exists("__init__.py") && exists("client.py"):
		return python(dir, out, group)
	case exists("esm/index.mjs") && exists("esm/index.d.mts"):
		fmt.Printf("%s: compiled (esm/index.mjs and its types), as a release ships it\n", out)
		return nil
	case exists("Cargo.toml"):
		cargo, _ := os.ReadFile(filepath.Join(out, "Cargo.toml"))
		bin := ""
		if m := regexp.MustCompile(`\[\[bin\]\][^\[]*?name\s*=\s*"([^"]+)"`).FindSubmatch(cargo); m != nil {
			bin = string(m[1])
		}
		// A debug build: it proves the CLI compiles and links, in a tenth of a release build's time (shipping one is
		// a release's job). Fern writes the CLI's folder anew each time, its build folder with it: Cargo builds into
		// charter's cache instead (CARGO_TARGET_DIR), so the crates it depends on are built once.
		target, err := run.Cache("cargo")
		if err != nil {
			return err
		}
		args := []string{"exec", "rust@" + rustVersion, "--", "cargo", "build"}
		if bin != "" {
			args = append(args, "--bin", bin)
		}
		args = append(args, "--no-default-features", "--features", "rustls")
		_, err = run.Quietly(out, map[string]string{"CARGO_TARGET_DIR": target}, "mise", args...)
		return err
	}
	fmt.Printf("%s: generated; charter proves Go, TypeScript, Python and the Rust CLI, not this language\n", out)
	return nil
}

// python proves a Python client: Fern writes the package's source alone, so it goes on an import path under the
// name its tests import it by; then mypy over it, and its own tests.
func python(dir, out, group string) error {
	var tests string
	if entries, err := os.ReadDir(filepath.Join(out, "tests")); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".py") {
				b, _ := os.ReadFile(filepath.Join(out, "tests", e.Name()))
				tests += string(b) + "\n"
			}
		}
	}
	name := "client"
	if m := regexp.MustCompile(`(?m)^\s*from ([a-z_][a-z0-9_]*)\.client import`).FindStringSubmatch(tests); m != nil {
		name = m[1]
	}
	site := filepath.Join(dir, "sdk", "out", ".work", group+"-import")
	os.RemoveAll(site)
	if err := os.MkdirAll(site, 0o755); err != nil {
		return err
	}
	defer func() {
		os.RemoveAll(site)
		os.Remove(filepath.Join(dir, "sdk", "out", ".work")) // only if no other proof is using it
	}()
	abs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if err := os.Symlink(abs, filepath.Join(site, name)); err != nil {
		return err
	}
	uv := func(args ...string) error {
		full := []string{"exec", "uv@" + uvVersion, "--", "uv", "run", "--no-project", "--python", pythonVersion}
		for _, p := range pythonPackages {
			full = append(full, "--with", p)
		}
		_, err := run.Quietly(site, nil, "mise", append(append(full, "--"), args...)...)
		return err
	}
	if err := uv("mypy", "-p", name); err != nil {
		return err
	}
	if tests != "" {
		return uv("python", "-m", "pytest", "-q", filepath.Join(name, "tests"))
	}
	return uv("python", "-c", "import "+name)
}

// firstOf is the first of rels that exists in dir, "" if none does.
func firstOf(dir string, rels ...string) string {
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			return rel
		}
	}
	return ""
}

// Prove generates every group dir's project declares (or only those named) and proves each, all at once: Fern
// generates each in a container of its own, and each is proven in its language as soon as it is generated (the Rust
// CLI's build, the long one, starts early). Then, in a project that commits its Go SDK, that sdk/go is what the specs
// generate now.
func Prove(dir string, only []string, committedGoSDK bool) error {
	wanted := only
	if len(wanted) == 0 {
		var err error
		if wanted, err = Groups(dir); err != nil {
			return err
		}
	}
	start := time.Now()
	fmt.Printf("== generating and proving %s, all at once\n", strings.Join(wanted, ", "))
	times := make([]string, len(wanted))
	errs := make([]error, len(wanted))
	var wg sync.WaitGroup
	for i, group := range wanted {
		wg.Go(func() {
			generated, err := Generate(dir, group, false, true)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", group, err)
				return
			}
			fmt.Printf("client %s: generated in %s\n", group, run.Seconds(generated))
			t0 := time.Now()
			if err := ProveGroup(dir, group); err != nil {
				errs[i] = fmt.Errorf("%s: %w", group, err)
				return
			}
			proven := time.Since(t0)
			fmt.Printf("client %s: proven in %s\n", group, run.Seconds(proven))
			times[i] = fmt.Sprintf("%s %s (%s + %s)", group, run.Seconds(generated+proven), run.Seconds(generated), run.Seconds(proven))
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("the SDKs that failed:\n%w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sdk", "go")); committedGoSDK && err == nil && len(only) == 0 {
		t0 := time.Now()
		if err := Publish(dir, true); err != nil {
			return err
		}
		times = append(times, "committed Go SDK "+run.Seconds(time.Since(t0)))
	}
	fmt.Printf("sdk -prove took %s, all at once: %s (generated + proven)\n", run.Seconds(time.Since(start)), strings.Join(times, ", "))
	return nil
}

// goSDKMadeFrom is the file in sdk/go that says what it was made from (SpecsHash).
const goSDKMadeFrom = ".made-from"

// GoSDKFresh fails if the specs changed since dir's sdk/go was made from them (no Docker: it compares a hash).
func GoSDKFresh(dir string) error {
	sdk := filepath.Join(dir, "sdk", "go")
	if _, err := os.Stat(sdk); err != nil {
		return nil
	}
	made, _ := os.ReadFile(filepath.Join(sdk, goSDKMadeFrom))
	hash, err := SpecsHash(dir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(made)) != hash {
		return errors.New("sdk/go was not made from the specs in fern/ as they are now: charter sdk -publish writes it again (Docker), then commit it")
	}
	fmt.Println("sdk/go was made from the specs in fern/ as they are now")
	return nil
}

// sources are the files of a generated Go SDK that are committed: what a program that imports it needs and the tests
// that run on their own; not Fern's run record (.fern), nor WireMock's fixtures and the tests that need its container.
func sources(dir string) (map[string][]byte, error) {
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != dir && (d.Name() == ".fern" || d.Name() == "wiremock" || strings.HasSuffix(d.Name(), "_test")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || d.Name() == "CONTRIBUTING.md" || d.Name() == goSDKMadeFrom {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		out[filepath.ToSlash(rel)] = b
		return err
	})
	return out, err
}

// Publish generates the Go SDK (Docker), proves it (build, vet, Fern's tests against WireMock) and copies its sources
// into sdk/go, the committed module another repo can go get. With check it writes nothing and fails if sdk/go is not
// what the specs generate now.
func Publish(dir string, check bool) error {
	out := filepath.Join(dir, "sdk", "out", "go")
	os.RemoveAll(out)
	if _, err := Generate(dir, "go", false, false); err != nil {
		return err
	}
	gomod, err := os.ReadFile(filepath.Join(out, "go.mod"))
	if err != nil {
		return err
	}
	module := ""
	if m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(gomod); m != nil {
		module = string(m[1])
	}
	if !strings.HasSuffix(module, "/sdk/go") {
		return fmt.Errorf("%s/go.mod declares %s: charter spec writes the go group's module as <the project's module>/sdk/go", out, module)
	}
	generated, err := sources(out)
	if err != nil {
		return err
	}
	generated["README.md"] = append([]byte("> **Generated code.** Fern generated this Go client from the API's contract (the specs in `fern/`). Don't edit it: change the contract, then `charter sdk -publish` writes this folder again.\n\n"), generated["README.md"]...)
	sdk := filepath.Join(dir, "sdk", "go")
	if check {
		committed := map[string][]byte{}
		if _, err := os.Stat(sdk); err == nil {
			if committed, err = sources(sdk); err != nil {
				return err
			}
		}
		var stale []string
		for _, file := range slices.Sorted(maps.Keys(generated)) {
			switch c, ok := committed[file]; {
			case !ok:
				stale = append(stale, "  missing: "+file)
			case !bytes.Equal(c, generated[file]):
				stale = append(stale, "  differs: "+file)
			}
		}
		for _, file := range slices.Sorted(maps.Keys(committed)) {
			if _, ok := generated[file]; !ok {
				stale = append(stale, "  not generated: "+file)
			}
		}
		if len(stale) > 0 {
			return fmt.Errorf("sdk/go is stale against the specs in fern/ (charter sdk -publish writes it again):\n%s", strings.Join(stale, "\n"))
		}
		fmt.Printf("sdk/go is what the specs in fern/ generate (%d files)\n", len(generated))
		return nil
	}
	// Proven before it is committed, as sdk -prove proves it.
	if err := ProveGroup(dir, "go"); err != nil {
		return err
	}
	os.RemoveAll(sdk)
	for file, content := range generated {
		path := filepath.Join(sdk, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}
	hash, err := SpecsHash(dir)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(sdk, goSDKMadeFrom), []byte(hash+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("published: sdk/go (module %s, %d files). Commit it; another repo gets it with: go get %s@<branch, commit or version>\n", module, len(generated), module)
	return nil
}
