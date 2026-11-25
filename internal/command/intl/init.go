package intl

import (
	"flag"
	"os"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/setup"
)

// initApp is charter add locale:<id>: languages for the app here. It writes charter.toml's [intl], a catalog per language,
// and for a gsx app gsx.toml's filters and renderers, leaving alone what is already there.
func initApp(args []string) {
	fs := flag.NewFlagSet("charter add locale:<id>", flag.ExitOnError)
	locales := fs.String("locales", "en", "the languages, BCP 47 ids, comma-separated, the source first")
	catalogs := fs.String("catalogs", "locales", "the folder of the catalogs, <locale>.toml")
	gsxui := fs.String("gsxui", "", "where gsxui add vendors components (default: found from gsxui.json)")
	fs.Parse(args)
	root, err := os.Getwd()
	must(err)
	if *gsxui == "" {
		*gsxui = setup.DetectGsxui(root)
	}
	must(setup.Init(root, setup.Options{Locales: strings.Split(*locales, ","), Catalogs: *catalogs, Gsxui: *gsxui}, os.Stdout))
}
