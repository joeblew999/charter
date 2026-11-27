package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/tools"
)

// An app's tools: Go, Node and fnox, at the versions its mise.toml pins, installed by mise on every machine and in
// CI. charter check holds two things to that:
//
//   - the app pins at least what the charter it builds with was tested with (charter's own mise.toml, which comes
//     with the library): an app made by charter create starts with those pins, and check --fix raises them after a
//     charter upgrade. It never lowers them.
//   - what runs is what is pinned. A shell whose PATH mise has not set for this folder runs another Go or Node, and
//     what is built on this machine is then not what CI builds.

// pinned are the tools charter's own mise.toml pins, by their names there.
var pinned = []string{"go", "node", "github:jdx/fnox"}

// fnoxTool is fnox's name in a mise.toml.
const fnoxTool = "github:jdx/fnox"

// pin is a tool's pinned version, and the file that pins it.
type pin struct{ Version, File string }

// effectivePins are the versions mise gives this app's folder, of the tools charter pins: from the app's mise.toml or
// one in a folder above it (charter's own, for its example). A version only a developer's global config gives is no
// pin of the app's.
func effectivePins(p *project.Project) (map[string]pin, error) {
	out, err := exec.Command(tools.Mise(), "ls", "--current", "--json", "-C", p.Root).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("mise could not say what this folder's tools are: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, err
	}
	var listed map[string][]struct {
		Version   string `json:"version"`
		Requested string `json:"requested_version"`
		Source    struct {
			Path string `json:"path"`
		} `json:"source"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		return nil, fmt.Errorf("mise ls: %w", err)
	}
	global := filepath.Join(home(), ".config", "mise")
	pins := map[string]pin{}
	for _, name := range pinned {
		for _, t := range listed[name] {
			if t.Source.Path == "" || strings.HasPrefix(t.Source.Path, global) {
				continue
			}
			pins[name] = pin{t.Version, t.Source.Path}
		}
	}
	return pins, nil
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

// libraryPins are the tools charter's own mise.toml pins: the file that comes with the version of the library the app
// builds with.
func libraryPins(p *project.Project) (dir string, pins map[string]string, err error) {
	out, err := run.Output(p.Root, nil, "go", "list", "-m", "-f", "{{.Dir}}", library)
	if err != nil {
		return "", nil, err
	}
	dir = strings.TrimSpace(out)
	pins, err = miseTools(filepath.Join(dir, "mise.toml"))
	return dir, pins, err
}

// toolLine is one tool of a [tools] table: its name, bare or quoted, and its version.
var toolLine = regexp.MustCompile(`^(\s*)("[^"]+"|[A-Za-z0-9_.:/-]+)(\s*=\s*)"([^"]*)"`)

// miseTools are the tools of a mise.toml's [tools] table, by name.
func miseTools(file string) (map[string]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "[") {
			in = trimmed == "[tools]"
			continue
		}
		if m := toolLine.FindStringSubmatch(line); in && m != nil {
			out[strings.Trim(m[2], `"`)] = m[4]
		}
	}
	return out, nil
}

// raise sets each tool of want in a mise.toml's [tools] table to its version, where the file has it lower or not at
// all, and says which. Comments and everything else stay as they are.
func raise(file string, want map[string]string) ([]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	seen := map[string]bool{}
	var raised []string
	in, end := false, -1
	for i, line := range lines {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "[") {
			if in {
				end = i
			}
			in = trimmed == "[tools]"
			continue
		}
		m := toolLine.FindStringSubmatch(line)
		if !in || m == nil {
			continue
		}
		name := strings.Trim(m[2], `"`)
		v, ok := want[name]
		if !ok {
			continue
		}
		seen[name] = true
		if older(m[4], v) {
			lines[i] = m[1] + m[2] + m[3] + strconv.Quote(v) + line[len(m[0]):]
			raised = append(raised, fmt.Sprintf("%s %s → %s", name, m[4], v))
		}
	}
	var missing []string
	for _, name := range pinned {
		if v, ok := want[name]; ok && !seen[name] {
			key := name
			if strings.ContainsAny(name, ":/") {
				key = strconv.Quote(name)
			}
			missing = append(missing, key+" = "+strconv.Quote(v))
			raised = append(raised, name+" "+v)
		}
	}
	if len(missing) > 0 {
		if !slices.ContainsFunc(lines, func(l string) bool { return strings.TrimSpace(l) == "[tools]" }) {
			lines = append(lines, "", "[tools]")
			end = -1
		}
		if end < 0 {
			lines = append(lines, missing...)
		} else {
			lines = slices.Insert(lines, end, missing...)
		}
	}
	if len(raised) == 0 {
		return nil, nil
	}
	return raised, os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644)
}

// older reports whether version a is older than b: 1.27.1 than 1.27.4. Parts that are no numbers compare as text.
func older(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		var x, y string
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil && xn != yn:
			return xn < yn
		case (xerr != nil || yerr != nil) && x != y:
			return x < y
		}
	}
	return false
}

// running is the version of a tool that runs here, as charter's own commands start it.
func running(p *project.Project, name string) string {
	switch name {
	case "go":
		out, _ := run.Output(p.Root, nil, "go", "env", "GOVERSION")
		return strings.TrimPrefix(strings.TrimSpace(out), "go")
	case "node":
		out, _ := exec.Command("node", "--version").Output()
		return strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	case fnoxTool:
		bin, err := tools.Fnox()
		if err != nil {
			return ""
		}
		out, _ := exec.Command(bin, "--version").Output()
		return strings.TrimPrefix(strings.TrimSpace(string(out)), "fnox ")
	}
	return ""
}

// toolchain is check's part "tools": the app pins at least what its charter was tested with (fix: they are raised,
// and installed), and what runs here is what is pinned.
func toolchain(p *project.Project, fix bool) error {
	changed := false
	if tools.Mise() == "" {
		return errors.New(tools.NoMise)
	}
	libDir, lib, err := libraryPins(p)
	if err != nil {
		return fmt.Errorf("charter's own mise.toml: %w", err)
	}
	own := filepath.Join(p.Root, "mise.toml")
	if filepath.Clean(libDir) != filepath.Clean(p.Root) && !exists(own) {
		// An app without its own (one made before charter pinned its tools): it gets charter's.
		pins, err := effectivePins(p)
		if err != nil {
			return err
		}
		if len(pins) == 0 || !strings.HasPrefix(filepath.Clean(pins["go"].File), filepath.Clean(libDir)) {
			if !fix {
				return errors.New("the app has no mise.toml, which pins the Go, Node and fnox it is built with: go tool charter check --fix writes charter's")
			}
			if err := copyFile(filepath.Join(libDir, "mise.toml"), own); err != nil {
				return err
			}
			if exists(filepath.Join(libDir, "mise.lock")) {
				if err := copyFile(filepath.Join(libDir, "mise.lock"), filepath.Join(p.Root, "mise.lock")); err != nil {
					return err
				}
			}
			say("mise.toml: charter's pins, which this app now builds with")
			if err := installTools(p); err != nil {
				return err
			}
			changed = true
		}
	} else if exists(own) && filepath.Clean(libDir) != filepath.Clean(p.Root) {
		// Not below what this charter is tested with.
		app, err := miseTools(own)
		if err != nil {
			return err
		}
		var low []string
		for _, name := range pinned {
			if v, ok := lib[name]; ok && (app[name] == "" || older(app[name], v)) {
				low = append(low, fmt.Sprintf("%s %s (charter is built with %s)", name, orNone(app[name], "not pinned"), v))
			}
		}
		if len(low) > 0 && !fix {
			return fmt.Errorf("mise.toml pins less than the charter it builds with was tested with: go tool charter check --fix raises them\n  %s", strings.Join(low, "\n  "))
		}
		if len(low) > 0 {
			raised, err := raise(own, lib)
			if err != nil {
				return err
			}
			say("mise.toml: %s", strings.Join(raised, ", "))
			if err := installTools(p); err != nil {
				return err
			}
			changed = true
		}
	}
	// Tools changed by this run are installed, and run in the next shell: this one started before them.
	if changed {
		return nil
	}
	pins, err := effectivePins(p)
	if err != nil {
		return err
	}
	var differ []string
	for _, name := range pinned {
		want, ok := pins[name]
		if !ok {
			differ = append(differ, name+": mise.toml does not pin it")
			continue
		}
		if got := running(p, name); got != want.Version {
			differ = append(differ, fmt.Sprintf("%s %s runs here; %s pins %s", name, orNone(got, "(none)"), want.File, want.Version))
		}
	}
	if len(differ) > 0 {
		return fmt.Errorf("this machine does not run the tools the app pins, so what it builds is not what CI builds:\n  %s\n"+
			"mise install, then a new shell (mise activate sets the PATH as you change folder; a shell that does not run its hook, as an editor's task or an agent's, gets it with eval \"$(mise hook-env)\")",
			strings.Join(differ, "\n  "))
	}
	return nil
}

// installTools has mise trust the app's mise.toml, bring its lock file up to its pins, and install them.
func installTools(p *project.Project) error {
	mise := tools.Mise()
	if err := run.Run(p.Root, nil, mise, "trust", "--quiet"); err != nil {
		return err
	}
	if exists(filepath.Join(p.Root, "mise.lock")) {
		if err := run.Run(p.Root, nil, mise, "lock"); err != nil {
			return err
		}
	}
	if err := run.Run(p.Root, nil, mise, "install"); err != nil {
		return err
	}
	say("· the tools changed: a new shell runs them (or eval \"$(mise hook-env)\" in this one)")
	return nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}
