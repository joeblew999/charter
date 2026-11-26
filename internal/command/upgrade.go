package command

import (
	"github.com/joeblew999/charter/internal/run"
)

// Upgrade runs charter upgrade: charter itself moved to its latest, the library and the command together, then
// charter migrate for what the new one names differently.
func Upgrade(args []string) {
	p := load()
	check(run.Run(p.Root, nil, "go", "get", library+"@latest"))
	check(run.Run(p.Root, nil, "go", "get", "-tool", library+"/cmd/charter@latest"))
	check(run.Run(p.Root, nil, "go", "mod", "tidy"))
	check(run.Run(p.Root, nil, "go", "tool", "charter", "migrate"))
}
