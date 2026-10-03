package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func init() {
	commands["unchanged"] = command{"-files <pattern,...> <program> [args]",
		"run a program that writes generated files the project commits (gsx's *.x.go), and fail, naming them, if it changed, added or removed one: the committed ones were stale", unchanged}
}

// Not looked in: what is installed or built, and what git keeps.
var notSources = map[string]bool{"node_modules": true, "build": true, "dist": true, "target": true, "out": true}

// unchanged is the drift check of generated files that no spec check covers: run the generator,
// compare. The generator's output is left in place, so what to commit is there.
func unchanged(args []string) error {
	var patterns string
	rest := flags("unchanged", args, func(f *flag.FlagSet) {
		f.StringVar(&patterns, "files", "", "the files to compare, by name, comma-separated patterns such as *.x.go")
	})
	if patterns == "" || len(rest) == 0 {
		return errors.New("unchanged needs -files <pattern,...> and the program to run, e.g. -files \"*.x.go\" go tool gsx generate")
	}
	match := strings.Split(patterns, ",")
	for _, pattern := range match {
		if _, err := filepath.Match(pattern, ""); err != nil {
			return fmt.Errorf("-files %s: %w", pattern, err)
		}
	}
	before, err := snapshot(match)
	if err != nil {
		return err
	}
	path, err := program(rest[0])
	if err != nil {
		return err
	}
	if err := sh(".", path, rest[1:]...); err != nil {
		return err
	}
	after, err := snapshot(match)
	if err != nil {
		return err
	}
	var changed []string
	for file, content := range after {
		if old, ok := before[file]; !ok {
			changed = append(changed, file+" (new)")
		} else if !bytes.Equal(old, content) {
			changed = append(changed, file)
		}
	}
	for file := range before {
		if _, ok := after[file]; !ok {
			changed = append(changed, file+" (removed)")
		}
	}
	if len(changed) == 0 {
		return nil
	}
	slices.Sort(changed)
	return fmt.Errorf("%s changed what it generates, so the committed files were stale. They are written now: commit them.\n  %s", strings.Join(rest, " "), strings.Join(changed, "\n  "))
}

// snapshot is every file of the project whose name matches a pattern, with its content.
func snapshot(patterns []string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && (strings.HasPrefix(d.Name(), ".") || notSources[d.Name()]) {
				return filepath.SkipDir
			}
			return nil
		}
		for _, pattern := range patterns {
			if ok, _ := filepath.Match(pattern, d.Name()); ok {
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[filepath.ToSlash(path)] = content
				break
			}
		}
		return nil
	})
	return files, err
}
