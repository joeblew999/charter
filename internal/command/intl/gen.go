package intl

import (
	"os"

	"github.com/joeblew999/charter/internal/intlkit/setup"
)

// gen is charter build (charter build): the catalogs compiled into the app's messages package
// (intl_gen.go, intl-messages.mjs), the gsxui translator, and intl.sum.
func gen(args []string) {
	c := app()
	p := read(c)
	must(p.Generate(os.Stderr))
	must(setup.GsxuiRenderer(c.Root, c.Gsxui, os.Stderr))
}
