package command

import (
	"github.com/joeblew999/charter/internal/run"
)

// Upgrade runs charter upgrade: charter itself moved to its latest, the library and the command together, and then
// the new charter's check --fix, so that what it writes for an app (its screens, its generated files) follows, and
// the app's mise.toml pins at least the Go, Node and fnox that charter was tested with, installed.
func Upgrade(args []string) {
	p := load()
	check(run.Run(p.Root, nil, "go", "get", library+"@latest"))
	check(run.Run(p.Root, nil, "go", "get", "-tool", library+"/cmd/charter@latest"))
	check(run.Run(p.Root, nil, "go", "mod", "tidy"))
	// What the new charter writes differently is written: the screens the app has not changed, the generated files.
	if err := run.Run(p.Root, nil, "go", "tool", "charter", "check", "--fix"); err != nil {
		// A release may change how an app is written: what it wrote for the release before no longer compiles, or
		// no longer passes. The docs say what each release changed and what to write instead.
		say("charter is upgraded, and the app does not pass its check yet. What each release changed in how an app is written, what to write instead, and what tells you: %s", upgrading)
		check(err)
	}
}

// upgrading is the page of charter's docs that says what each release changed in how an app is written.
const upgrading = "https://charter.gedw99.workers.dev/docs/upgrading"
