package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/project"
	"github.com/joeblew999/charter/internal/run"
)

// skills copies the agent skills of everything the app is built with into .claude/skills, where Claude Code and
// other agents read them: each module of go.mod that ships skills (skills/<name>/SKILL.md, as gsx, gsxui, structpages
// and charter do) gives its own, at the version go.mod pins. So a skill is always the one for the code in use, and
// moving a module moves its skill: nothing to remember, nothing to commit (.claude/skills is the machine's).
//
// It runs with charter install, and by itself before dev, check and build whenever go.sum has changed since.
func skills(p *project.Project, force bool) error {
	dest := filepath.Join(p.Root, ".claude", "skills")
	stamp := filepath.Join(dest, ".from")
	sum, _ := os.Stat(filepath.Join(p.Root, "go.sum"))
	if at, err := os.Stat(stamp); !force && err == nil && sum != nil && !sum.ModTime().After(at.ModTime()) {
		return nil
	}
	out, err := run.Output(p.Root, nil, "go", "list", "-m", "-json", "all")
	if err != nil {
		return err
	}
	var from []string
	for dec := json.NewDecoder(strings.NewReader(out)); dec.More(); {
		var m struct{ Path, Version, Dir string }
		if err := dec.Decode(&m); err != nil {
			return err
		}
		if m.Dir == "" {
			continue
		}
		found, _ := filepath.Glob(filepath.Join(m.Dir, "skills", "*", "SKILL.md"))
		for _, file := range found {
			src := filepath.Dir(file)
			name := filepath.Base(src)
			if err := os.RemoveAll(filepath.Join(dest, name)); err != nil {
				return err
			}
			if err := os.CopyFS(filepath.Join(dest, name), os.DirFS(src)); err != nil {
				return err
			}
			// The module cache is read-only, and so are its copies: the next sync must be able to replace them.
			filepath.WalkDir(filepath.Join(dest, name), func(path string, d os.DirEntry, err error) error {
				if err == nil {
					os.Chmod(path, map[bool]os.FileMode{true: 0o755, false: 0o644}[d.IsDir()])
				}
				return nil
			})
			from = append(from, name+" "+m.Path+" "+m.Version)
		}
	}
	if len(from) == 0 {
		return nil
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	say("skills: %s", strings.Join(names(from), ", "))
	return os.WriteFile(stamp, []byte("# Written by charter install: each skill here, and the module and version it came from.\n"+strings.Join(from, "\n")+"\n"), 0o644)
}

func names(from []string) []string {
	out := make([]string, len(from))
	for i, line := range from {
		out[i], _, _ = strings.Cut(line, " ")
	}
	return out
}
