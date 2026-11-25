package intl

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/setup"
	"github.com/joeblew999/charter/site/gsxintl/registry"
	intlui "github.com/joeblew999/charter/site/gsxintl/screens"
)

// add is charter intl add: intl's screens, copied into the app the way gsxui add copies components (#77), into
// intl.toml's ui (views/intl, package intlui), with their text, already translated, written into the catalogs. The
// copying is intl/registry's, which gsxkit's components use too (#164).
func add(args []string) {
	c := app()
	f := flag.NewFlagSet("add", flag.ExitOnError)
	diff := f.Bool("diff", false, "show how intl's screens differ from the app's copies, writing nothing (exit 1 if they do)")
	overwrite := f.Bool("overwrite", false, "replace copies the app changed")
	f.Parse(args)
	if f.NArg() == 0 || *diff && *overwrite {
		fmt.Fprintf(os.Stderr, "usage: charter intl add [-diff | -overwrite] <screen>…   screens: %s\n", strings.Join(intlui.Screens.Names(), ", "))
		os.Exit(2)
	}
	if !*diff && c.UI == "" {
		must(setup.UI(c.Root, filepath.ToSlash(c.UIDir()), os.Stdout))
	}
	code, err := intlui.Screens.Add(c, f.Args(), registry.Options{Dir: c.UIDir(), Diff: *diff, Overwrite: *overwrite})
	must(err)
	os.Exit(code)
}
