package command

import (
	"github.com/joeblew999/charter/internal/run"
	"path/filepath"
)

// Install runs charter install: everything the app pins, installed: npm's packages as package-lock.json has them, Go's
// modules, and the tools go.mod names.
func Install(args []string) {
	p := load()
	if exists(filepath.Join(p.Root, "package-lock.json")) {
		check(run.Run(p.Root, nil, "npm", "ci"))
	} else if exists(filepath.Join(p.Root, "package.json")) {
		check(run.Run(p.Root, nil, "npm", "install"))
	}
	check(run.Run(p.Root, nil, "go", "mod", "download"))
}
