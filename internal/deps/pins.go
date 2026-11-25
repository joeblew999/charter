package deps

import (
	"encoding/json"
	"fmt"
	"maps"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Charter's own two pins in a repo that uses it: the library in go.mod and the command in mise.toml, the same
// version, moved together.
const (
	CharterModule = "github.com/joeblew999/charter"
	CharterTool   = "github:joeblew999/charter"
)

// Options say which pins a pass looks at, and whether it moves them.
type Options struct {
	Update bool                   // move each pin that isn't held; false only reports
	Skip   func(path string) bool // Go modules left to something else (a repo's own modules are always left)
	Warn   func(error)            // a pin whose latest version couldn't be asked: reported, never fatal (nil drops it)
}

func (o Options) warn(err error) {
	if o.Warn != nil && err != nil {
		o.Warn(err)
	}
}

// where is a module's go.mod as the report names it: from the root.
func where(root, dir string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return "go.mod"
	}
	return filepath.ToSlash(filepath.Join(rel, "go.mod"))
}

// GoModules are the Go modules the repository's modules require directly (or for a tool) that upstream has a newer
// version of; with Update each is moved, in every module that requires it, unless it is held or the module replaces
// it. A module of the repository itself is never moved: its version is the repository's. Charter's library moves
// with its command (mise.toml), and charter init then writes charter's files again.
func GoModules(root string, mods []Module, conf Config, o Options) ([]Change, error) {
	own := map[string]bool{}
	for _, m := range mods {
		own[m.Path] = true
	}
	latest := map[string]string{}
	var out []Change
	for _, m := range mods {
		for _, path := range slices.Sorted(maps.Keys(m.Requires)) {
			current := m.Requires[path]
			if own[path] || ownSub(own, path) || (o.Skip != nil && o.Skip(path)) {
				continue
			}
			if _, ok := latest[path]; !ok {
				v, err := Latest(root, path)
				o.warn(err)
				latest[path] = v
			}
			if !Newer(latest[path], current) {
				continue
			}
			c := Change{Name: path, From: current, To: latest[path], Where: where(root, m.Dir)}
			switch why, held := conf.Hold.Why(path); {
			case held:
				c.Held = why
			case m.Replaced[path]:
				c.Held = "go.mod replaces it"
			case o.Update:
				if err := Run(m.Dir, "go", "get", path+"@"+latest[path]); err != nil {
					return out, err
				}
				if _, pinned := MisePin(root, CharterTool); path == CharterModule && m.Dir == root && pinned {
					if err := SetMisePin(root, CharterTool, strings.TrimPrefix(latest[path], "v")); err != nil {
						return out, err
					}
					for _, step := range [][]string{{"mise", "install"}, {"mise", "exec", "--", "charter", "init"}} {
						if err := Run(root, step[0], step[1:]...); err != nil {
							return out, err
						}
					}
				}
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// ownSub is whether path is inside one of the repository's own modules (a package or a nested module of it).
func ownSub(own map[string]bool, path string) bool {
	for o := range own {
		if strings.HasPrefix(path, o+"/") {
			return true
		}
	}
	return false
}

// MiseTools are the tools root's mise.toml pins that mise knows a newer version of (mise outdated --bump); with
// Update each pin that isn't held is moved, with its copies (deps.toml's [copies]), and mise installs them. Charter's
// command is left to GoModules where the repository's go.mod requires its library: the two move together.
func MiseTools(root string, mods []Module, conf Config, o Options) ([]Change, error) {
	withLibrary := false
	for _, m := range mods {
		if _, ok := m.Requires[CharterModule]; ok && m.Dir == root {
			withLibrary = true
		}
	}
	cmd := exec.Command("mise", "outdated", "--bump", "--json")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		o.warn(fmt.Errorf("mise outdated: %w", err))
	}
	var tools map[string]struct {
		Requested, Bump string
		Source          struct{ Path string }
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &tools); err != nil {
			return nil, fmt.Errorf("mise outdated: %w", err)
		}
	}
	file, _ := filepath.EvalSymlinks(filepath.Join(root, "mise.toml"))
	var out []Change
	moved := false
	for _, name := range slices.Sorted(maps.Keys(tools)) {
		t := tools[name]
		source, _ := filepath.EvalSymlinks(t.Source.Path)
		if source != file || t.Bump == "" || t.Bump == t.Requested || (name == CharterTool && withLibrary) {
			continue
		}
		c := Change{Name: name, From: t.Requested, To: t.Bump, Where: "mise.toml"}
		if why, held := conf.Hold.Why(name); held {
			c.Held = why
		} else if o.Update {
			if err := SetMisePin(root, name, t.Bump); err != nil {
				return out, err
			}
			if err := conf.MoveCopies(root, name, t.Requested, t.Bump); err != nil {
				return out, err
			}
			moved = true
		}
		out = append(out, c)
	}
	if moved {
		if err := Run(root, "mise", "install"); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Tidy runs go mod tidy in every module: a dependency moved in one module and not tidied in another fails its build.
func Tidy(mods []Module) error {
	for _, m := range mods {
		if err := Run(m.Dir, "go", "mod", "tidy"); err != nil {
			return err
		}
	}
	return nil
}

// Dirty is whether root's working tree differs from its last commit.
func Dirty(root string) (bool, error) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}
