// Package deps is what charter outdated and charter update need to move a repo's pins: the holds it keeps
// (deps.toml: what stays where it is, and why), the repository's Go modules, the latest version of a Go module, and the
// pins in mise.toml.
package deps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// Holds are deps.toml's [hold] table: a name (a Go module, a mise tool as mise.toml names it, an npm package, "htmx")
// or a pattern ending in * (a prefix: "@moq/*"), and why it stays where it is.
type Holds map[string]string

// Config is root's deps.toml: its holds, and the copies of a pin written outside its own file ([copies]: a name and
// the files, from the root, that write its version too, as a quoted string), which an update moves with it.
//
// Under the name "*" are files written as mise.toml is (a pin a line, key = "version"): every tool they pin moves
// with mise.toml's. Skip are folders, from the root, whose go.mod isn't the repo's to move: a generated SDK's.
type Config struct {
	Hold   Holds
	Copies map[string][]string
	Skip   []string
}

// Read reads root's deps.toml; none is no holds and no copies.
func Read(root string) (Config, error) {
	var c Config
	if _, err := toml.DecodeFile(filepath.Join(root, "deps.toml"), &c); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("deps.toml: %w", err)
	}
	if c.Hold == nil {
		c.Hold = Holds{}
	}
	return c, nil
}

// ReadHolds is root's deps.toml's holds.
func ReadHolds(root string) (Holds, error) {
	c, err := Read(root)
	return c.Hold, err
}

// MoveCopies moves name's copies ([copies]) from version from to version to: "from" becomes "to" in each file.
func (c Config) MoveCopies(root, name, from, to string) error {
	for _, rel := range c.Copies["*"] {
		if _, pinned := pinIn(filepath.Join(root, rel), name); pinned {
			if err := setPinIn(filepath.Join(root, rel), name, to); err != nil {
				return err
			}
		}
	}
	for _, rel := range c.Copies[name] {
		path := filepath.Join(root, rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("deps.toml's copy of %s: %w", name, err)
		}
		if !bytes.Contains(b, []byte(`"`+from+`"`)) {
			return fmt.Errorf("deps.toml says %s writes %s's version, and it has no %q", rel, name, from)
		}
		if err := os.WriteFile(path, bytes.ReplaceAll(b, []byte(`"`+from+`"`), []byte(`"`+to+`"`)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Why is why name is held, ok false when it isn't.
func (h Holds) Why(name string) (why string, ok bool) {
	if why, ok := h[name]; ok {
		return why, true
	}
	for pattern, why := range h {
		if prefix, ok := strings.CutSuffix(pattern, "*"); ok && strings.HasPrefix(name, prefix) {
			return why, true
		}
	}
	return "", false
}

// Module is one of the repository's Go modules.
type Module struct {
	Dir      string            // its directory
	Path     string            // its module path
	Requires map[string]string // the modules it requires directly (or for a tool), by path: their versions
	Replaced map[string]bool   // the modules it replaces with another
}

// Modules are every Go module of the repository at root: each go.mod git tracks, but those in the folders skip names.
func Modules(root string, skip ...string) ([]Module, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "go.mod", "*/go.mod").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var mods []Module
	for _, rel := range strings.Fields(string(out)) {
		if slices.Contains(skip, filepath.ToSlash(filepath.Dir(rel))) {
			continue
		}
		path := filepath.Join(root, rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		f, err := modfile.Parse(path, b, nil)
		if err != nil {
			return nil, err
		}
		m := Module{Dir: filepath.Dir(path), Path: f.Module.Mod.Path, Requires: map[string]string{}, Replaced: map[string]bool{}}
		var tools []string
		for _, t := range f.Tool {
			tools = append(tools, t.Path)
		}
		for _, r := range f.Require {
			tool := false
			for _, t := range tools {
				tool = tool || t == r.Mod.Path || strings.HasPrefix(t, r.Mod.Path+"/")
			}
			if !r.Indirect || tool {
				m.Requires[r.Mod.Path] = r.Mod.Version
			}
		}
		for _, r := range f.Replace {
			m.Replaced[r.Old.Path] = true
		}
		mods = append(mods, m)
	}
	return mods, nil
}

// Latest is the newest version of a Go module, as the module proxy has it: path@latest, or path@query (path@main).
// It asks from outside any module: inside one that replaces the module, go list answers with a placeholder version.
func Latest(dir, query string) (string, error) {
	if !strings.Contains(query, "@") {
		query += "@latest"
	}
	cmd := exec.Command("go", "list", "-m", "-json", query)
	cmd.Dir = os.TempDir()
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %v: %s", query, err, strings.TrimSpace(stderr.String()))
	}
	var m struct{ Version string }
	if err := json.Unmarshal(out, &m); err != nil {
		return "", err
	}
	return m.Version, nil
}

// Newer is whether latest is a newer version than current (a pseudo-version past the latest tag is not behind it).
func Newer(latest, current string) bool {
	return semver.IsValid(latest) && semver.IsValid(current) && semver.Compare(latest, current) > 0
}

// Commit is the commit a pseudo-version names: v0.0.0-20261030163719-65f4813ea8be is 65f4813ea8be.
func Commit(version string) string {
	if i := strings.LastIndex(version, "-"); i >= 0 && len(version)-i == 13 {
		return version[i+1:]
	}
	return ""
}

// Run runs name with args in dir, its output to the terminal.
func Run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// misePin matches a [tools] line of mise.toml: "go:github.com/gsxhq/gsxui/cmd/gsxui" = "65f4813e", workerd = "1.2".
func misePin(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(\s*"?` + regexp.QuoteMeta(key) + `"?\s*=\s*")([^"]*)(")`)
}

// MisePin is the version root's mise.toml pins the tool key at.
func MisePin(root, key string) (string, bool) { return pinIn(filepath.Join(root, "mise.toml"), key) }

// pinIn is the version the file at path, written as mise.toml is, pins key at.
func pinIn(path, key string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	m := misePin(key).FindSubmatch(b)
	if m == nil {
		return "", false
	}
	return string(m[2]), true
}

// SetMisePin pins the tool key at version in root's mise.toml.
func SetMisePin(root, key, version string) error {
	return setPinIn(filepath.Join(root, "mise.toml"), key, version)
}

// setPinIn pins key at version in the file at path, written as mise.toml is.
func setPinIn(path, key, version string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	re := misePin(key)
	if !re.Match(b) {
		return fmt.Errorf("%s pins no %s", filepath.Base(path), key)
	}
	return os.WriteFile(path, re.ReplaceAll(b, []byte("${1}"+version+"${3}")), 0o644)
}

// Change is one pin moved, or one that could be.
type Change struct {
	Name, From, To string
	Where          string // the file or module it is pinned in
	Held           string // why it stays, when it does
	Notes          string // where to read what changed
}

// Report prints changes as a table, the held ones last.
func Report(title string, changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Held == "" && changes[j].Held != "" })
	fmt.Printf("\n%s\n", title)
	if len(changes) == 0 {
		fmt.Println("  · nothing")
		return
	}
	for _, c := range changes {
		mark := "↑"
		if c.Held != "" {
			mark = "="
		}
		fmt.Printf("  %s %s %s → %s  (%s)\n", mark, c.Name, c.From, c.To, c.Where)
		if c.Held != "" {
			fmt.Printf("      held: %s\n", c.Held)
		} else if c.Notes != "" {
			fmt.Printf("      %s\n", c.Notes)
		}
	}
}
