package command

import (
	"errors"
	"github.com/joeblew999/charter/internal/run"
	"os"
	"path/filepath"
	"strings"
)

// Cache runs charter cache clean: everything a build wrote is removed, so the next one starts from the sources.
func Cache(args []string) {
	if len(args) != 1 || args[0] != "clean" {
		check(errors.New("usage: charter cache clean"))
	}
	p := load()
	for _, dir := range generated {
		if exists(filepath.Join(p.Root, dir)) {
			check(os.RemoveAll(filepath.Join(p.Root, dir)))
			say("removed %s", dir)
		}
	}
	n := 0
	filepath.WalkDir(p.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		name := d.Name()
		if !d.IsDir() && (strings.HasSuffix(name, ".x.go") || name == "messages.gen.go") {
			if os.Remove(path) == nil {
				n++
			}
		}
		return nil
	})
	say("removed %d generated files", n)
	// gsx's own cache of what it generated.
	if hasGSX(p) {
		check(run.Run(p.Root, nil, "go", "tool", "gsx", "clean", "--cache"))
	}
}
