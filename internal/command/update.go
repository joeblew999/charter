package command

import (
	"github.com/joeblew999/charter/internal/run"
	"path/filepath"
	"strings"
)

// Update runs charter update: every pin moved to its latest: Go's direct modules and tools, npm's packages within
// the ranges package.json gives. Then charter check and charter test say whether the app still works.
func Update(args []string) {
	p := load()
	out, err := run.Output(p.Root, nil, "go", "list", "-m", "-f", "{{if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}", "all")
	check(err)
	if mods := strings.Fields(out); len(mods) > 0 {
		check(run.Run(p.Root, nil, "go", append([]string{"get", "-u"}, mods...)...))
		check(run.Run(p.Root, nil, "go", "mod", "tidy"))
	}
	if exists(filepath.Join(p.Root, "package.json")) {
		check(run.Run(p.Root, nil, "npm", "update"))
	}
	say("\nNext: charter check && charter test")
}
