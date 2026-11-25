// Package intl is charter intl: an app's languages, from its charter.toml's [intl] (found up from the working directory). Each
// command is a file of its own, holding the function named for it (gen.go holds gen). The app imports intl's
// packages from this same module (github.com/joeblew999/charter/intl, gsxintl).
package intl

import (
	"fmt"
	"os"
	"strings"

	"github.com/joeblew999/charter/intl/config"
)

const help = `charter intl: the app's languages, through the runtime's own Intl, from its charter.toml's [intl].

  charter intl init            languages for this app: charter.toml's [intl], a catalog each (--locales en,de)
  charter intl                 per language: translated, inherited, stale, missing (status)
  charter intl check           every problem, for CI (--missing warn; or only the locales named)
  charter intl gen             the catalogs compiled into the app's messages package (charter build runs it)
  charter intl add-locale      a language added: de, pt-BR
  charter intl todo            what's left to translate (--json, for agents)
  charter intl translate       Claude Code translates what's left, a worker per language
  charter intl export          XLIFF for translators (--all, --out dir)
  charter intl import          translators' XLIFF read back
  charter intl confirm         stale translations that are still right, kept (<locale>[:<key>]…)
  charter intl prune           keys the source no longer has, removed

And:
  charter intl add             intl's screens, in a gsx app (--diff, --overwrite): language-list, language-offer, preferences
  charter intl assets          the JavaScript a Worker and a page import (--worker dir, --browser dir)
  charter intl help            this
`

// Main runs charter intl with args, the arguments after "intl".
func Main(args []string) {
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	switch command {
	case "":
		status(args)
	case "init":
		initApp(args)
	case "check":
		check(args)
	case "gen":
		gen(args)
	case "status":
		status(args)
	case "add-locale":
		addLocale(args)
	case "todo":
		todo(args)
	case "translate":
		translate(args)
	case "export":
		export(args)
	case "import":
		importXLIFF(args)
	case "confirm":
		confirm(args)
	case "prune":
		prune(args)
	case "add":
		add(args)
	case "assets":
		assets(args)
	case "show-help", "help":
		showHelp()
	default:
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}
}

// app is the app's charter.toml's [intl], found up from the working directory.
func app() config.Config {
	c, err := config.Find(".")
	if err != nil {
		fail(fmt.Errorf("%w: run charter intl init where the app's go.mod is", err))
	}
	return c
}
