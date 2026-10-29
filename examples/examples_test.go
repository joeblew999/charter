// Package examples holds the tests that keep this repo's projects the same where they can be (docs/self/standard.md):
// no npm scripts (a project's steps are charter's, from its charter.toml), one version of each package they share,
// and one TypeScript setup. A difference here is an accident, found before anyone reads the projects side by side.
// Each project is a module or package of its own; this folder is part of charter's root module, so go test ./... at the
// top runs these (charter check -go).
package examples

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type pkg struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Workspaces      []string          `json:"workspaces"`
}

func read(t *testing.T, path string) pkg {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var p pkg
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return p
}

// projects are the root's npm workspaces that are projects (a charter.toml), as folders from here.
func projects(t *testing.T) []string {
	var out []string
	for _, w := range read(t, filepath.Join("..", "package.json")).Workspaces {
		if _, err := os.Stat(filepath.Join("..", w, "charter.toml")); err == nil {
			out = append(out, filepath.Join("..", w))
		}
	}
	if len(out) < 8 {
		t.Fatalf("the projects are charter.toml's: %v", out)
	}
	return out
}

func TestNoScripts(t *testing.T) {
	for _, dir := range projects(t) {
		if s := read(t, filepath.Join(dir, "package.json")).Scripts; len(s) > 0 {
			t.Errorf("%s: npm scripts %v: a project's steps are charter's, from its charter.toml", dir, s)
		}
		for _, f := range []string{"charter.config.ts", "cloudflare.config.ts", "vite.config.ts"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
				t.Errorf("%s: %s, which charter.toml replaces", dir, f)
			}
		}
	}
	// The repo's only script is npm's own: charter-lib built on install. Every command is charter's.
	if s := read(t, filepath.Join("..", "package.json")).Scripts; len(s) != 1 || s["prepare"] == "" {
		t.Errorf("the root package.json's scripts: %v, want only prepare", s)
	}
}

func TestOneVersionEach(t *testing.T) {
	versions := map[string]map[string][]string{}
	for _, dir := range projects(t) {
		p := read(t, filepath.Join(dir, "package.json"))
		for name, v := range mergeDeps(p) {
			if versions[name] == nil {
				versions[name] = map[string][]string{}
			}
			versions[name][v] = append(versions[name][v], filepath.Base(dir))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(versions)) {
		if len(versions[name]) > 1 {
			t.Errorf("%s at more than one version: %v", name, versions[name])
		}
	}
}

func TestOneTypeScript(t *testing.T) {
	for _, dir := range projects(t) {
		dev := read(t, filepath.Join(dir, "package.json")).DevDependencies
		if dev["typescript"] == "" || dev["typescript-sdk"] == "" {
			t.Errorf("%s: typescript and typescript-sdk (the one Fern's TypeScript SDK builds with)", dir)
		}
	}
}

// TestSameTSConfig: every project's tsconfig.json (the Worker's) and tsconfig.node.json (its tests') have the same
// options: each its own, so the project stands alone.
func TestSameTSConfig(t *testing.T) {
	for _, file := range []string{"tsconfig.json", "tsconfig.node.json"} {
		first, firstDir := "", ""
		for _, dir := range projects(t) {
			b, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				continue
			}
			var c struct {
				CompilerOptions json.RawMessage `json:"compilerOptions"`
			}
			if err := json.Unmarshal(b, &c); err != nil {
				t.Fatalf("%s/%s: %v", dir, file, err)
			}
			var v any
			json.Unmarshal(c.CompilerOptions, &v)
			got := fmt.Sprint(v)
			if first == "" {
				first, firstDir = got, dir
			} else if got != first {
				t.Errorf("%s/%s: its compilerOptions differ from %s's", dir, file, firstDir)
			}
		}
		if first == "" {
			t.Errorf("no project has a %s", file)
		}
	}
}

// TestPinned: every package is pinned by version, not by a tag that moves (beta, latest): a project made by new
// installs what was tested.
func TestPinned(t *testing.T) {
	moving := regexp.MustCompile(`^(latest|beta|next|canary|alpha|\*)$`)
	all := append([]string{filepath.Join("..", "ts")}, projects(t)...)
	for _, dir := range all {
		for name, v := range mergeDeps(read(t, filepath.Join(dir, "package.json"))) {
			if moving.MatchString(strings.TrimSpace(v)) {
				t.Errorf("%s: %s@%s, a tag that moves", dir, name, v)
			}
		}
	}
}

func mergeDeps(p pkg) map[string]string {
	out := maps.Clone(p.Dependencies)
	if out == nil {
		out = map[string]string{}
	}
	maps.Copy(out, p.DevDependencies)
	return out
}
