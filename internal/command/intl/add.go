package intl

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/joeblew999/charter/intl/registry"
	"github.com/joeblew999/charter/app/screens"
)

// add is charter add screen:<name>: charter's screens (app/screens), copied into the app the way gsxui add copies
// components, into pages/screens (package screens), with their text, already translated, written into the catalogs.
func add(args []string) {
	c := app()
	f := flag.NewFlagSet("add", flag.ExitOnError)
	diff := f.Bool("diff", false, "show how charter's screens differ from the app's copies, writing nothing (exit 1 if they do)")
	overwrite := f.Bool("overwrite", false, "replace copies the app changed")
	f.Parse(args)
	if f.NArg() == 0 || *diff && *overwrite {
		fmt.Fprintf(os.Stderr, "usage: charter add screen:<name> [-diff | -overwrite] <screen>…   screens: %s\n", strings.Join(screens.Screens.Names(), ", "))
		os.Exit(2)
	}
	code, err := screens.Screens.Add(c, f.Args(), registry.Options{Dir: screens.Dir, Diff: *diff, Overwrite: *overwrite})
	must(err)
	os.Exit(code)
}
