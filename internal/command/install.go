package command

import (
	"github.com/joeblew999/charter/internal/run"
	"github.com/joeblew999/charter/internal/tools"
	"os"
	"path/filepath"
)

// Install runs charter install: everything the app pins, installed: npm's packages as package-lock.json has them, Go's
// modules, the tools go.mod names, the agent skills of everything the app is built with (skills.go), and the two
// programs charter brings beside Go and Node: fnox and a container engine (package tools).
func Install(args []string) {
	p := load()
	if exists(filepath.Join(p.Root, "package-lock.json")) {
		check(run.Run(p.Root, nil, "npm", "ci"))
	} else if exists(filepath.Join(p.Root, "package.json")) {
		check(run.Run(p.Root, nil, "npm", "install"))
	}
	check(run.Run(p.Root, nil, "go", "mod", "download"))
	check(skills(p, true))
	// The two programs charter picks for every developer and brings itself (package tools): fnox for secrets, and a
	// container engine (Kiri, unless one already answers) for the SDK generators.
	if _, err := tools.Fnox(); err != nil {
		check(err)
	}
	if os.Getenv("CI") == "" {
		_, err := tools.Containers()
		check(err)
	}
}
