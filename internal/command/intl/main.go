// Package intl is the languages' commands, which charter's verbs and tasks call (add locale:, check, build, run
// translate, …): an app's languages, from its charter.toml's [intl] (found up from the working directory). Each
// command is a file of its own, holding the function named for it (gen.go holds gen). The app imports intl's
// packages from this same module (github.com/joeblew999/charter/intl, gsxintl).
package intl

import (
	"fmt"
	"os"
	"strings"

	"github.com/joeblew999/charter/intl/config"
)

const help = "The app's languages are charter's own verbs and tasks: charter add locale:<id>, charter check, charter run (translate, languages, todo, confirm, export, import, prune).\n"

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
		fail(fmt.Errorf("%w: run charter add locale:<id> where the app's go.mod is", err))
	}
	return c
}
