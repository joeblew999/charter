package command

import (
	"github.com/joeblew999/charter/internal/run"
)

// Upgrade runs charter upgrade: charter itself moved to its latest, the library and the command together, and then
// the new charter's check --fix, so that what it writes for an app (its screens, its generated files) follows.
func Upgrade(args []string) {
	p := load()
	check(run.Run(p.Root, nil, "go", "get", library+"@latest"))
	check(run.Run(p.Root, nil, "go", "get", "-tool", library+"/cmd/charter@latest"))
	check(run.Run(p.Root, nil, "go", "mod", "tidy"))
	// What the new charter writes differently is written: the screens the app has not changed, the generated files.
	check(run.Run(p.Root, nil, "go", "tool", "charter", "check", "--fix"))
}
