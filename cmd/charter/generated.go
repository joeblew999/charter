package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// docs/_generated.toml lists the pages of a repo's docs that a command of the repo writes: each
// committed like every page (GitHub Pages runs no code), written by charter docs from what its
// command prints and checked by charter docs -check, as spec:check keeps the specs. Its name starts
// with an underscore, so the site does not publish it.
//
//	[[generated]]
//	page = "reference/commands.md"            # below docs/
//	run = "go run ./cmd/mytool docs-commands" # at the repo's root

const generatedFile = "_generated.toml"

// generatedPage is one [[generated]] table.
type generatedPage struct {
	Page string // its path below the docs folder, e.g. reference/commands.md
	Run  string // the command, run at the repo's root: program and arguments, "double quotes" for one with spaces
}

// readGenerated reads docs/_generated.toml in the docs folder; without one, a repo has no generated pages.
func readGenerated(docs string) ([]generatedPage, error) {
	text, err := os.ReadFile(filepath.Join(docs, generatedFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return parseGenerated(string(text))
}

// parseGenerated reads the format: [[generated]] tables of page = "..." and run = "...", comments
// on lines of their own. Anything else is an error, not ignored.
func parseGenerated(text string) ([]generatedPage, error) {
	var list []generatedPage
	seen := map[string]bool{}
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		at := func(format string, a ...any) error {
			return fmt.Errorf("docs/%s, line %d: "+format, append([]any{generatedFile, number + 1}, a...)...)
		}
		if line == "[[generated]]" {
			list, seen = append(list, generatedPage{}), map[string]bool{}
			continue
		}
		m := tomlLine.FindStringSubmatch(line)
		if m == nil || len(list) == 0 || (m[1] != "page" && m[1] != "run") {
			return nil, at(`want [[generated]], then page = "..." and run = "..."`)
		}
		value, err := strconv.Unquote(m[2])
		if err != nil || !strings.HasPrefix(m[2], `"`) {
			return nil, at(`%s is a string in double quotes`, m[1])
		}
		if seen[m[1]] {
			return nil, at("%s twice", m[1])
		}
		seen[m[1]] = true
		if m[1] == "page" {
			list[len(list)-1].Page = value
		} else {
			list[len(list)-1].Run = value
		}
	}
	pages := map[string]bool{}
	for _, g := range list {
		clean := filepath.ToSlash(filepath.Clean(g.Page))
		switch {
		case g.Page == "" || len(splitCommand(g.Run)) == 0:
			return nil, fmt.Errorf(`docs/%s: every [[generated]] needs page = "..." and run = "..."`, generatedFile)
		case clean != g.Page || filepath.IsAbs(g.Page) || strings.HasPrefix(clean, "../") || !strings.HasSuffix(clean, ".md"):
			return nil, fmt.Errorf("docs/%s: page %q: a .md path below docs/, like reference/commands.md", generatedFile, g.Page)
		case pages[clean]:
			return nil, fmt.Errorf("docs/%s: page %s twice", generatedFile, clean)
		}
		pages[clean] = true
	}
	return list, nil
}
