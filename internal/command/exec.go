package command

import (
	"errors"
	"github.com/joeblew999/charter/internal/run"
)

// Exec runs charter exec <tool> [args]: one of the app's own tools, as the app pins it: an npm one (wrangler, vite,
// fern: node_modules/.bin) or a Go one (gsx, gsxui: go.mod's tool lines).
func Exec(args []string) {
	if len(args) == 0 {
		check(errors.New("usage: charter exec <tool> [arguments]   (wrangler, vite, gsx, gsxui, …)"))
	}
	p := load()
	if bin := run.Bin(args[0], p.Root); bin != args[0] {
		check(passthrough(p, bin, args[1:]...))
		return
	}
	check(passthrough(p, "go", append([]string{"tool"}, args...)...))
}
